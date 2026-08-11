# Research Findings — P1.M2.T1.S2 (docs/cli.md + docs/configuration.md: --format / format → <base>[+body])

## 0. Task shape (one sentence)
Pure **Mode-A docs** edit to TWO files: sync the 4 stale format-base enumerations with the already-
shipped `<base>[+body]` grammar (PRD §9.19/§17.8, FR-F1/FR-F9). NO code, NO tests. The canonical
wording is LIVE in the binary (P1.M1.T1.S4, "Complete"); these docs lag it.

## 1. ⭐ The canonical wording is ALREADY SHIPPED (Mode A — docs must match the binary)
P1.M1.T1.S4 (Complete) established the canonical `<base>[+body]` phrasing in the shipped source:
- **root.go:202** (the `--format` flag help): `"Message format: <base>[+body] — auto|conventional|gitmoji|plain, append +body to force a "…`
- **config.go:105** (the `Format` field doc): `a <base>[+body] grammar where … the "+body" suffix forces a subject-plus-body message regardless of repo history (FR-F9)`
- **bootstrap.go:376** (the `config init` template comment): `# format = "auto"   # <base>[+body]: auto|conventional|gitmoji|plain, each optionally +body; unknown = hard error (exit 1)`

→ The docs edits below make the 4 doc surfaces AGREE with this shipped binary text. The docs/README
note ("if anything disagrees with `stagecoach --help`, the binary is authoritative") is the governing
principle: the binary is the source of truth; the docs are catching up.

## 2. The 4 stale enumeration locations (verified current text + line numbers)
`gitmoji` is the reliable marker — it appears in EXACTLY these 4 format-base enumerations (the two
other `format` refs — cli.md L486 flag/env/git mapping table, configuration.md L152 defaults table —
list only `"auto"` as a value, NOT the 4 bases, so they are correctly OUT of scope):

### (a) docs/cli.md L38 — the `--format` global-flags table row (Description cell)
CURRENT (backtick-wrapped, escaped pipes `\|` inside the markdown table cell):
```
Message format: `auto` (style learning) \| `conventional` \| `gitmoji` \| `plain`. An unknown mode is a hard error (exit 1). Also `[generation].format`.
```
Note: the row has 6 cells (`--format <mode>` | string | `auto` | `STAGECOACH_FORMAT` | `stagecoach.format` | <description>). Preserve the leading 5 cells + the trailing `Also \`[generation].format\`.` clause.

### (b) docs/configuration.md L116 — the File-format config template comment
CURRENT (TOML comment, LITERAL pipes — not a markdown table):
```
# format                = "auto"   # auto|conventional|gitmoji|plain; unknown = hard error (exit 1)
```
This MUST match the shipped bootstrap.go:376 template comment (the `config init` output). The alignment
column (the `#` after `"auto"`) lines up with the other template-comment fields above/below it
(locale L117, template L118).

### (c) docs/configuration.md L216 — the Environment-variables table row
CURRENT (escaped pipes `\|` in the markdown table cell):
```
Message format (auto\|conventional\|gitmoji\|plain; unknown = hard error) | `STAGECOACH_FORMAT=conventional stagecoach` |
```
Row = `STAGECOACH_FORMAT` | `--format` | <description> | <example>. The example cell shows the OLD
`STAGECOACH_FORMAT=conventional`; update it to `conventional+body` to demonstrate the suffix.

### (d) docs/configuration.md L247 — the Git-config-keys table row
CURRENT (backtick-wrapped bases, escaped pipes `\|`):
```
Message format: `auto` \| `conventional` \| `gitmoji` \| `plain`. Unknown = hard error (exit 1).
```
Row = `stagecoach.format` | string | `git config --get stagecoach.format` | <description>.

## 3. ⚠️ CRITICAL — the contract's acceptance grep is NARROWER than its intent
The contract's acceptance criterion (e): `rg -n "auto\|conventional\|gitmoji\|plain" docs/cli.md docs/configuration.md`
returns "no row that omits the +body option". **BUT ripgrep treats `\|` as a LITERAL pipe character**
(Rust regex: `|` = alternation, `\|` = escaped literal pipe). So this grep matches the literal substring
`auto|conventional|gitmoji|plain` (three literal pipes) — which appears ONLY at **configuration.md L116**
(the TOML comment with literal pipes). The other 3 locations use escaped `\|` (markdown cells) or
backtick-wrapped bases, so the literal-pipe pattern does NOT match them.

Verified live: `rg -n 'auto\|conventional\|gitmoji\|plain' docs/cli.md docs/configuration.md` → exactly
ONE hit (configuration.md:116).

**Resolution:** the contract's LOGIC (a)–(d) explicitly names all 4 edit locations with target text —
that is the real deliverable. The acceptance grep's literal-pipe semantics are a red herring; do NOT be
misled into editing only L116. Use the CORRECT verification grep that reliably finds all 4-base
enumerations:
```
rg -n 'gitmoji' docs/cli.md docs/configuration.md   # 4 hits (L38, L116, L216, L247); each must mention +body after the edit
```
(After the edit, every `gitmoji` line also mentions `+body`.) This is the gate that matches the contract's
STATED intent ("the grammar is stated wherever the 4 bases are listed").

## 4. Coordination with the parallel S1 (P1.M2.T1.S1) — CONTRACT
S1 is a **TEST-only** task (load_test / format_test / system_test — the +body regression suite). It does
NOT touch docs. It CONSUMES the same P1.M1.T1.S1–S4 implementation whose canonical wording this docs
task surfaces. So:
- S1 does NOT edit docs/cli.md or docs/configuration.md (no file overlap).
- Both S1 and S2 build on P1.M1.T1.S4's canonical `<base>[+body]` wording (shipped).
- The acceptance gate for S2 is the docs grep (`gitmoji` → +body at all 4 sites); for S1 it is the Go
  test suite. Independent, non-conflicting.

## 5. Markdown table gotchas (the load-bearing mechanical details)
- **cli.md L38, configuration.md L216, configuration.md L247 are markdown TABLE cells.** Pipes inside
  the Description cell MUST stay escaped as `\|` (or backtick-wrapped) — an unescaped `|` splits the
  cell and breaks the row. Preserve the existing escaping style per row.
- **configuration.md L116 is a TOML comment, NOT a table** — pipes are LITERAL (`auto|conventional|…`).
  Do NOT escape them there. (It must byte-match the bootstrap.go:376 template comment form.)
- **`<base>[+body]` uses angle brackets + square brackets** — these are fine in markdown table cells
  (no escaping needed; they render literally). Backtick-wrap the whole token `` `<base>[+body]` `` for
  consistency with the backtick style of the existing cells (cli.md L38, configuration.md L247).
- **configuration.md L116 alignment column:** the `#` inline comment must stay column-aligned with the
  other `[generation]` template-comment fields (work_desc_read_rounds L115, exclude L116, locale L117,
  template L118). Keep the same leading spaces before `=` and before `#`.
- Docs are NOT in `make` (`.markdownlint.json` exists with MD013/MD033/MD060 OFF but is not wired to a
  make target; `make lint` is golangci-lint for Go only). Validation = grep gates + manual render.

## 6. Scope fences (what NOT to do)
- NO Go code, NO tests (the +body implementation is P1.M1, Complete; the tests are the parallel S1).
- NO README.md edits here — README.md L63 (`Message shaping | --format (auto, conventional, gitmoji, plain)…`)
  and L196 (`stagecoach --format conventional`) are Mode-B changeset-docs territory (P1.M2.T2.S1), a
  SEPARATE task. Do NOT touch README.md.
- NO docs/how-it-works.md format-section rewrite (it's the prose spec §17.8 mirror; Mode B, later).
- NO PRD.md / tasks.json / prd_snapshot.md (read-only).
- The ONLY files touched: docs/cli.md + docs/configuration.md.
- Do NOT "fix" the cli.md L486 mapping table or configuration.md L152 defaults table — they don't
  enumerate the 4 bases (only `"auto"` as a value), so they're correctly untouched.

## 7. Exact target text (verbatim, to transcribe)

**(a) cli.md L38 Description cell** (preserve the 5 leading cells + trailing `Also [generation].format.`):
```
Message format: `<base>[+body]` — `auto` (style learning) \| `conventional` \| `gitmoji` \| `plain`; append `+body` to force a subject+body. Unknown = hard error (exit 1). Also `[generation].format`.
```

**(b) configuration.md L116** (byte-match bootstrap.go:376; keep alignment column):
```
# format                = "auto"   # <base>[+body]: auto|conventional|gitmoji|plain, each optionally +body; unknown = hard error (exit 1)
```

**(c) configuration.md L216 Description + Example cells** (mention +body; example shows conventional+body):
```
Message format: `<base>[+body]` — auto\|conventional\|gitmoji\|plain; append `+body` to force a subject+body; unknown = hard error (exit 1) | `STAGECOACH_FORMAT=conventional+body stagecoach`
```

**(d) configuration.md L247 Description cell** (backtick-wrapped bases, escaped pipes):
```
Message format: `<base>[+body]` — `auto` \| `conventional` \| `gitmoji` \| `plain`. Append `+body` to force a subject+body. Unknown = hard error (exit 1).
```