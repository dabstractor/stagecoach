name: "P1.M1.T3.S1 — Update docs/cli.md + docs/configuration.md for config init --force re-targeting (BUG-001) and version-advisory accuracy (BUG-002)"
description: >
  Mode B changeset-level documentation sweep. Document the two code fixes in the two user-facing doc
  files: (1) BUG-001 — config init --force now re-targets the regenerated template to the PRESERVED
  [defaults] provider (not auto-detected pi), keeping [role.*] blocks consistent with the preserved
  default; an explicit --provider always wins, a preserved custom/unknown provider falls back to
  auto-detect. (2) BUG-002 — the load-time version advisory NEVER suggests config init --force:
  newer-than-binary ⇒ "Upgrade stagecoach" only; older/missing ⇒ config upgrade. Two doc files, four
  edit sites (two --force re-targeting clarifications + two advisory-sentence rewrites). Verify no
  other doc file references the old behavior. Documentation-only — no code, no tests.

---

## Goal

**Feature Goal**: Reconcile the user-facing documentation with the BUG-001 and BUG-002 code fixes so
the docs no longer describe the old (buggy/destructive) behavior. After this sweep, `docs/cli.md` and
`docs/configuration.md` accurately describe (a) that `config init --force` re-targets to the preserved
provider, and (b) that the version-mismatch advisory never suggests the destructive `config init --force`.

**Deliverable**: Four precise text edits across two files (NO code, NO tests):
1. `docs/cli.md` config-init section — add a `--force` re-targeting sentence after the flag table.
2. `docs/cli.md` line 214 — rewrite the advisory sentence (older/missing → `config upgrade`; newer →
   "Upgrade stagecoach"; never `config init --force`).
3. `docs/configuration.md` Bootstrap section — add a `--force` re-targeting sentence.
4. `docs/configuration.md` line 68 — rewrite the advisory sentence (drop `config init --force`; add the
   newer-than-binary upgrade-only case).
Plus a grep verification that no other doc site references the old behavior.

**Success Definition**:
- `docs/cli.md` config-init describes `--force` re-targeting to the preserved `[defaults] provider`.
- `docs/cli.md` + `docs/configuration.md` advisory sentences describe BOTH version-mismatch cases
  accurately and NEVER claim the advisory suggests `config init --force`.
- `docs/configuration.md` Bootstrap describes `--force` re-targeting.
- The `config init --force` EXAMPLE in `docs/cli.md:182` and the `upgrade --force` row in
  `docs/cli.md:413` are UNCHANGED (they are not advisory claims).
- `npx markdownlint-cli2 docs/cli.md docs/configuration.md` shows NO NEW errors (baseline = 1
  pre-existing MD028 at configuration.md:164, unrelated; docs/cli.md clean).
- `go test ./...` / `make test` pass (sanity — confirms no source file accidentally touched).
- No source code, PRD.md, tasks.json, or prd_snapshot.md modified.

## User Persona (if applicable)

**Target User**: A stagecoach user reading the docs to understand `config init --force` or to interpret
a load-time version-mismatch advisory.

**Use Case**: (a) "If I run `config init --force` over my `provider = "claude"` config, will it keep my
default?" — the docs now say yes, and that role blocks are re-targeted to claude. (b) "My config has a
newer schema than my binary — what should I do?" — the docs now say upgrade stagecoach (not
`config init --force`, which would destroy the newer config).

**Pain Points Addressed**: Pre-fix docs described `--force` as always auto-detecting pi (the BUG-001
behavior) and told users the version advisory suggests `config init --force` (the BUG-002 behavior the
fix removed). Both are now stale/inaccurate; this sweep corrects them.

## Why

- **Mode B docs-sync**: P1.M1.T1.S2 (BUG-001) and P1.M1.T2.S1 (BUG-002) each did Mode-A code-comment
  edits inline; the changeset-level user-facing docs are THIS task's job. Shipping the fixes with stale
  docs would (a) mislead users into thinking `--force` still injects an inconsistent stager provider,
  and (b) point users at the destructive `config init --force` for a version mismatch — the exact
  harmful advice BUG-002 removed from the code.
- **FR-B4 compliance**: FR-B4 forbids suggesting `config init --force` for version mismatches (it is a
  re-bootstrap, not an upgrade; for a newer config it discards an unreadable schema). The docs must
  match the code's now-correct advisory.
- **Bounded, surgical**: four text edits in two markdown files. No code, no tests, no new flags/config.

## What

**User-visible behavior**: none at runtime (docs-only). The artifact is accurate documentation.

**Technical change** (four edits, exact current → new text in the Blueprint below):
- `docs/cli.md`: +1 re-targeting sentence (config-init section); rewrite the line-214 advisory.
- `docs/configuration.md`: +1 re-targeting sentence (Bootstrap section); rewrite the line-68 advisory.

### Success Criteria
- [ ] `docs/cli.md` config-init section states `--force` (no `--provider`) re-targets to the preserved `[defaults] provider`
- [ ] `docs/cli.md:214` advisory describes older/missing (→ `config upgrade`) AND newer-than-binary (→ "Upgrade stagecoach"); never `config init --force`
- [ ] `docs/configuration.md` Bootstrap section states the same `--force` re-targeting
- [ ] `docs/configuration.md:68` advisory drops `config init --force` and adds the newer-than-binary upgrade-only case
- [ ] `docs/cli.md:182` (example) and `docs/cli.md:413` (`upgrade --force` row) UNCHANGED
- [ ] markdownlint shows no NEW errors (baseline: 1 pre-existing MD028 at configuration.md:164)
- [ ] `make test` passes (no source touched); no PRD/tasks.json modified

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact current text of all four edit sites (with line numbers), the exact replacement text,
the post-fix behavior (verified against the code), the grep inventory of every `config init --force` doc
occurrence (which to edit vs keep), the markdownlint baseline, and the scope fences (don't touch the
example at cli.md:182 or the `upgrade --force` row at cli.md:413) are all enumerated below.

### Documentation & References

```yaml
# MUST READ — the two files under edit
- file: docs/cli.md
  why: "Edit sites: config-init section flag table (--force row @197) + the advisory sentence @214
        (end of the config-upgrade section). KEEP the example @182 and the upgrade-command --force row @413."
  pattern: "config-upgrade section ends with a one-line advisory sentence (@214) — that is the BUG-002 doc site."
  gotcha: "line 413 '| `--force` | Override a detected package-manager install and self-swap (FR-U1) |' is the
           `upgrade` command's flag, NOT config init's. Do NOT touch it. line 182 is a legitimate example
           command — keep it."

- file: docs/configuration.md
  why: "Edit sites: Bootstrap section --force row (@49) + the advisory sentence @68 (Schema-versioning
        section). The pre-existing MD028 error is @164 — far from both; do not disturb it."
  pattern: "Schema-versioning section ends with the advisory sentence (@68) — the BUG-002 doc site."
  gotcha: "the 'Chrome is a separate axis'-style blockquote MD028 at line 164 is pre-existing and unrelated;
           your edits at 49/68 must not introduce a new blank-line-in-blockquote. New sentences are plain
           paragraphs (not blockquotes) ⇒ no MD028 risk."

# MUST READ — the post-fix BEHAVIOR to document (verified against the code)
- docfile: plan/018_29b859efcd56/bugfix/001_4be1bd5c953f/P1M1T3S1/research/notes.md
  why: "§1 gives the exact post-fix behavior for both bugs (re-targeting rules; advisory wording per
        branch, incl. which branches are LIVE vs DEAD in Load and what migrationNotice says). §2 has the
        exact current text of all four sites. §3 is the grep inventory (edit vs keep). §4 the lint baseline."

# CONTRACTS — the code fixes this task documents (treat as the source of truth for behavior)
- docfile: plan/018_29b859efcd56/bugfix/001_4be1bd5c953f/P1M1T2S1/PRP.md
  why: "BUG-002 contract: configVersionNotice newer branch → 'Upgrade stagecoach.' (no config init --force);
        older/missing dead branches → 'config upgrade' (no config init --force). The advisory doc sentences
        must match this."
- docfile: plan/018_29b859efcd56/bugfix/001_4be1bd5c953f/P1M1T1S2/PRP.md
  why: "BUG-001 contract: --force re-targets to preservedDefaultProvider; explicit --provider wins;
        custom/unknown preserved → auto-detect. The config-init doc sentence must match this."

# CODE (read-only — the behavior source; do NOT edit)
- file: internal/cmd/config.go
  why: "preservedDefaultProvider @449; runConfigInit @485-523 with the BUG-001 re-targeting comment. Confirms
        the exact --force re-targeting rules to document."
- file: internal/config/load.go
  why: "configVersionNotice @629-645 (BUG-002, already landed): newer branch → 'Upgrade stagecoach.';
        doc comment @629-631 states the FR-B4 rationale. Confirms the advisory wording to document."
- file: internal/config/migrate.go
  why: "migrationNotice @106 — the LIVE advisory for older/missing config_version ('Run stagecoach config
        upgrade to persist this to the file.'). Confirms the older/missing case says 'config upgrade', not
        'config init --force'."
```

### Current Codebase tree (relevant slice)

```bash
docs/
  cli.md              # EDIT: config-init --force re-targeting sentence + line-214 advisory rewrite
  configuration.md    # EDIT: Bootstrap --force re-targeting sentence + line-68 advisory rewrite
internal/cmd/config.go     # BUG-001 source (read-only) — preservedDefaultProvider @449
internal/config/load.go    # BUG-002 source (read-only, landed) — configVersionNotice @629
internal/config/migrate.go # older/missing advisory source (read-only) — migrationNotice @106
```

### Desired Codebase tree with files to be added

```bash
docs/cli.md           # MODIFY: +1 re-targeting sentence (config init); rewrite line-214 advisory
docs/configuration.md # MODIFY: +1 re-targeting sentence (Bootstrap); rewrite line-68 advisory
# (no new files; no code/test/PRD/tasks.json change)
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (two --force rows in docs/cli.md — only ONE is config init's): line 197
     '| `--force` | Overwrite an existing config file |' is config init's (EDIT context). Line 413
     '| `--force` | Override a detected package-manager install and self-swap (FR-U1) |' is the UPGRADE
     command's flag — a DIFFERENT command. Do NOT touch line 413. -->

<!-- CRITICAL (keep the example at docs/cli.md:182): 'stagecoach config init --force' inside the
     config-init code block is a legitimate USAGE EXAMPLE, not an advisory claim. It stays. Only the
     two advisory SENTENCES (cli.md:214, configuration.md:68) and the two flag-section clarifications
     change. A blind "remove all config init --force" sweep would wrongly delete the example. -->

<!-- CRITICAL (the advisory sentences must describe BOTH cases accurately): the current sentences only
     mention the older/missing case AND wrongly include 'config init --force'. Post-fix, the live
     advisory for older/missing is migrationNotice → 'config upgrade' (no --force), and for newer-than-
     binary it is 'Upgrade stagecoach' (no --force). Rewrite each sentence to cover both cases and to
     NEVER claim the advisory suggests 'config init --force' (FR-B4). -->

<!-- CRITICAL (mentioning 'config init --force' in the NEW sentences is intentional): the contract
     requires noting that the advisory 'must NOT suggest config init --force'. So the new advisory
     sentences WILL contain the string 'config init --force' — in a 'never suggests' context. A grep
     guard that flags ANY occurrence would false-positive on these intentional mentions. Scope the grep
     to the ADVISORY claim pattern ('advisory pointing at ... config init --force' / 'or config init
     --force to regenerate'), not the bare string. -->

<!-- GOTCHA (markdownlint baseline = 1 pre-existing error): docs/configuration.md:164 MD028 (blank line
     inside a blockquote) is pre-existing and UNRELATED to the edit sites (lines 49/68). Your edits must
     not add a NEW MD028/MD0xx error. New sentences are plain paragraphs (not blockquotes, not inside a
     blockquote) ⇒ safe. docs/cli.md is currently clean — keep it clean. -->

<!-- SCOPE: edit ONLY docs/cli.md + docs/configuration.md. No source code (the fixes are done/in-flight).
     No PRD.md / tasks.json / prd_snapshot.md / .gitignore. Mode B = user-facing docs only; no code
     comments (those were Mode A in T1.S2/T2.S1). -->
```

## Implementation Blueprint

### Data models and structure
None. Four text edits in two markdown files.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT docs/cli.md — add the --force re-targeting sentence to the config-init section
  - LOCATE the config-init flag table (the --force row is @197: '| `--force` | Overwrite an existing config file |').
  - AFTER the flag table (i.e. after the --interactive row, BEFORE the "--interactive runs a three-step
    wizard" paragraph @~199), ADD a sentence:
      With `--force` and no `--provider`, the regenerated template is **re-targeted to the preserved
      `[defaults] provider`** rather than auto-detecting pi — so the generated `[role.*]` blocks stay
      consistent with the default you kept (e.g. preserving `provider = "claude"` regenerates claude's
      role models, not pi's). An explicit `--provider <name>` always overrides this; a preserved
      custom/unknown provider falls back to auto-detection.
  - DO NOT touch line 413 (the upgrade command's --force) or line 182 (the example command).
  - DEPENDENCIES: none.

Task 2: EDIT docs/cli.md — rewrite the advisory sentence at line 214 (end of config-upgrade section)
  - LOCATE line 214: "At load time, a missing or outdated `config_version` triggers an advisory pointing
    at `config upgrade` or `config init --force`."
  - REPLACE WITH:
      At load time, a missing or outdated `config_version` triggers an advisory pointing at `config
      upgrade`; a **newer-than-binary** `config_version` triggers an advisory to **upgrade stagecoach**.
      The advisory never suggests `config init --force` — that would regenerate at the older binary's
      schema and destroy a config the binary cannot read (FR-B4).
  - DEPENDENCIES: none.

Task 3: EDIT docs/configuration.md — add the --force re-targeting sentence to the Bootstrap section
  - LOCATE the Bootstrap section's "If a config file already exists, it is NOT overwritten unless
    `--force` is passed (exit code 1). Parent directories are created as needed." line (@~51, just after
    the flag table whose --force row is @49).
  - AFTER that line, ADD a sentence:
      With `--force` and no `--provider`, the regenerated template is re-targeted to the preserved
      `[defaults] provider` (rather than auto-detecting pi), keeping the generated `[role.*]` blocks
      consistent with the preserved default. An explicit `--provider <name>` always overrides this.
  - DO NOT touch line 164 (the pre-existing MD028 blockquote).
  - DEPENDENCIES: none.

Task 4: EDIT docs/configuration.md — rewrite the advisory sentence at line 68 (Schema-versioning section)
  - LOCATE line 68: "At load time, if `config_version` is missing or older, stagecoach prints an advisory
    to stderr pointing at `config upgrade` (or `config init --force` to regenerate)."
  - REPLACE WITH:
      At load time, if `config_version` is missing or older, stagecoach prints an advisory to stderr
      pointing at `config upgrade` (never `config init --force` — that is a re-bootstrap, not an upgrade,
      per FR-B4). If `config_version` is **newer than the binary supports**, the advisory says only to
      **upgrade stagecoach** (regenerating would discard a schema the binary cannot read).
  - DEPENDENCIES: none.

Task 5: VERIFY — markdownlint, grep guards, no-regression, scope
  - npx --no-install markdownlint-cli2 docs/cli.md docs/configuration.md   # no NEW errors (baseline: 1 @configuration.md:164)
  - grep guards (see Validation Level 4): the two rewrites present; the two re-targeting sentences present;
    cli.md:182 example kept; cli.md:413 upgrade--force untouched.
  - go test ./...   # sanity (docs-only; confirms no source accidentally touched)
  - git diff --stat # ONLY docs/cli.md + docs/configuration.md
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN: the re-targeting sentence (consistent wording across both files; cli.md adds the
     custom/unknown fallback detail, configuration.md keeps it shorter) -->
With `--force` and no `--provider`, the regenerated template is re-targeted to the preserved
`[defaults] provider` (rather than auto-detecting pi), keeping the generated `[role.*]` blocks
consistent with the preserved default. An explicit `--provider <name>` always overrides this.

<!-- PATTERN: the advisory rewrite (both files cover older/missing + newer-than-binary; never --force) -->
At load time, a missing or outdated `config_version` triggers an advisory pointing at `config upgrade`;
a newer-than-binary `config_version` triggers an advisory to upgrade stagecoach. The advisory never
suggests `config init --force` (FR-B4) — that would regenerate at the older binary's schema and destroy
a config the binary cannot read.

<!-- CRITICAL: do NOT run a blind "delete all 'config init --force'" sweep. The string legitimately
     remains in (a) the cli.md:182 example command and (b) the new advisory sentences' "never suggests"
     context. Edit only the two advisory SENTENCES + add the two re-targeting sentences. -->
```

### Integration Points

```yaml
NO code / test / config / build / CLI changes. Four markdown text edits.

DOCS:
  - docs/cli.md: +re-targeting sentence (config-init section, after the flag table); rewrite line-214 advisory
  - docs/configuration.md: +re-targeting sentence (Bootstrap section, after the "If a config file already
    exists" line); rewrite line-68 advisory

CONSUMED (read-only — the behavior source):
  - internal/cmd/config.go preservedDefaultProvider @449 + runConfigInit @485-523 (BUG-001 re-targeting)
  - internal/config/load.go configVersionNotice @629-645 (BUG-002 newer advisory → "Upgrade stagecoach")
  - internal/config/migrate.go migrationNotice @106 (older/missing advisory → "config upgrade")

UNCHANGED (do NOT touch): docs/cli.md:182 (example); docs/cli.md:413 (upgrade --force row);
  docs/configuration.md:164 (pre-existing MD028); all source code; PRD.md; tasks.json; prd_snapshot.md.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Markdown lint (project ships .markdownlint.json). Baseline = 1 pre-existing error
# (docs/configuration.md:164 MD028, unrelated). The edits MUST NOT add errors.
npx --no-install markdownlint-cli2 docs/cli.md docs/configuration.md
# Expected: still exactly the 1 pre-existing configuration.md:164 MD028; docs/cli.md clean; NO error
# referencing the config-init flag table, the Bootstrap section, or the advisory sentences (lines 49/68/214).
# (If a NEW error appears at an edit site, fix the markdown — likely a stray blank line.)
```

### Level 2: Unit Tests (Component Validation)

```bash
# No code changed — run the suite as a no-regression sanity check (proves no source file was touched).
go test ./...
# Expected: ALL pass (a docs-only change cannot affect Go tests; this confirms scope discipline).
```

### Level 3: Integration Testing (System Validation)

```bash
# Docs-only subtask — no runtime behavior. The within-scope proof is the grep guards (Level 4) +
# markdownlint (Level 1). Optional: confirm the doc claims match the live binary advisory:
cat > /tmp/ahead.toml <<'EOF'
config_version = 99
[defaults]
provider = "stub"
EOF
go build -o /tmp/sc ./cmd/stagecoach
/tmp/sc --config /tmp/ahead.toml --dry-run 2>&1 | grep -i 'schema version 99'
# Expected: a line ending "Upgrade stagecoach." and NO "config init --force" (matches the rewritten doc).
rm -f /tmp/ahead.toml /tmp/sc
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: the two advisory sentences were rewritten (the STALE phrasing is gone).
grep -n 'advisory pointing at .config upgrade. or .config init --force' docs/cli.md            # expect: empty
grep -n 'config upgrade. (or .config init --force. to regenerate)' docs/configuration.md       # expect: empty

# Grep guard: the two NEW advisory sentences are present (they mention config init --force in a "never" context).
grep -n 'never suggests .config init --force' docs/cli.md                                        # expect: 1 hit
grep -n 'never .config init --force' docs/configuration.md                                       # expect: 1 hit

# Grep guard: the two re-targeting sentences are present.
grep -n 're-targeted to the preserved' docs/cli.md docs/configuration.md                         # expect: 1 hit each

# Scope guard: the example (cli.md:182) and the upgrade --force row (cli.md:413) are UNCHANGED.
grep -n 'stagecoach config init --force' docs/cli.md                                             # expect: 1 hit (the example, kept)
grep -n 'Override a detected package-manager install and self-swap' docs/cli.md                  # expect: 1 hit (upgrade --force, kept)

# Scope guard: only the two doc files changed.
git diff --stat
# Expected: docs/cli.md + docs/configuration.md ONLY. No source/PRD/tasks.json change.

# Scope guard: no source file touched.
git diff --stat -- '*.go' PRD.md '**/tasks.json' '**/prd_snapshot.md'
# Expected: empty.
```

## Final Validation Checklist

### Technical Validation
- [ ] `npx markdownlint-cli2 docs/cli.md docs/configuration.md` shows NO NEW errors (baseline 1 @configuration.md:164)
- [ ] `go test ./...` passes (no-regression sanity; no source touched)

### Feature Validation
- [ ] docs/cli.md config-init section states `--force` (no `--provider`) re-targets to the preserved `[defaults] provider`
- [ ] docs/cli.md:214 advisory covers older/missing (→ `config upgrade`) AND newer-than-binary (→ "Upgrade stagecoach"); never `config init --force`
- [ ] docs/configuration.md Bootstrap section states the same `--force` re-targeting
- [ ] docs/configuration.md:68 advisory drops `config init --force` and adds the newer-than-binary upgrade-only case

### Scope-Boundary Validation
- [ ] docs/cli.md:182 (example) UNCHANGED
- [ ] docs/cli.md:413 (`upgrade --force` row) UNCHANGED
- [ ] docs/configuration.md:164 (pre-existing MD028) undisturbed
- [ ] ONLY docs/cli.md + docs/configuration.md modified; no source/PRD/tasks.json/prd_snapshot.md change

### Documentation Quality
- [ ] Re-targeting wording is consistent across both files (cli.md adds the custom/unknown fallback detail)
- [ ] Advisory wording is accurate to the post-fix code (migrationNotice for older/missing; "Upgrade stagecoach" for newer)
- [ ] No claim that the advisory suggests `config init --force` remains in either file

---

## Anti-Patterns to Avoid

- ❌ **Don't run a blind "remove all `config init --force`" sweep.** The string legitimately remains in the docs/cli.md:182 example command AND in the new advisory sentences' "never suggests" context. Edit only the two advisory SENTENCES + add the two re-targeting sentences.
- ❌ **Don't touch docs/cli.md:413.** That `--force` row belongs to the `upgrade` command ("Override a detected package-manager install and self-swap"), NOT `config init`. It is a different command's flag.
- ❌ **Don't leave the advisory sentences describing only the older/missing case.** Post-fix there are TWO live cases (older/missing → `config upgrade` via migrationNotice; newer-than-binary → "Upgrade stagecoach" via configVersionNotice). Both must be documented, and neither suggests `config init --force`.
- ❌ **Don't claim `config init --force` is suggested for a version mismatch.** FR-B4 forbids it; the BUG-002 code fix removed it. The docs must say the advisory NEVER suggests it (and why — it would destroy a newer config / is a re-bootstrap).
- ❌ **Don't add markdown that triggers a NEW lint error.** The new sentences are plain paragraphs (not blockquotes); keep them out of the blockquote at configuration.md:164. Baseline is 1 pre-existing error — your edits must not increase it.
- ❌ **Don't touch source code, PRD.md, tasks.json, or prd_snapshot.md.** This is a docs-only Mode-B sweep; the code fixes are done (BUG-001) / in-flight (BUG-002, contract assumed landed).
- ❌ **Don't reword the re-targeting rules inconsistently between the two files.** The core claim ("`--force` re-targets to the preserved `[defaults] provider`") must match across docs/cli.md and docs/configuration.md; cli.md may add the custom/unknown-fallback detail, but the primary behavior must agree.
- ❌ **Don't document the auto-detect-pi behavior for `--force`.** That was the BUG-001 bug; post-fix `--force` re-targets to the preserved default. Describing the old behavior would re-introduce the doc drift this task exists to fix.

---

## Confidence Score

**9/10** — one-pass success likelihood. All four edit sites are pinned with their exact current text and
line numbers, the exact replacement text is specified verbatim, the post-fix behavior is verified against
the actual code (config.go preservedDefaultProvider; load.go configVersionNotice already landed;
migrate.go migrationNotice for the live older/missing advisory), the grep inventory distinguishes
edit-vs-keep (the cli.md:182 example and cli.md:413 upgrade--force row), and the markdownlint baseline
is established (1 pre-existing error far from the edit sites). The −1 covers two judgment calls: (1) the
advisory sentences will still CONTAIN the string "config init init --force" in a "never suggests" context
(intentional per the contract, but a careless grep-guard could false-positive — scoped in Level 4); and
(2) the exact placement of the re-targeting sentence relative to the flag table / the "If a config file
already exists" line (specified, but markdown structure around tables can be finicky). Both are flagged
with scoped grep guards.