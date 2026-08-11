name: "P1.M2.T1.S2 — docs/cli.md + docs/configuration.md: sync the 4 format-base enumerations to the shipped <base>[+body] grammar (FR-F1/FR-F9, §9.19/§17.8)"
description: >
  Pure Mode-A docs edit to TWO files. Sync the 4 stale format-base enumerations (docs/cli.md L38;
  docs/configuration.md L116, L216, L247) with the ALREADY-SHIPPED `<base>[+body]` grammar that
  P1.M1.T1.S4 (Complete) established in root.go:202 / config.go:105 / bootstrap.go:376. Each enumeration
  gains the `<base>[+body]` lead-in + the "append +body to force a subject+body" note. NO code, NO tests
  (the +body implementation is P1.M1; the +body tests are the parallel P1.M2.T1.S1). README.md + how-it-
  works.md are Mode-B changeset-docs (P1.M2.T2.S1) — NOT this task. CRITICAL: the contract's literal
  acceptance grep (`rg 'auto\|conventional\|gitmoji\|plain'`) matches only L116 due to ripgrep's `\|`
  = literal-pipe semantics; the REAL gate is `rg -n gitmoji` = 4 lines, each mentioning +body (matches
  the contract's STATED intent "wherever the 4 bases are listed"). Edit ALL 4 locations per LOGIC (a)–(d).

---

## Goal

**Feature Goal**: Make the 4 user-facing doc surfaces that enumerate the format bases (`auto |
conventional | gitmoji | plain`) reflect the shipped `<base>[+body]` grammar — so a reader of
docs/cli.md's `--format` flag table or docs/configuration.md's config/env/git-config tables learns that
every base optionally takes a `+body` suffix (FR-F9), matching what `stagecoach --help` and `config init`
already print. Today all 4 lag the binary (they list only the 4 bases).

**Deliverable**: TWO modified doc files, no new files:
1. **docs/cli.md** — L38 `--format` table-row Description cell updated.
2. **docs/configuration.md** — L116 config template comment, L216 env-table row, L247 git-config-table
   row updated.

**Success Definition**:
- Every `gitmoji` line in docs/cli.md + docs/configuration.md (exactly 4: L38, L116, L216, L247) now
  mentions `+body` and the `<base>[+body]` grammar.
- docs/configuration.md L116 byte-matches the shipped bootstrap.go:376 `config init` template comment.
- Each table cell remains valid markdown (pipes inside cells stay escaped `\|` or backtick-wrapped; no
  unescaped `|` splits a row; cell count per row unchanged).
- The 2 non-enumeration `format` refs (cli.md L486 mapping table, configuration.md L152 defaults table)
  are UNTOUCHED (they list only `"auto"` as a value).
- `make test` stays green (a docs edit cannot break Go tests).

## User Persona (if applicable)

**Target User**: A developer reading the CLI/config reference docs to pick a `--format` / `[generation].format` /
`STAGECOACH_FORMAT` / `stagecoach.format` value — specifically one who wants a commit body and is
looking for how to force one.

**Use Case**: "I want conventional-commit subjects WITH a body" → the docs now show `conventional+body`
(the env example) and the `<base>[+body]` grammar at every surface, not just the PRD.

**User Journey**: docs/cli.md `--format` row → sees `<base>[+body]` + "append +body to force a
subject+body" → docs/configuration.md L116/L216/L247 reinforce it → `STAGECOACH_FORMAT=conventional+body
stagecoach` (the new env example) → subject + body lands.

**Pain Points Addressed**: the 4 doc surfaces currently describe only the 4 bases, so a doc-only reader
never discovers `+body` (FR-F9) — they'd have to find it in `--help` or the PRD. This closes the gap.

## Why

- **Mode A (docs match the binary)**: P1.M1.T1.S4 (Complete) shipped the canonical `<base>[+body]`
  wording in root.go:202 / config.go:105 / bootstrap.go:376. The docs/README note ("if anything
  disagrees with `stagecoach --help`, the binary is authoritative") mandates the docs catch up. This
  task is that catch-up for the CLI + config references.
- **FR-F9 / §9.19 / §17.8**: the `+body` modifier is a shipped feature; the 4 enumeration surfaces are
  the places a user picks a format value, so each must state the grammar.
- **Bounded scope**: 4 localized cell/comment edits across 2 markdown files. No code, no tests, no
  schema. The implementation (P1.M1) and the test suite (parallel S1) are separate; README/how-it-works
  are the later Mode-B sweep (P1.M2.T2.S1).

## What

**User-visible behavior**: the 4 doc surfaces now describe `<base>[+body]`. No runtime change.

**Technical change** (4 edits across 2 files; verbatim target text in the Blueprint):

### Edit (a) — docs/cli.md L38: `--format` table-row Description cell
Lead with `` `<base>[+body]` ``, list the 4 bases, add the `+body` note. Preserve the 5 leading cells
(flag/type/default/env/git-config) + the trailing `Also \`[generation].format\`.` clause + the escaped
`\|` pipe style.

### Edit (b) — docs/configuration.md L116: config template comment
Byte-match the shipped bootstrap.go:376 template comment (`<base>[+body]: auto|conventional|gitmoji|plain,
each optionally +body; unknown = hard error (exit 1)`). Keep the alignment column.

### Edit (c) — docs/configuration.md L216: env-table row
Description mentions `+body`; the example cell shows `STAGECOACH_FORMAT=conventional+body stagecoach`.

### Edit (d) — docs/configuration.md L247: git-config-table row
Description leads with `` `<base>[+body]` `` + the `+body` note. Backtick-wrapped bases + escaped pipes.

### Success Criteria
- [ ] `rg -n 'gitmoji' docs/cli.md docs/configuration.md` → exactly 4 hits (L38, L116, L216, L247); each mentions `+body`
- [ ] docs/configuration.md L116 byte-matches bootstrap.go:376 (the `config init` template comment)
- [ ] cli.md L38 row still has 6 cells (no broken `|`); the trailing `Also [generation].format.` preserved
- [ ] configuration.md L216/L247 rows keep their cell counts (escaped `\|` pipes intact)
- [ ] cli.md L486 + configuration.md L152 (non-enumeration `format` refs) UNTOUCHED
- [ ] only docs/cli.md + docs/configuration.md changed

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim target text for all 4 edits (findings §7), the exact current text + line numbers
(findings §2), the canonical shipped wording to match (findings §1), the acceptance-grep trap and its
correct resolution (findings §3), the markdown-table escaping rules (findings §5), and the explicit
scope fences (findings §6).

### Documentation & References

```yaml
# MUST READ — the authoritative research (verbatim target text + the grep trap + escaping rules)
- docfile: plan/021_086bbc57a2b2/P1M2T1S2/research/findings.md
  why: "§1 = the canonical wording is ALREADY SHIPPED (root.go:202/config.go:105/bootstrap.go:376) — the
        docs catch up to the binary. §2 = the 4 stale locations with exact current text + line numbers.
        §3 = CRITICAL: the contract's literal acceptance grep matches only L116 (ripgrep `\|` = literal
        pipe); the REAL gate is `rg -n gitmoji` = 4 lines each mentioning +body. §5 = markdown table
        escaping rules. §7 = verbatim target text to transcribe."
  critical: "§3 is the make-or-break insight: do NOT edit only L116. The contract's LOGIC (a)-(d) names
             all 4 locations — edit ALL 4. Verify with `rg -n gitmoji` (4 hits, each with +body), NOT
             the contract's literal grep (which silently passes after a 1-line edit). §5: pipes inside
             markdown table cells MUST stay escaped `\\|` (cli.md L38, config L216/L247); L116 is a TOML
             comment with LITERAL pipes."

# MUST READ — the architecture doc (exact current text + line numbers for all 4 locations)
- docfile: plan/021_086bbc57a2b2/architecture/test_and_docs_surfaces.md
  why: "The 'Docs surfaces' section tabulates docs/cli.md L38 and docs/configuration.md L116/L216/L247
        with their exact current text, plus README.md L63/L196 (Mode B — NOT this task). The 'Acceptance
        grep' section is the (narrow) contract criterion — see findings §3 for why `gitmoji` is the
        correct marker instead."
  critical: "It confirms the 4 edit locations and that README.md is a SEPARATE surface (P1.M2.T2.S1)."

# MUST READ — the source of truth this docs task matches (the shipped canonical wording)
- file: internal/cmd/root.go
  why: "Line 202 — the `--format` flag help string — is the canonical user-facing wording:
        `Message format: <base>[+body] — auto|conventional|gitmoji|plain, append +body to force a …`.
        docs/cli.md L38 must agree with this (the docs/README 'binary is authoritative' rule)."
  pattern: "The flag help is the single canonical phrasing; the docs transcribe it (adapted to the
            table-cell escaped-pipe style). Do NOT invent different wording per surface."
  gotcha: "READ-ONLY. Do not edit root.go. It is the source of truth; the docs catch up to it."

# MUST READ — the config.go Format field doc (the canonical grammar description)
- file: internal/config/config.go
  why: "Line 105 — `Format selects the commit-message style … a <base>[+body] grammar where … the
        \"+body\" suffix forces a subject-plus-body message regardless of repo history (FR-F9)` — is the
        canonical grammar statement docs/configuration.md should mirror."
  gotcha: "READ-ONLY. The docs agree with this; do not edit it."

# MUST READ — bootstrap.go:376 (the config init template comment L116 must byte-match)
- file: internal/config/bootstrap.go
  why: "Line 376 — `# format = \"auto\"   # <base>[+body]: auto|conventional|gitmoji|plain, each
        optionally +body; unknown = hard error (exit 1)` — is the EXACT string docs/configuration.md L116
        must reproduce (the config template comment is documented in BOTH the Go source and the docs;
        they must agree byte-for-byte)."
  critical: "L116 must byte-match this (same words, same alignment column). Copy it verbatim."

# MUST READ — the two files being edited (locate edit points by content; lines drift)
- file: docs/cli.md
  why: "The `--format` row is L38 in the §15.2 Global flags table (locate via
        `grep -n -- '--format <mode>' docs/cli.md`). It is a 6-cell markdown table row; the Description
        cell is the 6th. L486 is a separate flag/env/git-config MAPPING table (`| --format |
        STAGECOACH_FORMAT | stagecoach.format |`) — it does NOT enumerate the 4 bases; leave it alone."
  pattern: "Table rows use escaped `\\|` for in-cell pipes + backticks around tokens. Preserve both."
  gotcha: "Line numbers drift on any sibling edit. Locate the 4 edit points by content: cli.md L38 via
           `grep -n gitmoji docs/cli.md`; configuration.md via `grep -n gitmoji docs/configuration.md`."

- file: docs/configuration.md
  why: "L116 is in the '## File format' template-comment block (locate via
        `grep -n 'auto|conventional|gitmoji|plain'`). L216 is in the Environment-variables table; L247 in
        the Git-config-keys table. L152 (`| format | \"auto\" | config.Defaults |`) is a defaults table
        row — it lists only \"auto\" as a value, NOT the 4 bases; leave it alone."
  pattern: "L116 = TOML comment (literal pipes); L216/L247 = markdown table cells (escaped `\\|` pipes)."

# CONTEXT — the PRD spec (the canonical grammar definition)
- docfile: plan/021_086bbc57a2b2/prd_snapshot.md
  why: "§17.8 (h3.90) defines the `+body` modifier (FR-F9): a `+body` variant replaces the multi-line
        rule with an unconditional body directive; the subject contract is unchanged by the suffix.
        §15.2 (h2.16) shows the canonical `--format` row; §16.2 (h3.80) shows the canonical config
        template comment. The docs edits encode exactly this."
  section: "§17.8 Format modes (the +body modifier, FR-F9); §15.2 Global flags (--format); §16.2 config template"

# CONTEXT — the parallel sibling (NO overlap; confirms the same canonical wording)
- docfile: plan/021_086bbc57a2b2/P1M2T1S1/PRP.md
  why: "Parallel S1 is TEST-only (load_test/format_test/system_test +body regression suite). It does NOT
        touch docs. It consumes the same P1.M1.T1.S1-S4 implementation whose canonical wording this docs
        task surfaces. No file overlap; no conflict."
  critical: "Do NOT edit any Go test file (S1's domain). This task is docs/cli.md + docs/configuration.md only."
```

### Current Codebase tree (relevant slice)

```bash
docs/
  cli.md              # EDIT (a) — L38 --format table-row Description cell
  configuration.md    # EDIT (b)(c)(d) — L116 template comment, L216 env table, L247 git-config table
  how-it-works.md     # READ-ONLY / NOT this task — Mode B (P1.M2.T2.S1) format-section prose
README.md             # READ-ONLY / NOT this task — Mode B (P1.M2.T2.S1) L63/L196
internal/cmd/
  root.go             # READ-ONLY — L202 the canonical --format help (the source of truth docs match)
internal/config/
  config.go           # READ-ONLY — L105 the canonical Format grammar doc
  bootstrap.go        # READ-ONLY — L376 the config init template comment L116 must byte-match
.markdownlint.json    # READ-ONLY — {MD013:false, MD033:false, MD060:false}; no make target for docs
Makefile              # READ-ONLY — make lint = golangci-lint (Go only); docs not linted by make
```

### Desired Codebase tree with files to be added/modified

```bash
# MODIFIED (no new files):
docs/cli.md           # L38 --format Description cell → <base>[+body] grammar + +body note
docs/configuration.md # L116 template comment + L216 env row + L247 git-config row → <base>[+body]
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (edit ALL 4 locations, not just L116): the contract's literal acceptance grep
     `rg 'auto\|conventional\|gitmoji|plain'` matches ONLY configuration.md L116, because ripgrep treats
     `\|` as a LITERAL pipe (Rust regex), and only L116's TOML comment has literal pipes. The contract's
     LOGIC (a)-(d) names all 4 locations — edit ALL 4. Verify with the CORRECT gate:
       rg -n 'gitmoji' docs/cli.md docs/configuration.md   # 4 hits (L38, L116, L216, L247), each with +body
     (findings §3). Do NOT stop after L116. -->

<!-- CRITICAL (pipe escaping differs by location): cli.md L38 + configuration.md L216 + L247 are markdown
     TABLE CELLS — in-cell pipes MUST stay escaped `\|` (or backtick-wrapped); an unescaped `|` splits the
     cell and breaks the row. configuration.md L116 is a TOML COMMENT — pipes are LITERAL (`auto|conventional|…`),
     do NOT escape them (it must byte-match bootstrap.go:376). -->

<!-- CRITICAL (L116 must byte-match the shipped template): configuration.md L116 is the documented form of
     the `config init` template comment (bootstrap.go:376). Copy that line VERBATIM (same wording, same
     alignment column). A divergence means the docs disagree with `config init` output — the exact thing
     this task fixes. -->

<!-- GOTCHA (the 2 non-enumeration format refs stay untouched): cli.md L486 (`| --format |
     STAGECOACH_FORMAT | stagecoach.format |` — a flag/env/git mapping) and configuration.md L152
     (`| format | "auto" | config.Defaults |` — a defaults table) list only "auto" as a VALUE, not the
     4-base enumeration. They do NOT trip the `gitmoji` gate and are correctly out of scope. -->

<!-- GOTCHA (preserve cell counts + trailing clauses): cli.md L38 keeps its 6 cells AND the trailing
     `Also [configuration].format.` clause. configuration.md L216 keeps 4 cells; L247 keeps 4 cells.
     A lost cell or dropped clause is a regression. -->

<!-- GOTCHA (angle brackets + square brackets render literally in markdown): `<base>[+body]` needs no
     escaping in a table cell. Backtick-wrap it (`<base>[+body]`) for consistency with the existing
     backtick style of cli.md L38 / configuration.md L247. -->

<!-- GOTCHA (locate by content, not line number): lines drift on sibling edits. Locate the 4 points via
     `grep -n gitmoji docs/cli.md docs/configuration.md` (the reliable enumeration marker). -->

<!-- GOTCHA (docs are NOT in make): there is no markdownlint make target. Validation = grep gates +
     manual render. MD013 (line length) is OFF, so long table cells are fine. -->
```

## Implementation Blueprint

### Data models and structure
None. Pure markdown. No code, no types.

### Implementation Tasks (ordered — independent edits; do in any order)

```yaml
Task 1: EDIT docs/cli.md L38 — the --format table-row Description cell
  - LOCATE: grep -n -- '--format <mode>' docs/cli.md  (or grep -n gitmoji docs/cli.md → the single hit).
    The row is 6 cells; the Description is the 6th (last) cell.
  - REPLACE the Description cell text. CURRENT:
      Message format: `auto` (style learning) \| `conventional` \| `gitmoji` \| `plain`. An unknown mode is a hard error (exit 1). Also `[generation].format`.
    TARGET:
      Message format: `<base>[+body]` — `auto` (style learning) \| `conventional` \| `gitmoji` \| `plain`; append `+body` to force a subject+body. Unknown = hard error (exit 1). Also `[generation].format`.
  - PRESERVE: the 5 leading cells (--format <mode> | string | auto | STAGECOACH_FORMAT | stagecoach.format),
    the escaped `\|` in-cell pipes, the backticks around tokens, and the trailing `Also [generation].format.` clause.
  - VERIFY the row still has 6 cells (5 unescaped `|` separators) after the edit.

Task 2: EDIT docs/configuration.md L116 — the File-format config template comment
  - LOCATE: grep -n 'auto|conventional|gitmoji|plain' docs/configuration.md  (L116, the TOML comment).
  - REPLACE the inline comment. CURRENT:
      # format                = "auto"   # auto|conventional|gitmoji|plain; unknown = hard error (exit 1)
    TARGET (byte-match bootstrap.go:376):
      # format                = "auto"   # <base>[+body]: auto|conventional|gitmoji|plain, each optionally +body; unknown = hard error (exit 1)
  - PRESERVE: the leading `# format` key, the `= "auto"` value, the alignment column (spaces before `=`
    and before the `#` inline comment — must line up with locale L117 / template L118 / the fields above).
    Pipes stay LITERAL (TOML comment, not a markdown table).

Task 3: EDIT docs/configuration.md L216 — the Environment-variables table row
  - LOCATE: grep -n 'STAGECOACH_FORMAT' docs/configuration.md  (L216, env table).
  - REPLACE the Description + Example cells. CURRENT:
      Message format (auto\|conventional\|gitmoji\|plain; unknown = hard error) | `STAGECOACH_FORMAT=conventional stagecoach`
    TARGET:
      Message format: `<base>[+body]` — auto\|conventional\|gitmoji\|plain; append `+body` to force a subject+body; unknown = hard error (exit 1) | `STAGECOACH_FORMAT=conventional+body stagecoach`
  - PRESERVE: the 2 leading cells (STAGECOACH_FORMAT | --format) and the escaped `\|` in-cell pipes.
    The example now demonstrates the +body suffix (conventional+body).
  - VERIFY the row still has 4 cells.

Task 4: EDIT docs/configuration.md L247 — the Git-config-keys table row
  - LOCATE: grep -n 'stagecoach.format' docs/configuration.md  (L247, git-config table).
  - REPLACE the Description cell. CURRENT:
      Message format: `auto` \| `conventional` \| `gitmoji` \| `plain`. Unknown = hard error (exit 1).
    TARGET:
      Message format: `<base>[+body]` — `auto` \| `conventional` \| `gitmoji` \| `plain`. Append `+body` to force a subject+body. Unknown = hard error (exit 1).
  - PRESERVE: the 3 leading cells (stagecoach.format | string | `git config --get stagecoach.format`),
    the escaped `\|` pipes, and the backticks around the 4 bases.
  - VERIFY the row still has 4 cells.

Task 5: VERIFY — the gitmoji gate + scope guard + render check
  - rg -n 'gitmoji' docs/cli.md docs/configuration.md          # 4 hits; each line contains '+body'
  - rg -n 'base>\[.body.\]\|<base>\[+body\]\|+body' docs/cli.md docs/configuration.md  # +body present at all 4
  - grep -c 'gitmoji' docs/cli.md docs/configuration.md        # cli=1, configuration=3 (total 4)
  - git diff --name-only                                       # == {docs/cli.md, docs/configuration.md}
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN: the canonical lead-in (transcribe the shipped wording; adapt escaping per surface) -->
#   `<base>[+body]` — auto | conventional | gitmoji | plain; append `+body` to force a subject+body. Unknown = hard error (exit 1).

<!-- PATTERN: escaping differs by surface -->
#   markdown TABLE cell (cli.md L38, config L216/L247):  escaped `\|` for in-cell pipes; backticks around tokens
#   TOML COMMENT (config L116):                          LITERAL pipes; byte-matches bootstrap.go:376

<!-- PATTERN: the L116 byte-match contract -->
#   configuration.md L116 == bootstrap.go:376 (both are the documented `config init` template comment).
#   Copy the Go-source line verbatim (same words + alignment column).
```

### Integration Points

```yaml
NO code / tests / schema / config / routes / new deps. TWO markdown files edited (no new files).

DOCS (docs/cli.md):
  - L38 --format table-row Description cell → <base>[+body] grammar + +body note.

DOCS (docs/configuration.md):
  - L116 File-format template comment → byte-match bootstrap.go:376.
  - L216 Environment-variables row → +body in description; example shows conventional+body.
  - L247 Git-config-keys row → <base>[+body] grammar + +body note.

SCOPE FENCES: NO Go code/tests (P1.M1 + parallel S1); NO README.md or docs/how-it-works.md (Mode B,
  P1.M2.T2.S1); NO cli.md L486 / configuration.md L152 (non-enumeration format refs); NO PRD.md /
  tasks.json / prd_snapshot.md.
```

## Validation Loop

> Docs are NOT linted by `make` (`.markdownlint.json` exists with MD013/MD033/MD060 OFF but is not wired
> to a make target; `make lint` is golangci-lint for Go only). Validation = grep gates + manual render +
> scope guard. The mechanical gate is `rg -n gitmoji` = 4 lines, each mentioning +body.

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Scope guard: exactly the 2 doc files changed.
git diff --name-only
# Expected: docs/cli.md  docs/configuration.md (exactly these 2).

# Confirm no Go file changed (docs-only task).
git diff --stat -- '*.go'
# Expected: empty.

# (Optional) markdownlint if installed locally — baseline green except known pre-existing.
npx --no-install markdownlint-cli2 'docs/cli.md' 'docs/configuration.md' 2>/dev/null \
  || echo "(markdownlint not installed / not in make — skip; MD013 is off so long cells are fine)"
```

### Level 2: Unit Tests (Component Validation)

```bash
# No Go tests are authored or affected by this docs edit. Run the suite ONLY to prove the tree is clean
# (parallel Go work / the S1 test suite may be in flight, but this docs edit cannot break Go tests).
make test
# Expected: green (race detector). Unaffected by these doc edits.
```

### Level 3: Integration Testing (System Validation)

```bash
# THE mechanical gate: every format-base enumeration now states +body.
# (Use gitmoji as the reliable marker — it appears in exactly the 4 enumeration locations.)
rg -n 'gitmoji' docs/cli.md docs/configuration.md
# Expected: 4 hits — cli.md L38, configuration.md L116/L216/L247. Eyeball EACH: it must contain "+body".

# Confirm +body appears on all 4 of those lines (no enumeration omits the suffix).
rg -n 'gitmoji' docs/cli.md docs/configuration.md | rg -v '\+body'
# Expected: NO output (every gitmoji line also has +body). Any output = a missed location.

# Byte-match check: configuration.md L116 == the shipped config init template comment (bootstrap.go:376).
diff <(grep -n 'format.*=.*"auto"' docs/configuration.md | head -1) \
     <(grep -n '# format .* = "auto"' internal/config/bootstrap.go | head -1) 2>/dev/null || true
# (Manual: confirm the two `# format = "auto"   # <base>[+body]: …` strings agree word-for-word.)

# Manual render check of the 3 table cells (pipes must not break the rows).
glow docs/cli.md 2>/dev/null | sed -n '/--format/,/--locale/p' || sed -n '37,40p' docs/cli.md
# Expected: the --format row renders as one row (6 cells); the Description cell shows <base>[+body> + the bases + "+body".
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard 1 (THE gate): every gitmoji line mentions +body.
rg -n 'gitmoji' docs/cli.md docs/configuration.md | rg -v '\+body'
# Expected: empty (no output).

# Grep guard 2: the <base>[+body> grammar is stated at all 4 locations.
rg -c '<base>\[+body\]' docs/cli.md docs/configuration.md
# Expected: cli.md >= 1; configuration.md >= 3 (L116 + L216 + L247).

# Grep guard 3: configuration.md L116 byte-matches the shipped template (the Mode-A catch-up).
grep -F '<base>[+body]: auto|conventional|gitmoji|plain, each optionally +body; unknown = hard error (exit 1)' docs/configuration.md internal/config/bootstrap.go
# Expected: 2 hits (one in each file — they agree).

# Grep guard 4: the 2 NON-enumeration format refs are UNTOUCHED (no +body added there — they list only "auto").
rg -n 'STAGECOACH_FORMAT.*stagecoach.format\b' docs/cli.md            # L486 mapping table (no gitmoji → unchanged)
grep -n '| `format` | `"auto"` |' docs/configuration.md               # L152 defaults table (no gitmoji → unchanged)
# Expected: both present; NEITHER contains gitmoji or +body (they were correctly left alone).

# Grep guard 5 (cell-count sanity — no broken table row): the cli.md --format row has exactly 6 cells.
awk '/--format <mode>/{n=gsub(/\|/,"|"); print "cli.md L38: "n" pipes (= "n-1" cells)"}' docs/cli.md
# Expected: 7 pipes = 6 cells (a markdown row with N cells has N+1 pipes). !=7 means a broken row.

# Grep guard 6 (scope — no other file changed).
git diff --name-only | grep -vE '^docs/cli\.md$|^docs/configuration\.md$'
# Expected: empty (no output).
git diff --name-only | grep -E '\.go$|README\.md|how-it-works\.md'
# Expected: empty (no Go / README / how-it-works touched — those are other tasks).
```

## Final Validation Checklist

### Technical Validation
- [ ] `git diff --name-only` == exactly {docs/cli.md, docs/configuration.md}
- [ ] `git diff --stat -- '*.go'` empty (docs-only)
- [ ] `make test` green (working tree otherwise clean — no behavioral regression)

### Feature Validation
- [ ] `rg -n 'gitmoji' docs/cli.md docs/configuration.md` → 4 hits (L38, L116, L216, L247); each mentions `+body`
- [ ] `rg -n 'gitmoji' … | rg -v '\+body'` → empty (no enumeration omits the suffix)
- [ ] `<base>[+body]` grammar stated at all 4 locations
- [ ] configuration.md L116 byte-matches bootstrap.go:376 (the shipped config init template comment)
- [ ] cli.md L38 row renders as 6 cells (7 pipes); trailing `Also [configuration].format.` preserved
- [ ] configuration.md L216 example shows `STAGECOACH_FORMAT=conventional+body stagecoach`

### Scope-Boundary Validation
- [ ] Only docs/cli.md + docs/configuration.md changed
- [ ] cli.md L486 (mapping table) + configuration.md L152 (defaults table) UNTOUCHED (no gitmoji/+body)
- [ ] NO Go code / test change (P1.M1 + parallel S1)
- [ ] NO README.md / docs/how-it-works.md change (Mode B, P1.M2.T2.S1)

### Code Quality & Docs
- [ ] Wording transcribes the shipped canonical form (root.go:202 / config.go:105 / bootstrap.go:376)
- [ ] Pipe escaping correct per surface (markdown cells `\|`; TOML comment literal)
- [ ] Edit points located by content (gitmoji grep), not stale line numbers
- [ ] Table cell counts preserved; alignment column at L116 preserved

---

## Anti-Patterns to Avoid

- ❌ Don't edit only L116. The contract's literal acceptance grep (`rg 'auto\|conventional\|gitmoji|plain'`)
  matches ONLY L116 (ripgrep `\|` = literal pipe; only L116's TOML comment has literal pipes). The
  contract's LOGIC (a)-(d) names all 4 locations and its STATED intent is "wherever the 4 bases are
  listed". Edit ALL 4 (L38, L116, L216, L247). Verify with `rg -n gitmoji | rg -v '\+body'` = empty.
- ❌ Don't unescape the in-cell pipes. cli.md L38, configuration.md L216, and configuration.md L247 are
  markdown TABLE CELLS — their in-cell `|` MUST stay escaped as `\|` (or backtick-wrapped). An unescaped
  `|` splits the cell and breaks the row. (configuration.md L116 is a TOML comment — pipes are literal
  there; do NOT escape them.)
- ❌ Don't diverge L116 from bootstrap.go:376. configuration.md L116 is the documented form of the
  `config init` template comment; it must BYTE-MATCH the shipped Go-source line (same words, same
  alignment column). A divergence is the exact docs-vs-binary disagreement this task fixes.
- ❌ Don't invent per-surface wording. The canonical phrasing is shipped (root.go:202 / config.go:105).
  Transcribe it (adapting only the escaping style per surface). Different wording per surface is the
  inconsistency this task removes.
- ❌ Don't touch the 2 non-enumeration format refs. cli.md L486 (flag/env/git mapping table) and
  configuration.md L152 (defaults table) list only `"auto"` as a VALUE — they do NOT enumerate the 4
  bases, so they don't trip the `gitmoji` gate and are correctly out of scope. Adding `+body` there
  would be wrong (those rows aren't format-value enumerations).
- ❌ Don't edit README.md or docs/how-it-works.md. README.md L63 (`Message shaping | --format (auto,
  conventional, gitmoji, plain)…`) and L196 (`stagecoach --format conventional`) are Mode-B changeset-
  docs territory (P1.M2.T2.S1, a SEPARATE task). The how-it-works format-section prose is also Mode B.
- ❌ Don't edit any Go file or test. The +body implementation (P1.M1) is Complete; the +body tests are
  the parallel P1.M2.T1.S1. This task is the 2 doc files only.
- ❌ Don't anchor to line numbers (38, 116, 216, 247). They drift on sibling edits. Locate the 4 points
  via `grep -n gitmoji docs/cli.md docs/configuration.md` (the reliable enumeration marker).
- ❌ Don't drop the trailing `Also [configuration].format.` clause on cli.md L38 or the example cell on
  configuration.md L216. Preserve every cell and every useful cross-reference; only the format-grammar
  wording changes.

---

## Confidence Score: 9/10

This is a 4-edit docs change across 2 markdown files, with the verbatim target text for all 4 edits
(findings §7), the exact current text + line numbers (findings §2), the canonical shipped wording to
match (findings §1 — root.go:202/config.go:105/bootstrap.go:376, all Complete), the acceptance-grep trap
and its correct resolution (findings §3 — edit all 4, verify with `gitmoji`), and the markdown-table
escaping rules (findings §5). No code, no tests, no schema. The one residual (not a full 10) is the
contract's literal acceptance grep being narrower than its intent (a trap an unwary implementer could
trip by editing only L116) — but the PRP spells out the trap and the correct `gitmoji` gate in the
Critical gotcha, the Blueprint, and the Validation Loop, so it is fully mitigated. One-pass success is
highly likely.