# Research Findings — P1.M1.T1.S4 (User-facing config/flag text for the +body grammar)

Verified by direct source read of `internal/config/config.go`, `internal/config/bootstrap.go`,
`internal/cmd/root.go`, `internal/config/load.go`, `internal/prompt/format.go` + grep across
`internal/**/*_test.go` (2026-08-08).

## 0. Task shape

S4 is **pure documentation text** (comment + string-literal edits). No code dependency, no tests to
write, no signature/config changes. Can be done in parallel with S1–S3 (the item says so). It updates
THREE user-facing surfaces so they describe the `<base>[+body]` grammar (FR-F1/FR-F9) instead of the
bare 4-mode list.

## 1. The canonical grammar (confirmed against landed code)

- **Grammar**: `<base>[+body]`, base ∈ {`auto`, `conventional`, `gitmoji`, `plain`}; an optional
  case-sensitive `+body` suffix forces a subject-plus-body message (FR-F9).
- **splitFormat** (S1, `internal/prompt/format.go:58`): `strings.HasSuffix(format, "+body")` →
  `TrimSuffix`. Suffix is exactly `"+body"`, case-sensitive.
- **validateFormat** (S3, **LANDED** — `internal/config/load.go:589`): strips one optional `+body`,
  validates the BASE against `validFormats` (the 4 bases, `load.go:571` — UNCHANGED). The error
  message is now the canonical phrasing S4's doc text must be consistent with:
  `invalid format %q (valid: <base>[+body], base ∈ auto, conventional, gitmoji, plain)` (`load.go:601`).
- **bodyForceDirective** (`format.go:24`): the FR-F9 unconditional body directive that `+body` swaps in.

➡ S4's three surfaces should use `<base>[+body]` + the 4-base list + "append/optionally +body to force
a subject+body", matching S3's error message and the PRD §15.2/§16.2 canonical wording (see §4).

## 2. The three exact current texts (verified line numbers — anchor on constructs)

### (a) `internal/config/config.go` — Format field doc (~L107-111)

```go
	// Format selects the commit-message style (PRD §9.19 FR-F1): "auto" (style learning, default),
	// "conventional", "gitmoji", or "plain". Resolved through the standard 5-layer precedence
	// (file → git → env → flag). Validated against validFormats at the tail of Load() — an unknown
	// mode is a hard error (exit 1). Consumed by S3 (prompt scaffolds).
	Format string `toml:"format"`
```

- CHANGE: the 4-mode enumeration → the `<base>[+body]` grammar; add the FR-F9 +body note; make the
  error clause grammar-aware ("unknown base or malformed suffix").
- KEEP VERBATIM: the "5-layer precedence" sentence + the "Validated against validFormats at the tail
  of Load()" reference + the trailing "Consumed by S3 (prompt scaffolds)." annotation (cross-plan
  task breadcrumb — NOT S4's job to renumber; leave it).

### (b) `internal/config/bootstrap.go` — config template comment (L376)

```go
# format                  = "auto"   # auto|conventional|gitmoji|plain; unknown = hard error (exit 1)
```

- This line lives inside a Go double-quoted string literal (the template constant). It has NO backtick,
  so it is a plain-text edit (no `+ "`" +` concatenation needed). `<`, `>`, `[`, `]` are all valid in
  a Go `"..."` string.
- CHANGE only the text AFTER the second `#`:
  - OLD: `auto|conventional|gitmoji|plain; unknown = hard error (exit 1)`
  - NEW: `<base>[+body]: auto|conventional|gitmoji|plain, each optionally +body; unknown = hard error (exit 1)`
- The leading `# format = "auto"   ` and its column alignment are PRESERVED (the change is purely the
  trailing comment text, so the aligned comment column is unaffected).

### (c) `internal/cmd/root.go` — `--format` flag help (L200-203)

```go
	pf.StringVar(&flagFormat, "format", "",
		"Message format: auto|conventional|gitmoji|plain (env STAGECOACH_FORMAT; git stagecoach.format; "+
			"[generation].format; default auto). Unknown mode is a hard error.")
```

- Two string literals joined with `+`. CHANGE the wording to the grammar; KEEP the env/git-config/
  config-file references + the default + the hard-error note.
- NEW (concise, terminal-appropriate — mirrors the item's suggested phrasing):
  - `Message format: <base>[+body] — auto|conventional|gitmoji|plain, append +body to force a subject+body (env STAGECOACH_FORMAT; git stagecoach.format; [generation].format; default auto). Unknown base or malformed suffix is a hard error.`
- Split across 2–3 `+`-joined literals (gofmt does NOT reflow string literals — the implementer picks
  the breaks; keep each line ≤ ~100 cols to stay gofmt-clean). `—` (em-dash U+2014) is UTF-8 and valid
  in a Go source string (the codebase already uses em-dashes, e.g. `antiReuseProhibition`). A plain
  `-` or `:` separator is an acceptable substitute if a minimal terminal is a concern.

## 3. No test pins the old wording (confirmed — the key risk for a doc-text task is a stale assertion)

```
grep -rn 'auto|conventional|gitmoji|plain|Message format:' internal/ --include=*_test.go   → EMPTY
grep -rn 'hard error (exit 1)' internal/config/*_test.go                                   → EMPTY
```

No `--help` golden-file test and no config-template snapshot test asserts these exact strings. So the
text edits break ZERO existing assertions. (The validateFormat error string is a SEPARATE surface
owned by S3 — S4 does not touch it.) S4's validation is therefore: `go build`/`go vet`/`gofmt`/`make
test`/`make lint` + a manual `--help` / `config init --template` smoke.

## 4. Canonical reference wording (PRD — the authoritative phrasing to mirror)

- **PRD §15.2 `--format` row**: `Message format: <base>[+body] — base auto (style learning) |
  conventional | gitmoji | plain; append +body to force a subject-plus-body message (§9.19, FR-F1/FR-F9).`
- **PRD §16.2 config example**: `format = "auto" # <base>[+body]: auto | conventional | gitmoji |
  plain, each optionally +body (§9.19, FR-F1/FR-F9)`

S4's in-code text adapts these to comment/help-string style (concise, no per-mode backticks in the
flag help) but uses the identical grammar tokens (`<base>[+body]`, the 4 bases, "optionally/append
+body"). Consistency with S3's `valid: <base>[+body], base ∈ ...` error message is the tie-breaker.

## 5. Scope fences (what S4 does NOT touch)

- Any CODE: `load.go` (validateFormat — S3 owns it), `format.go`/`planner.go` (S1), `system.go` (S2),
  `validFormats` (4 bases, unchanged), the `Format` field's struct tag or type, the flag's name/default.
- The `validateFormat` ERROR STRING (S3's surface — already landed with the `<base>[+body]` grammar).
- The "Consumed by S3 (prompt scaffolds)." annotation in config.go (cross-plan breadcrumb — preserve).
- Any OTHER format/config mention (docs/cli.md, docs/configuration.md, README — those are P1.M2.T1.S2
  and P1.M2.T2.S1). S4 is the THREE in-code surfaces only.
- No new tests (doc text has no test surface; the comprehensive +body suite is P1.M2.T1.S1).