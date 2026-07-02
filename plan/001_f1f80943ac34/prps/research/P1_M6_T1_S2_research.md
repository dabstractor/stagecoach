# Research — P1.M6.T1.S2: internal/generate/dedupe.go — isDuplicate + firstLine

## 1. What the task builds (contract)

Two PURE, exported-ish helper functions in a NEW file `internal/generate/dedupe.go`
(the FIRST file in package `generate`; generate.go S1 + rescue.go S3 come later):

- `func firstLine(msg string) string` — the generated-message subject = the first
  `'\n'`-delimited line of `msg`, `strings.TrimSpace`'d. Ports the reference
  `subject = head -1 commit_msg` (reference_impl.md §1/§2; PRD FR30 "Extract the
  generated subject (first line of the message)").
- `func isDuplicate(subject string, subjects []string) bool` — EXACT, case-sensitive
  membership test of `subject` against `subjects` (PRD FR31/FR32 "exactly matches
  one of the 50"). Fuzzy / Levenshtein is explicitly v1.1, NOT here (decisions.md §3
  note; task description RESEARCH NOTE).

`subjects` is the `[]string` returned by `git.RecentSubjects(50)` (P1.M3.T4.S1 —
the dependency, already shipped in internal/git/log.go): trimmed, newest-first,
empties dropped. So `isDuplicate`'s inputs are ALREADY trimmed on both sides — it
is a pure `==` comparison.

## 2. How the consumer uses them (decisions.md §3 — the two-nested-loop orchestrator)

```
recentSubjects = git.RecentSubjects(50)
...
subject = firstLine(msg)
if !isDuplicate(subject, recentSubjects) { goto COMMIT }   // unique → done
rejected = append(rejected, subject)                        // dup → outer retry
```
This file supplies ONLY firstLine + isDuplicate (the "dedupe subject in git log
--format=%s -50" porting-map entry, decisions.md §9). The orchestrator (S1) wires
them into the OUTER duplicate-rejection loop (FR30-FR33). S1 depends on S2.

## 3. Trim-responsibility boundary (the key design decision)

- `firstLine` TRIMS its result (the generated subject may carry leading/trailing
  spaces or a trailing `\r` from CRLF; `head -1` + trim). `strings.IndexByte(msg,'\n')`
  + `strings.TrimSpace` handles `\r\n` because `\r` is whitespace TrimSpace strips.
- `git.RecentSubjects` (dependency) ALREADY trims each element.
- `isDuplicate` does NO trimming — it is a pure EXACT, case-sensitive `==`. This
  matches "EXACT" (FR31/FR32) and keeps the responsibility in firstLine/RecentSubjects.
  => The MOCKING scenario "leading/trailing whitespace ignored via trim" is satisfied
  by firstLine's trim (tested end-to-end: msg with whitespace first line → firstLine
  trims → isDuplicate exact-matches a history subject).

## 4. Edge cases to test (table-driven, MOCKING scenarios)

firstLine:
- single line (no '\n') → whole msg trimmed
- multi-line → ONLY line 1 trimmed (★ contract bullet)
- leading/trailing whitespace on line 1 → trimmed (★ contract bullet)
- CRLF line ending ("x\r\nbody") → "x" (TrimSpace strips trailing \r)
- empty "" → ""
- first line all whitespace ("   \nbody") → ""
- trailing newline only ("x\n") → "x"

isDuplicate:
- exact match → true (★ contract)
- no match → false
- case differs ("Feat: X" vs "feat: x") → false (★ FR31/FR32 exact, case-sensitive)
- near/paraphrase ("add parsers" vs "add parser"; "refactor the parser" vs
  "refactor parser") → false (★ fuzzy explicitly deferred)
- internal whitespace differs ("add  parser" vs "add parser") → false (exact = byte-equal)
- empty subjects slice / nil → false
- empty subject "" vs non-empty list → false (RecentSubjects drops empties, so ""
  is never a real dup; "" also can't be a real commit subject)

## 5. Codebase conventions confirmed

- Pure-function table test precedent: internal/provider/parse.go + parse_test.go,
  internal/prompt/payload.go + payload_test.go — white-box `package <name>` (NOT
  `<name>_test`), stdlib `testing` (+ `strings`) ONLY, `tc := tc` capture, `t.Run`
  subtests, `t.Errorf` with `%q`, no testify, no real-git.
- Package-doc ownership: ONE file owns `// Package <name>`; siblings use a PLAIN
  `package <name>` line preceded by an adjacent file-level comment (internal/git/log.go
  defers to git.go; internal/prompt/system.go + payload.go defer to examples.go).
  VERIFIED: gofmt -l + go vet clean on internal/git/ with this exact pattern.
  => dedupe.go (first file in package generate) uses a plain `package generate` line
     + file-level comment DEFERRING the package doc to generate.go (P1.M6.T1.S1).

## 6. Validation gates (NEW package internal/generate, mirrors log.go PRP gates)

- `go build ./internal/generate/` (compiles)
- `go vet ./internal/generate/` (clean; also typechecks _test.go)
- `test -z "$(gofmt -l internal/generate/)"` (formatted)
- `go test ./internal/generate/` (dedupe tests pass)
- `go test ./...` (whole-module integrity — nothing else breaks; baseline green)

## 7. Scope boundaries

ONLY create internal/generate/dedupe.go + internal/generate/dedupe_test.go. Do NOT
create generate.go (S1), rescue.go (S3), or any orchestrator/rescue/signal code.
Do NOT touch go.mod/go.sum (stdlib `strings` only). No go-git, no testify, no real
git binary (pure string functions). No README/docs/providers TOML (Mode A = godoc).
