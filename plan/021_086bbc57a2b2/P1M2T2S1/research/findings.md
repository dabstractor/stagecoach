# P1.M2.T2.S1 Research Findings — README + overview coherence sweep across the +body delta

Source: `architecture/test_and_docs_surfaces.md` §README.md, a codebase-wide `grep`/`rg` sweep, and the
parallel PRP P1.M2.T1.S2 (which owns docs/cli.md + docs/configuration.md — OUT of my scope).

## 0. What's already done (NOT mine — do not touch)

P1.M2.T1.S2 (parallel, Complete/in-flight) synced these 4 surfaces to `<base>[+body]`:
- docs/cli.md L38 — ✅ has `<base>[+body]`
- docs/configuration.md L116 — ✅ has `<base>[+body]` (byte-matches bootstrap.go:376)
- docs/configuration.md L216 — ✅ has `<base>[+body]` (+ `conventional+body` example)
- docs/configuration.md L247 — ✅ has `<base>[+body]`
**My task does NOT touch docs/cli.md or docs/configuration.md.** Confirmed via grep — all 4 already show the grammar.

## 1. My surfaces — the Mode-B changeset-docs sweep (4 edit sites, 3 files)

### (a) README.md L63 — Message shaping features table row
CURRENT (single-line enumeration):
`| Message shaping | \`--format\` (auto, conventional, gitmoji, plain), \`--locale\`, \`--context\`, \`--template\` ([docs](docs/how-it-works.md#format-modes-and-locale)). |`
TARGET: insert the +body clause into the format list:
`| Message shaping | \`--format\` (auto, conventional, gitmoji, plain; append \`+body\` to force a subject+body), \`--locale\`, \`--context\`, \`--template\` ([docs](docs/how-it-works.md#format-modes-and-locale)). |`
- This is a single markdown table row; the edit is inline (no escaping needed — commas, not pipes, separate the bases).

### (b) README.md L196 — the format example line
CURRENT: `stagecoach --format conventional  # force conventional-commit style`
TARGET: add a companion line demonstrating +body (place immediately after L196 inside the same examples block):
`stagecoach --format conventional+body  # conventional-commit subject PLUS a descriptive body`
- The implementer should read the surrounding fenced block (it's a ```bash examples block) to place the line cleanly; the existing L196 stays unchanged.

### (c) docs/how-it-works.md — the "Format modes and locale" section (L289-300)
CURRENT: 4 bullets (auto/conventional/gitmoji/plain) + a closing paragraph "The multi-line rule and
subject-length target still apply in every mode." — NO mention of +body anywhere in the section.
TARGET: add a dedicated **`+body` paragraph** immediately AFTER the "multi-line rule still apply" paragraph
(L300) and BEFORE the `--locale` paragraph (L302). Mirror PRD §17.8 / FR-F9 wording:
> **Body forcing (`+body`).** Append `+body` to any format — `auto+body`, `conventional+body`,
> `gitmoji+body`, `plain+body` — to replace the multi-line rule with an unconditional directive: always
> follow the subject with a wrapped (~72-column) body explaining what the change does and why. Use this
> when you want explanatory bodies even on a repo whose history is single-line. The subject contract
> (`conventional` / `gitmoji` / `plain`) is unchanged by the suffix; `auto+body` keeps the learned subject
> style and forces only the body.
- IMPORTANT: do NOT edit the 4 individual bullets — the +body modifier is documented in its own paragraph
  (adding it to each bullet would be noisy and the bullets describe the BASE contract, not the suffix).
  This is the one surface where +body lives in a dedicated paragraph, not inline on every base line.

### (d) docs/index.md L58 — Features bullet (NEW find from the sweep; not in the contract's named list)
CURRENT: `- **Message shaping** — \`--format\` (conventional/gitmoji/plain), \`--locale\`, \`--template\`.`
TARGET: `- **Message shaping** — \`--format\` (conventional/gitmoji/plain; append \`+body\` for a subject+body), \`--locale\`, \`--template\`.`
- A single-line feature blurb; inline edit like (a).

## 2. The acceptance-grep nuance (per-surface, NOT blanket)

The contract's literal sweep `rg -n "conventional.*gitmoji.*plain|gitmoji.*plain" README.md docs/` matches
ONLY single-line enumerations (README L63, index.md L58, cli.md L38, configuration.md rows). It does NOT
match docs/how-it-works.md (whose 4 bases are on SEPARATE bullet lines). The research note explicitly says
"docs/how-it-works.md — Check for format lists," so how-it-works IS in scope by intent.

CORRECT per-surface gates (after my edits):
- README.md L63: `grep '+body' README.md` → the L63 line matches.
- docs/index.md L58: `grep '+body' docs/index.md` → the L58 line matches.
- docs/how-it-works.md: `sed -n '/Format modes and locale/,/--locale/p' docs/how-it-works.md | grep '+body'`
  → matches (the dedicated paragraph). Do NOT require every `gitmoji` bullet line to contain +body — that's
  a false-positive trap (the bullets describe bases; the modifier is its own paragraph).
- The 4 P1.M2.T1.S2 surfaces (cli.md L38, configuration.md L116/216/247) are unchanged by me and already
  pass — confirm via `rg -n gitmoji docs/cli.md docs/configuration.md` (4 hits, each already has +body).

## 3. Out of scope (correctly — verified by the "plain" sweep)

These `plain`/`gitmoji` hits are NOT format-base enumerations — leave them alone:
- README.md L441, docs/how-it-works.md L428/L438, docs/cli.md L43/L203/L207, docs/configuration.md L57,
  docs/ci-validation.md L29 — all are English "plain `git commit`" / "plain `config init`" / "plain `push`"
  usage, unrelated to the format grammar. (The full `grep -rn 'plain'` sweep confirmed no other format
  enumeration beyond the 8 total: my 4 + P1.M2.T1.S2's 4.)

## 4. Scope fences

- NO edit to docs/cli.md or docs/configuration.md (P1.M2.T1.S2's domain — already done).
- NO Go code / tests (P1.M1 Complete; P1.M2.T1.S1 parallel test suite).
- NO PRD.md / tasks.json / prd_snapshot.md / spec.
- NO new files — 4 edits across 3 existing markdown files (README.md, docs/how-it-works.md, docs/index.md).
- The shipped canonical wording to mirror: root.go --format help / config.go Format doc / bootstrap.go:376
  template (all Complete from P1.M1.T1.S4) + PRD §17.8 (the +body modifier, FR-F9).

## 5. Validation

- `make test` green (a docs edit cannot break Go tests — confirms the tree is otherwise clean).
- `go vet ./...` clean (no Go touched).
- Grep gates (§2): the 3 my-surfaces each show +body; the 4 sibling surfaces unchanged + already pass.
- Scope guard: `git diff --name-only` == exactly {README.md, docs/how-it-works.md, docs/index.md}.
- No make target for docs (`.markdownlint.json` exists with MD013/MD033/MD060 OFF but isn't wired to make);
  validation = grep gates + manual render + scope guard.