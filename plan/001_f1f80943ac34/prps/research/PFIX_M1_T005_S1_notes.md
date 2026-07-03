# Research notes — BUG-005 / PFIX_M1_T005_S1

## Bug
`cmd/stagehand/stage.go:139` — `maybeAutoStage` prints a hardcoded plural:
```go
out.Progressf("Nothing staged — staging all changes (%d files).\n", n) // FR18
```
For `n==1` this yields the grammatically incorrect `(1 files).`

## Root cause
The format string hardcodes the noun `files`; there is no singular branch and no
pluralize helper exists anywhere in the repo (`grep plural` → no hits).

## Fix (minimal, idiomatic Go)
Go has no ternary, so compute the noun before the call:
```go
noun := "files"
if n == 1 {
    noun = "file"
}
out.Progressf("Nothing staged — staging all changes (%d %s).\n", n, noun) // FR18
```
- Keeps the `// FR18` requirement tag.
- The existing `AutoStagesThenProceeds` subtest (n=3) still passes because
  `(3 files)` is unchanged — only the noun is now data, not literal.
- `Progressf` delegates to `fmt.Fprintf(stderr, format, args...)`
  (internal/ui/output.go:103), so `%d` + `%s` work identically.

## Tests (cmd/stagehand/stage_test.go — white-box `package main`)
Existing `fakeStager{fileCount: int}` already drives the count; extend
`TestMaybeAutoStage` with an n==1 subtest asserting stderr contains
`(1 file)` and does NOT contain `(1 files)`.

## Validation tools (Makefile)
- build: `go build ./...`
- vet:   `go vet ./...`
- fmt:   `gofmt -l .` (must be empty)
- test:  `go test ./...`
- lint:  `golangci-lint run` (optional; may be absent on CI box)

## Docs impact
None required. PRD.md FR18 example `(3 files)` is already plural-correct and
unaffected by the fix. Internal code comments in stage.go / ui/output.go /
git/stage.go describe the "(N files)" notice generically and stay accurate.

## Scope boundary
Touch ONLY `cmd/stagehand/stage.go` (the single call site) and
`cmd/stagehand/stage_test.go` (add one subtest). Do NOT introduce a shared
pluralize helper (over-engineering for one call site) and do NOT touch the
PRD/docs.
