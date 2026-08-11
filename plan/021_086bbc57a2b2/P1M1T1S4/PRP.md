name: "P1.M1.T1.S4 — User-facing config/flag text: config.go Format doc, bootstrap.go template comment, root.go --format help (the <base>[+body] grammar)"
description: >
  Update the THREE user-facing in-code text surfaces for the +body format modifier (FR-F1/FR-F9) from the
  bare 4-mode list to the <base>[+body] grammar: (a) the Format field doc comment in internal/config/
  config.go (~L107-111); (b) the `# format = "auto"` template comment in internal/config/bootstrap.go
  (L376); (c) the --format flag help string in internal/cmd/root.go (L200-203). PURE documentation text —
  no code dependency, no signature/config/flag-name changes, no new tests. Can be done in parallel with
  S1-S3 (S3's validateFormat error message has ALREADY landed with the `<base>[+body]` grammar at
  load.go:601 — S4's text mirrors it). Each surface keeps its existing precedence/env/git-config/
  hard-error references; only the format enumeration changes to the grammar. NO test asserts the old
  wording (grep-confirmed), so the edits break zero existing assertions. Downstream docs (docs/cli.md,
  docs/configuration.md, README) are P1.M2.T1.S2 / P1.M2.T2.S1 — NOT this subtask.

---

## Goal

**Feature Goal**: Make every in-code, user-facing surface that describes the `format` setting state the
`<base>[+body]` grammar (FR-F1) and the FR-F9 +body semantics, so users discover the new suffix from the
field doc, the generated config template, and `--help` — instead of only the 4 bare bases.

**Deliverable**: Three text edits (no code logic):
1. `internal/config/config.go` — `Format` field doc comment: 4-mode enumeration → `<base>[+body]` grammar + FR-F9 note.
2. `internal/config/bootstrap.go` — config template's `# format = "auto"` trailing comment: add `+body`.
3. `internal/cmd/root.go` — `--format` flag help string: add the grammar + the +body semantics.

**Success Definition**:
- All three surfaces describe `<base>[+body]` where base ∈ {auto, conventional, gitmoji, plain} and an
  optional `+body` forces a subject-plus-body message (FR-F9).
- The wording is consistent with S3's landed `validateFormat` error message
  (`valid: <base>[+body], base ∈ auto, conventional, gitmoji, plain`, `load.go:601`) and the PRD §15.2/§16.2
  canonical phrasing.
- Each surface PRESERVES its existing references: config.go keeps the 5-layer-precedence + validateFormat
  sentences; root.go keeps the env (`STAGECOACH_FORMAT`) / git-config (`stagecoach.format`) /
  config-file (`[generation].format`) / default (`auto`) / hard-error notes; bootstrap.go keeps the
  "unknown = hard error (exit 1)" note.
- `go build ./...`, `go vet ./...`, `gofmt -l`, `go test ./internal/config/... ./internal/cmd/...`,
  `make test`, `make lint` all pass (text-only edits; no test asserts the old wording).

## User Persona (if applicable)

**Target User**: Stagecoach users who read the config template (`config init`), the field doc (source /
godoc), or `--help` to learn what `format` accepts. Once S4 lands, a user running `stagecoach --help` or
reading their generated config sees `+body` is available and what it does — without reading the PRD.

**Use Case**: A user wants every commit to have a body regardless of repo history. Today no in-code surface
mentions `+body`; after S4, `--help` says "append +body to force a subject+body" and the config template
comment says "each optionally +body".

**Pain Points Addressed**: Discoverability of the FR-F9 +body modifier (the grammar exists in S1–S3 but is
invisible to users until these surfaces are updated).

## Why

- **FR-F1 (P1)**: the format grammar is `<base>[+body]`. S1 (splitFormat + prompt assembly) + S2
  (system.go threading) + S3 (validateFormat load gate) make the grammar FUNCTIONAL; S4 makes it
  DISCOVERABLE in the three places users look. Without S4, `--format conventional+body` works (S1–S3) but
  no `--help` / config text tells the user it exists.
- **Consistency with S3**: S3's `validateFormat` error already says `valid: <base>[+body], base ∈ …`
  (`load.go:601`, landed). S4 brings the proactive surfaces (doc/template/help) into line with the
  reactive surface (the error), so the grammar is described identically everywhere.
- **Why pure text / parallel-safe**: the three surfaces are comments and string literals — no code paths,
  no types, no flags. S4 has no dependency on S1–S3's logic (only on the grammar semantics, which are
  fixed). It can land before or after S1–S3 without conflict.

## What

**User-visible behavior**: `stagecoach --help`, `stagecoach config init` (template), and the `Format` field
doc all describe `<base>[+body]` + the +body semantics.

**Technical change (three text edits, no logic):** see Implementation Tasks for the exact old → new text.

### Success Criteria
- [ ] config.go `Format` doc: enumerates `<base>[+body]` + cites FR-F9; keeps precedence + validateFormat sentences
- [ ] bootstrap.go template comment: `# format = "auto"` comment names `<base>[+body]` + "each optionally +body"
- [ ] root.go `--format` help: states `<base>[+body]` + "append +body to force a subject+body"; keeps env/git/config/default/hard-error refs
- [ ] Wording consistent with S3's `valid: <base>[+body], base ∈ …` error + PRD §15.2/§16.2
- [ ] No code/signature/flag-name/config/test changes
- [ ] `go build ./...`, `make test`, `make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact current text of all three surfaces (copy-paste-ready old strings), the exact new text
for each, the canonical grammar + PRD wording to mirror, the confirmation that S3's error message has
landed (so the phrasing is pinned), the confirmation that NO test asserts the old wording (so the edits are
safe), the Go string-literal gotchas (backticks, gofmt non-reflow), and the scope fences are all below.

### Documentation & References

```yaml
- file: internal/config/config.go
  why: "The Format field doc comment (~L107-111). CHANGE the 4-mode enumeration to the <base>[+body] grammar;
        KEEP the 5-layer-precedence sentence, the validateFormat reference, and the trailing 'Consumed by S3
        (prompt scaffolds).' annotation verbatim (cross-plan breadcrumb — not S4's job to renumber)."
  pattern: "Edit the comment block above `Format string `toml:\"format\"``; make the error clause grammar-aware
            ('unknown base or malformed suffix')."
  gotcha: "Preserve 'Consumed by S3 (prompt scaffolds).' — do not delete or renumber it."

- file: internal/config/bootstrap.go
  why: "The config template constant's `# format = \"auto\"` line (L376). The line is INSIDE a Go double-quoted
        string literal but has NO backtick, so it is a plain-text edit (no `+ \"`\" +` join needed)."
  pattern: "Change ONLY the text after the second `#`; preserve the leading `# format = \"auto\"   ` and its
            column alignment (the change is purely the trailing comment, so the aligned comment column holds)."
  gotcha: "`<`, `>`, `[`, `]` are all valid inside a Go \"...\" string — no escaping needed. Do NOT introduce a
           backtick (would force string concatenation)."

- file: internal/cmd/root.go
  why: "The --format flag help (L200-203): two string literals joined with `+`. CHANGE the wording; KEEP the
        env (STAGECOACH_FORMAT) / git-config (stagecoach.format) / config-file ([generation].format) / default
        (auto) / hard-error references."
  pattern: "gofmt does NOT reflow string literals — split the new wording across 2-3 `+`-joined lines yourself,
            each ≤ ~100 cols. `—` (em-dash U+2014) is valid UTF-8 in a Go source string; a plain `-`/`:` is an
            acceptable substitute."
  gotcha: "The flag's NAME (`format`), default (`\"\"`), and the fs.Changed read path are UNCHANGED — only the
           help string text changes."

- docfile: plan/021_086bbc57a2b2/architecture/format_subsystem.md
  why: "§User-facing config/flag surfaces — the verified line numbers + current text for all three sites."
  section: "User-facing config/flag surfaces"

- docfile: plan/021_086bbc57a2b2/P1M1T1S4/research/findings.md
  why: "The exact old→new text per site (§2), the canonical grammar (§1), the no-test-pins confirmation (§3),
        the PRD canonical wording to mirror (§4), scope fences (§5). READ THIS before editing."

- docfile: plan/021_086bbc57a2b2/P1M1T1S3/PRP.md
  why: "S3 is the CONTRACT (in parallel). S3 LANDED validateFormat's error as `valid: <base>[+body], base ∈ …`
        (load.go:601) — S4's text mirrors this exact grammar token. S4 does NOT touch load.go."

- url: PRD §15.2 (--format row) + §16.2 (config example) + §17.8 (format modes / +body)
  why: "The authoritative user-facing wording. §15.2: `Message format: <base>[+body] — base auto (style
        learning) | conventional | gitmoji | plain; append +body to force a subject-plus-body message`.
        §16.2: `format = \"auto\" # <base>[+body]: auto | conventional | gitmoji | plain, each optionally +body`.
        S4 adapts these to comment/help style (concise; no per-mode backticks in --help)."
  critical: "Use the identical grammar tokens: `<base>[+body]`, the 4 bases in order, '+body' suffix
             (case-sensitive). Consistency with S3's error message is the tie-breaker on phrasing."
```

### Current Codebase tree (relevant slice)

```bash
internal/config/config.go      # Format field doc comment (~L107-111) ← EDIT
internal/config/bootstrap.go   # config template `# format = "auto"` comment (L376) ← EDIT
internal/cmd/root.go           # --format flag help string (L200-203) ← EDIT
internal/config/load.go        # validateFormat (S3 — LANDED; the error message is the grammar source of truth) — UNCHANGED
internal/prompt/format.go      # splitFormat + bodyForceDirective (S1 — LANDED; the grammar semantics) — UNCHANGED
```

### Desired Codebase tree with files to be added

```bash
internal/config/config.go      # MODIFY: Format field doc comment (4-mode list → <base>[+body] grammar + FR-F9)
internal/config/bootstrap.go   # MODIFY: `# format = "auto"` trailing comment (add +body)
internal/cmd/root.go           # MODIFY: --format flag help string (add grammar + +body semantics)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (pure text — no logic). The three surfaces are Go comments / string literals. Do NOT change any
//   field type, struct tag, flag name, flag default, validateFormat, validFormats, or any code path. The
//   ONLY behavioral surface touched is the text users read.

// CRITICAL (S3 landed the canonical phrasing). validateFormat's error is `valid: <base>[+body], base ∈
//   auto, conventional, gitmoji, plain` (load.go:601). S4's three surfaces MUST use the same grammar tokens
//   (`<base>[+body]`, the 4 bases in that order, the `+body` suffix) so every surface — proactive (doc/
//   template/help) and reactive (the error) — describes the grammar identically.

// GOTCHA (bootstrap.go is a Go string literal). The `# format = "auto" # ...` line is inside a double-quoted
//   Go string. It has NO backtick today and the new text needs none, so edit it as plain text (no `+ "`" +`
//   concatenation). `<`, `>`, `[`, `]` are valid in a "..." string. Preserve the leading `# format = "auto"   `
//   and the aligned comment column — change ONLY the text after the second `#`.

// GOTCHA (root.go help is +joined literals; gofmt won't reflow). The --format help is two `"..."` literals
//   joined with `+`. gofmt does NOT break string literals, so you choose the line breaks. Keep each line
//   ≤ ~100 cols. `—` (em-dash, U+2014) is valid UTF-8 in Go source (the codebase uses em-dashes elsewhere);
//   a plain `-` or `:` is fine if you prefer terminal-minimal safety.

// GOTCHA (config.go 'Consumed by S3 (prompt scaffolds).'). This trailing annotation is a cross-plan task
//   breadcrumb. PRESERVE it verbatim — do not delete it, do not renumber S3→S1 (prompt scaffolds is S1 in
//   THIS plan, but renumbering cross-plan annotations is out of scope for a doc-grammar task and risks churn).

// GOTCHA (no test pins the old wording — confirmed). grep across internal/**/*_test.go for
//   `auto|conventional|gitmoji|plain`, `Message format:`, and `hard error (exit 1)` returns EMPTY. So the
//   text edits break ZERO existing assertions. Still run the full config + cmd suites to confirm.

// SCOPE: S4 is the THREE in-code text surfaces only. Do NOT touch load.go (validateFormat — S3), format.go/
//   planner.go (S1), system.go (S2), validFormats (4 bases), the Format field type/tag, the flag name/default.
//   Do NOT touch docs/cli.md, docs/configuration.md, or README (P1.M2.T1.S2 / P1.M2.T2.S1). Do NOT add tests
//   (the comprehensive +body suite is P1.M2.T1.S1; doc text has no test surface).
```

## Implementation Blueprint

### Data models and structure

None. No types, structs, signatures, config keys, flags, or constants. Three text edits.

### Implementation Tasks (ordered by dependencies)

> **No prerequisites.** S4 is pure text and parallel-safe. S3's `validateFormat` error message HAS landed
> (`load.go:601`: `valid: <base>[+body], base ∈ auto, conventional, gitmoji, plain`) — it pins the grammar
> phrasing S4 mirrors. ANCHOR ON CONSTRUCTS via grep, not line numbers (drift possible):
> `grep -n 'Format selects the commit-message' internal/config/config.go`,
> `grep -n '# format .* = "auto"' internal/config/bootstrap.go`,
> `grep -n 'flagFormat, "format"' internal/cmd/root.go`.

```yaml
Task 1: EDIT the Format field doc in internal/config/config.go
  - FIND (the current comment block above `Format string `toml:"format"`):
        // Format selects the commit-message style (PRD §9.19 FR-F1): "auto" (style learning, default),
        // "conventional", "gitmoji", or "plain". Resolved through the standard 5-layer precedence
        // (file → git → env → flag). Validated against validFormats at the tail of Load() — an unknown
        // mode is a hard error (exit 1). Consumed by S3 (prompt scaffolds).
  - REPLACE WITH:
        // Format selects the commit-message style (PRD §9.19 FR-F1/FR-F9): a <base>[+body] grammar where
        // base ∈ {"auto" (style learning, default), "conventional", "gitmoji", "plain"} and an optional
        // "+body" suffix forces a subject-plus-body message regardless of repo history (FR-F9). Resolved
        // through the standard 5-layer precedence (file → git → env → flag). Validated against validFormats
        // at the tail of Load() — an unknown base or malformed suffix is a hard error (exit 1). Consumed by
        // S3 (prompt scaffolds).
  - PRESERVED: the "5-layer precedence" sentence, the "Validated against validFormats at the tail of Load()"
    reference, and the trailing "Consumed by S3 (prompt scaffolds)." annotation (verbatim).
  - DEPENDENCIES: none.

Task 2: EDIT the config template comment in internal/config/bootstrap.go
  - FIND (the `# format` line in the template constant, ~L376):
        # format                  = "auto"   # auto|conventional|gitmoji|plain; unknown = hard error (exit 1)
  - REPLACE WITH:
        # format                  = "auto"   # <base>[+body]: auto|conventional|gitmoji|plain, each optionally +body; unknown = hard error (exit 1)
  - CHANGE ONLY the text after the second `#` (OLD: `auto|conventional|gitmoji|plain; unknown = hard error (exit 1)`
    → NEW: `<base>[+body]: auto|conventional|gitmoji|plain, each optionally +body; unknown = hard error (exit 1)`).
    The leading `# format                  = "auto"   ` and its column alignment are UNCHANGED.
  - NO backtick introduced (the new text is plain); `<`, `>`, `[`, `]` are valid in the Go "..." string.
  - DEPENDENCIES: none.

Task 3: EDIT the --format flag help in internal/cmd/root.go
  - FIND (the pf.StringVar for "format", ~L200-203):
        pf.StringVar(&flagFormat, "format", "",
            "Message format: auto|conventional|gitmoji|plain (env STAGECOACH_FORMAT; git stagecoach.format; "+
                "[generation].format; default auto). Unknown mode is a hard error.")
  - REPLACE WITH (split across 3 +joined literals — gofmt-clean; adjust breaks as needed):
        pf.StringVar(&flagFormat, "format", "",
            "Message format: <base>[+body] — auto|conventional|gitmoji|plain, append +body to force a "+
                "subject+body (env STAGECOACH_FORMAT; git stagecoach.format; [generation].format; default "+
                "auto). Unknown base or malformed suffix is a hard error.")
  - PRESERVED: env (`STAGECOACH_FORMAT`), git-config (`stagecoach.format`), config-file (`[generation].format`),
    default (`auto`), and the hard-error note (now grammar-aware: "unknown base or malformed suffix").
  - The flag NAME (`format`), VAR (`flagFormat`), and default (`""`) are UNCHANGED.
  - `—` is an em-dash (U+2014); valid UTF-8 in Go source. If a minimal-terminal-safe variant is preferred,
    substitute ` - ` or `: ` — the grammar tokens (`<base>[+body]`, the 4 bases, `+body`) are what matter.
  - DEPENDENCIES: none.

Task 4: VERIFY — gates (text-only; no test asserts the old wording)
  - go build ./...                      # comments + strings compile; no syntax error introduced
  - go vet ./...
  - gofmt -l internal/config/ internal/cmd/   # Expected: empty (re-run `gofmt -w` on the 3 files if non-empty)
  - go test ./internal/config/... ./internal/cmd/...   # confirm no test broke (none pins the old wording)
  - make test && make lint
  - SMOKE (optional, manual): `go run ./stagecoach --help | grep -A1 -- '--format'` shows the grammar;
    `go run ./stagecoach config init --template` (or the populated init) shows the +body template comment.
  - DEPENDENCIES: Tasks 1-3.
```

### Implementation Patterns & Key Details

```go
// PATTERN: the grammar token set used identically in ALL THREE surfaces + S3's error message
//   <base>[+body]   base ∈ {auto, conventional, gitmoji, plain}   "+body" suffix (case-sensitive)
//   "+body" forces a subject-plus-body message (FR-F9).
// (Mirror S3's load.go:601 error: `valid: <base>[+body], base ∈ auto, conventional, gitmoji, plain`.)

// PATTERN: bootstrap.go — edit only the trailing comment, preserve alignment
//   # format                  = "auto"   # <base>[+body]: auto|conventional|gitmoji|plain, each optionally +body; unknown = hard error (exit 1)
//   (the `# format ... = "auto"   ` prefix + its column are untouched; no backtick introduced)

// PATTERN: root.go — +joined help literals, gofmt won't reflow, keep refs
//   "Message format: <base>[+body] — auto|conventional|gitmoji|plain, append +body to force a " +
//       "subject+body (env STAGECOACH_FORMAT; git stagecoach.format; [generation].format; default " +
//       "auto). Unknown base or malformed suffix is a hard error."
```

### Integration Points

```yaml
NO code/config/flag/API/test changes. Three text edits (comment + comment + help string).

CODE (text only):
  - internal/config/config.go      — Format field doc comment
  - internal/config/bootstrap.go   — config template `# format` comment
  - internal/cmd/root.go           --format flag help string

CONSISTENCY ANCHOR (already landed — mirror its grammar tokens):
  - internal/config/load.go:601 — validateFormat error `valid: <base>[+body], base ∈ auto, conventional, gitmoji, plain` (S3)

UNCHANGED:
  - load.go (validateFormat / validFormats — S3), format.go/planner.go (S1), system.go (S2)
  - the Format field type/struct tag; the flag name/default/var; any code path

DOWNSTREAM (do NOT implement in S4):
  - P1.M2.T1.S2: docs/cli.md + docs/configuration.md --format / format references → <base>[+body]
  - P1.M2.T2.S1: README + overview coherence sweep across the +body delta
  - P1.M2.T1.S1: comprehensive +body test suite (doc text has no test surface)
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...                              # comments + strings compile cleanly
go vet ./...
gofmt -l internal/config/ internal/cmd/     # Expected: empty. (If non-empty: `gofmt -w` the 3 edited files.)
make lint                                   # Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Confirm no test broke (NONE pins the old wording — grep-confirmed; the edit is text-only).
go test ./internal/config/... -count=1      # incl. TestValidateFormat (S3 — unaffected; S4 doesn't touch load.go)
go test ./internal/cmd/...    -count=1      # incl. any flag/help tests (none assert the format help text)

# Whole suite
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# Manual smoke — the two user-visible outputs S4 changes (optional but recommended):
# (a) --format help now states the grammar:
go run ./stagecoach --help 2>&1 | grep -A1 -- '--format'
# Expected: a line containing "<base>[+body]" and "+body".

# (b) the config template (or populated init) now comments +body:
go run ./stagecoach config init --template 2>/dev/null | grep '^# format'   # or the populated `config init`
# Expected: the `# format = "auto"   # <base>[+body]: ... each optionally +body; ...` line.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: all THREE surfaces now state the grammar
grep -rn '<base>\[+body\]' internal/config/config.go internal/config/bootstrap.go internal/cmd/root.go
# Expected: ≥3 matches (one per surface). Confirm each file has its match.

# Grep guard: each surface still carries its required references
grep -n '5-layer precedence' internal/config/config.go                # config.go keeps precedence
grep -n 'Consumed by S3 (prompt scaffolds)' internal/config/config.go # config.go keeps the breadcrumb
grep -n 'hard error (exit 1)' internal/config/bootstrap.go            # bootstrap keeps the hard-error note
grep -n 'STAGECOACH_FORMAT' internal/cmd/root.go                      # root.go keeps the env ref
grep -n 'stagecoach.format' internal/cmd/root.go                      # root.go keeps the git-config ref

# Grep guard: consistency with S3's landed error message (same grammar tokens)
grep -n 'valid: <base>\[+body\], base ∈' internal/config/load.go      # S3's anchor (UNCHANGED — just confirm it's there)

# Grep guard: the bare 4-mode list is GONE from the three surfaces (no stale "auto|conventional|gitmoji|plain" alone)
grep -rn 'Message format: auto|conventional\|# .*auto|conventional|gitmoji|plain; unknown' internal/config/config.go internal/config/bootstrap.go internal/cmd/root.go
# Expected: 0 matches (the OLD bare-list phrasing is gone; the new phrasing wraps it in <base>[+body]:).

# Scope-boundary guard: only the 3 text files changed
git diff --name-only
# Expected: only internal/config/config.go + internal/config/bootstrap.go + internal/cmd/root.go
#   (S4's own diff. S1/S2/S3 may have changed their files in parallel — S4's diff is these 3 only.)

# Scope-boundary guard: NO code change (the diff is comments + string literals only)
git diff internal/config/config.go internal/config/bootstrap.go internal/cmd/root.go | grep -E '^\+' | grep -vE '^\+\s*//|^\+\s*"|\+\+\+'
# Expected: empty (every added line is a comment `//` or a string literal `"..."` — no code lines added).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `gofmt -l internal/config/ internal/cmd/` empty
- [ ] `make lint` zero errors
- [ ] `make test` all pass (no test pins the old wording)

### Feature Validation
- [ ] config.go `Format` doc: `<base>[+body]` grammar + FR-F9 +body note; keeps precedence + validateFormat refs
- [ ] bootstrap.go template `# format` comment: `<base>[+body]: … each optionally +body`; keeps hard-error note
- [ ] root.go `--format` help: `<base>[+body]` + "append +body to force a subject+body"; keeps env/git/config/default refs
- [ ] All three consistent with S3's `valid: <base>[+body], base ∈ …` error + PRD §15.2/§16.2
- [ ] Manual smoke: `--help` and config template show the grammar

### Scope-Boundary Validation
- [ ] NO code change (diff is comments + string literals only — Level 4 grep guard)
- [ ] load.go (validateFormat/validFormats — S3), format.go/planner.go (S1), system.go (S2) UNCHANGED
- [ ] Format field type/struct tag + flag name/default/var UNCHANGED
- [ ] "Consumed by S3 (prompt scaffolds)." annotation PRESERVED verbatim
- [ ] NO docs/cli.md / docs/configuration.md / README edits (P1.M2.T1.S2 / P1.M2.T2.S1)
- [ ] NO new tests (P1.M2.T1.S1 owns the +body suite; doc text has no test surface)
- [ ] Only config.go + bootstrap.go + root.go changed

### Code Quality
- [ ] Grammar tokens identical across all three surfaces + S3's error
- [ ] Each surface's existing references (precedence / env / git-config / config-file / default / hard-error) preserved
- [ ] bootstrap.go column alignment preserved (only the trailing comment text changed)
- [ ] root.go help string gofmt-clean (manual line breaks; no over-long line)

---

## Anti-Patterns to Avoid

- ❌ Don't change any CODE — no field type/struct tag, no flag name/default, no validateFormat, no validFormats,
  no code path. S4 is comments + string literals ONLY (the Level 4 grep guard enforces this).
- ❌ Don't invent a different grammar token — use `<base>[+body]` + the 4 bases in order + the `+body` suffix,
  exactly as S3's landed error (`load.go:601`) and PRD §15.2/§16.2 state. Every surface must agree.
- ❌ Don't reorder or rename the bases (`auto, conventional, gitmoji, plain` — that order is canonical and S3's
  error cites it via `strings.Join(validFormats, ", ")`).
- ❌ Don't drop the required references — config.go keeps precedence + validateFormat; root.go keeps env
  (`STAGECOACH_FORMAT`) + git-config (`stagecoach.format`) + config-file (`[generation].format`) + default +
  hard-error; bootstrap.go keeps "unknown = hard error (exit 1)".
- ❌ Don't delete or renumber the "Consumed by S3 (prompt scaffolds)." annotation in config.go — it's a
  cross-plan breadcrumb; preserve it verbatim. (Prompt scaffolds may be S1 in this plan, but renumbering
  cross-plan annotations is out of scope and risks churn.)
- ❌ Don't introduce a backtick in the bootstrap.go template line — the current line has none and the new text
  needs none; a backtick would force `+ "`" +` string concatenation for no reason. `<`, `>`, `[`, `]` are fine
  in a `"..."` string.
- ❌ Don't rely on gofmt to reflow the root.go help string — gofmt does NOT break string literals. Choose the
  `+`-joined line breaks yourself, each ≤ ~100 cols.
- ❌ Don't touch docs/cli.md, docs/configuration.md, or README — those are P1.M2.T1.S2 / P1.M2.T2.S1. S4 is the
  THREE in-code surfaces only.
- ❌ Don't add tests for the doc text — the comprehensive +body suite is P1.M2.T1.S1; doc text has no test
  surface (and no test pins the old wording, so the edits are inherently safe).
- ❌ Don't touch load.go's validateFormat error string — S3 owns it and it has ALREADY landed with the
  `<base>[+body]` grammar. S4 MIRRORS it, doesn't edit it.

---

## Confidence Score: 10/10

This is a pure text-edit task with the exact old strings, exact new strings, the canonical grammar pinned by
S3's already-landed error message, the PRD's authoritative wording to mirror, a grep-confirmed absence of any
test pinning the old wording, and explicit scope fences (what to preserve verbatim, what not to touch). The
only conceivable failure mode — an implementer accidentally editing code or dropping a required reference — is
caught by the Level 4 grep guards (every added line is a comment/string; each reference still present; the
bare-list phrasing gone). No concurrency, no integration step, no new tests: `go build` + `make test` + the
two manual smokes (`--help`, config template) are the proof. The task is parallel-safe with S1–S3 (no code
dependency; only the grammar semantics, which are fixed by S1's splitFormat).