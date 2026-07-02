# Research Notes — P1.M1.T2.S2: internal/ui/output.go (progress, color, TTY/NO_COLOR)

## 1. Contract source (tasks.json context_scope)
- **INPUT:** none — Output constructs its state from env (`NO_COLOR`) + fd stat (TTY check on stdout).
- **LOGIC:** `Output` struct holding stdout/stderr writers, `color` bool, `verbose` bool.
  - `Progressf(format, args)` → ALWAYS stderr.
  - `Resultf(format, args)` → stdout (subject + diff-tree name-status lines, cf. FR42 `[<sha>] <subject>` + `git diff-tree --name-status`).
  - `Verbosef(format, args)` → stderr ONLY when `verbose` (FR50).
  - Color enabled iff **stdout is a TTY AND `NO_COLOR` unset AND no `--no-color`** (FR51). Minimal ANSI helper, NO dependency.
- **MOCKING:** tests inject `bytes.Buffer` writers + set/unset `NO_COLOR`; assert `Progressf`→stderr, `Resultf`→stdout, color codes absent when `NO_COLOR=1`.
- **OUTPUT:** `ui.Output` consumed by `cmd/stagehand` (M7.T2) and `generate/rescue` (M6.T1.S3, M6.T2.S1).
- **DEPENDENCIES (in plan):** none. Sibling `exitcode.go` (P1.M1.T2.S1) is COMPLETE and already defines `// Package ui` doc comment.

## 2. PRD anchors
- **FR18** (PRD line ~259): auto-stage notice `Nothing staged — staging all changes (3 files).` → a Progressf message (stderr, never stdout).
- **FR42** (line ~258 region): on success print `[<short-sha>] <subject>` + `git diff-tree --no-commit-id --name-status -r` → the Resultf payload (stdout).
- **FR50** (line ~318): `--verbose`/`-v`/`STAGEHAND_VERBOSE=1` → resolved command, raw agent stdout, retries → **stderr**. Maps to Verbosef.
- **FR51** (line ~319): Color when stdout is a TTY; disable via `--no-color` or `NO_COLOR`; **progress → stderr so stdout stays clean for piping**.
- **FR35** (line ~288): env prefix `STAGEHAND_`; includes `STAGEHAND_NO_COLOR` and `STAGEHAND_VERBOSE`.
- **§15.2 flags table** (line ~947): `--no-color` ↔ `STAGEHAND_NO_COLOR`, default "TTY-aware", "Disable color. Respects `NO_COLOR`."
- **§15.5 example** (line ~995): `stagehand --dry-run --no-color | tee /tmp/msg.txt` — the canonical pipe-clean use case this Output must protect.
- **decisions.md §3:** routes rescue/progress prints through a UI writer; RESCUE branch prints notices.

## 3. NO_COLOR convention (no-color.org) — authoritative
Spec (https://no-color.org): command-line software adding ANSI color by default should check `NO_COLOR`; when **present and not an empty string (regardless of value)**, it prevents the addition of ANSI color.
- Implementation chosen: `os.Getenv("NO_COLOR") == ""` ⇒ color ALLOWED (covers unset AND empty-string), non-empty (e.g. `"1"`) ⇒ DISABLED.
- This matches BOTH the contract ("NO_COLOR unset" enables) AND the test ("NO_COLOR=1" disables) AND the official convention.
- Note: `--no-color` flag and `STAGEHAND_NO_COLOR` env are CLI-layer concerns (M7.T2 wires cobra/pflag). `internal/ui` must NOT import cobra/pflag. The CLI folds `flagNoColor || envSet(STAGEHAND_NO_COLOR)` into the single `noColor bool` arg passed to `NewOutput`; Output itself reads only the standard `NO_COLOR` + stats the fd. Clean layering.

## 4. TTY detection — stdlib idiom (no dep)
external_deps.md sanctions ONLY cobra + go-toml/v2. No `golang.org/x/term`, no `mattn/go-isatty`, no `fatih/color`. Use raw stdlib:
```go
func isTerminal(w io.Writer) bool {
    f, ok := w.(*os.File)            // type-assert; non-files (bytes.Buffer, pipes) → false
    if !ok { return false }
    fi, err := f.Stat()
    if err != nil { return false }
    return fi.Mode()&os.ModeCharDevice != 0   // char device ⇒ terminal
}
```
- **Why this is exactly right for FR51:** a pipe (`| tee`, `| grep`) makes stdout a pipe, not a char device ⇒ `isTerminal==false` ⇒ color off ⇒ stdout stays byte-clean. A `bytes.Buffer` in tests is not `*os.File` ⇒ color off ⇒ matches "color absent" test. Both fall out of the same idiom for free.
- See: https://pkg.go.dev/os#FileStat , https://pkg.go.dev/os#ModeCharDevice

## 5. ANSI escape sequences (no dep)
Use raw `\x1b[...m` SGR sequences:
- reset `\x1b[0m`, bold `\x1b[1m`, red `\x1b[31m`, green `\x1b[32m`, yellow `\x1b[33m`, cyan `\x1b[36m`.
- Central conditional helper: `func (o *Output) Color(ansi, s string) string { if !o.color { return s }; return ansi + s + "\x1b[0m" }` + thin wrappers (Green/Yellow/Red/Bold/Cyan).
- Downstream callers wrap the specific token they want styled (e.g. rescue.go red notice; main.go green `[sha]`). Routing methods (Progressf/Resultf/Verbosef) do NOT auto-colorize — they only choose the stream, keeping concerns separate.

## 6. Testability design (two layers → full coverage without a fake TTY)
Because a `bytes.Buffer` is never a TTY, `NewOutput(buf,...)` always yields `color==false`. To still cover the color-ENABLED path:
1. **Pure predicate** `shouldColor(isTTY bool, noColorEnv string, noColorFlag bool) bool` — table-tested directly over all 2×2×2 corners (TTY×NO_COLOR×flag). This is where the FR51 boolean logic lives and is exhaustively verified.
2. **White-box struct construction** `&Output{stdout:buf1, stderr:buf2, color:true, verbose:true}` — lets a test force `color==true` and assert `Color()` emits ANSI while `color==false` emits none; and assert stream routing (Progressf→stderr buf, Resultf→stdout buf, Verbosef suppressed when verbose==false).
- Test is `package ui` (white-box, like exitcode_test.go). Stdlib `testing` only (NO go.mod change). Uses `t.Setenv("NO_COLOR", ...)` (Go 1.17+) for the env-toggle cases — safe, auto-restored, parallel-friendly.

## 7. Conventions inherited from sibling PRP (P1.M1.T2.S1) — MUST follow
- `package ui`; **plain `package ui` line with NO `// Package ui` doc comment in output.go** — that package doc already lives in exitcode.go; duplicating it fires revive/golint "duplicate package comment".
- Module path `github.com/dustin/stagehand`; import internal as `github.com/dustin/stagehand/internal/ui`.
- Table-driven tests, stdlib `testing` only, `t.Errorf` on mismatch, `tc := tc` + `t.Run`.
- Validation gates: `go build ./internal/ui/`, `go vet ./internal/ui/`, `test -z "$(gofmt -l internal/ui/)"`, `go test ./internal/ui/`, `go test ./...`. (No golangci-lint gate — none configured repo-wide; match prior PRP.)
- DOCS impact: none (README is Mode B, M8.T4). Create NO README/docs.

## 8. Scope boundaries (do NOT cross)
- Do NOT wire `--no-color`/`--verbose`/`STAGEHAND_*` parsing here — that is cmd/stagehand (M7.T2). Output receives resolved bools.
- Do NOT import cobra/pflag/go-toml or any color/term library. Stdlib only (fmt, io, os).
- Do NOT touch exitcode.go, go.mod, go.sum, main.go, Makefile.
- Do NOT add a package doc comment (lives in exitcode.go).

## 9. Confidence
One-pass success likelihood: **9/10.** Self-contained, no external deps, no cross-package contract to satisfy yet (consumers come later), exhaustive pure-predicate testability. Only residual risk is the package-doc-comment duplicate trap, which is explicitly called out.
