# Research Findings — P1.M3.T2.S1: Remove prefix from ErrEmptyMessage literal (Issue 5)

## 0. The bug — one literal, one prefix-doubling site

`internal/generate/finalize.go:45`:
```go
var ErrEmptyMessage = errors.New("stagecoach: empty commit message — aborted")
```
`cmd/stagecoach/main.go:67` (the CLI's single error-printing site):
```go
fmt.Fprintf(os.Stderr, "stagecoach: %v\n", err)   // prepends "stagecoach: " to ANY non-empty err
```
Combined output on the --edit empty-message abort (and the hooks-empty-message guard):
`stagecoach: stagecoach: empty commit message — aborted` ← **double prefix** (Issue 5).

## 1. The fix — change ONE literal; sentinel identity is PRESERVED

Change finalize.go:45 to:
```go
var ErrEmptyMessage = errors.New("empty commit message — aborted")
```
`errors.New` returns a `*errors.errorString` holding the string. `errors.Is(err, ErrEmptyMessage)` compares
the POINTER (identity), NOT the message text. So changing the literal's TEXT does NOT change the sentinel's
IDENTITY — every `errors.Is(err, ErrEmptyMessage)` check still returns true. This is the crux: the fix is
safe because the codebase uses `errors.Is` (identity) everywhere, never a string match on this error.

After the fix, main.go:67's `"stagecoach: %v"` produces the correct single-prefixed:
`stagecoach: empty commit message — aborted`.

**EM-DASH GOTCHA (critical for the edit):** the literal uses a Unicode EM DASH `—` (U+2014; UTF-8 bytes
`E2 80 94`, shown as `M-bM-^@M-^T` under `cat -v`), NOT a hyphen `-`. The edit's oldText/newText MUST
preserve this exact character. A plain hyphen would (a) fail to match the oldText and (b) change the
user-visible message text. Copy the exact byte from the existing line.

## 2. The propagation paths — ALL return the BARE sentinel (no re-prefixing)

The sentinel is propagated BARE (no `%w` wrap that would add its own prefix) at every site. So the ONLY
prefix in the final output comes from main.go:67 — fixing the ONE literal fixes ALL paths simultaneously:

| Site | Code | Path |
|------|------|------|
| finalize.go:118 | `return "", ErrEmptyMessage` | --edit empty-message abort (§9.22 FR-E1) |
| generate.go:486 | `return Result{}, err` (bare) | single-commit --edit propagation |
| generate.go:517 | `return Result{}, ErrEmptyMessage` | single-commit hooks-empty-message guard |
| decompose.go:409 | `return DecomposeResult{}, editErr` (bare) | per-concept --edit abort (FR-E4) |
| message.go:213 | `return "", err` (bare) | decompose message EditMessage propagation |
| message.go:254 | `return "", generate.ErrEmptyMessage` | decompose hooks-empty-message guard |
| pkg/stagecoach/stagecoach.go:709,754 | `return Result{}, err` / `generate.ErrEmptyMessage` | public-API twins of the above |

`exitcode.go:65`: `if errors.Is(err, generate.ErrEmptyMessage) { return Error }` → exit 1. Identity-based;
unchanged by the literal edit. (exitcode_test.go:25 `{"ErrEmptyMessage → 1", generate.ErrEmptyMessage, Error}`
passes the sentinel VALUE to `For()` and checks the code — no string — stays GREEN.)

**No site wraps ErrEmptyMessage with `fmt.Errorf("stagecoach: %w", …)` or similar** (verified by reading
each return). So there is no SECOND prefix source to chase — main.go:67 is the only one.

## 3. NO existing test asserts on the literal string — "update test assertions" is a NO-OP

This is the key research result. The contract's step (b) says "Search for any test that asserts on the
literal string 'stagecoach: empty commit message' and update it." Exhaustive search found **NONE EXIST**.
Every test uses `errors.Is` (sentinel identity), which is invariant under the literal edit:

| Test (file:line) | Assertion form | Affected by literal edit? |
|------------------|----------------|---------------------------|
| generate_test.go:882 | `errors.Is(err, ErrEmptyMessage)` | NO (identity) |
| hooks_freeze_test.go:198-199 | `errors.Is(err, generate.ErrEmptyMessage)` | NO |
| message_test.go:546-547 | `errors.Is(err, generate.ErrEmptyMessage)` | NO |
| stagecoach_test.go:1681,1703 | `errors.Is(err, generate.ErrEmptyMessage)` | NO |
| exitcode_test.go:25 | `For(generate.ErrEmptyMessage) == Error` (value→code) | NO |

Exhaustive grep proof (run during research):
- `grep '.Error()' *_test.go | grep -i 'empty\|stagecoach:'` → ZERO hits for ErrEmptyMessage (the hits are
  UNRELATED: lock_contention "want empty (silent)", render "tooled mode requires...", message_test
  "empty concept diff" = a DIFFERENT error `ErrMessageFailed`).
- `grep 'strings.Contains' *_test.go | grep -i 'empty\|abort\|stagecoach:'` → ZERO hits for ErrEmptyMessage.
- `grep 'stagecoach: stagecoach\|empty commit message' internal/e2e/*_test.go` → ZERO (no e2e captures
  main.go's stderr output; the bug is purely user-facing).

**CONCLUSION:** there is no test to UPDATE. The implementer should NOT go hunting for a string assertion
that doesn't exist. (The architecture doc §Issue 5 line 130 phrases it defensively: "Any assertion on the
literal string … must be updated" — the honest answer is there are none.)

## 4. RECOMMENDED — add ONE pinning test (the positive proof + regression lock)

Since no test currently pins the literal, the rigorous move is to ADD a tiny unit test that asserts
`ErrEmptyMessage.Error()` equals `"empty commit message — aborted"` (no `"stagecoach: "` prefix). This:
- PROVES the fix (the literal no longer carries the prefix).
- LOCKS it against regression (a future edit re-adding `"stagecoach: "` fails the test).
- Satisfies the contract's "update test assertions" intent (ADD, since none exist to UPDATE).
- Costs ~6 lines; co-locates with the existing edit-abort test in generate_test.go.

Place it in `internal/generate/generate_test.go` near the existing `t.Run("fake editor empties message →
ErrEmptyMessage", …)` (line 858) — or as a standalone `TestErrEmptyMessage_NoStagecoachPrefix`. It tests the
PACKAGE-LEVEL sentinel directly (no harness needed):
```go
func TestErrEmptyMessage_NoStagecoachPrefix(t *testing.T) {
	// Issue 5: ErrEmptyMessage's literal must NOT start with "stagecoach: " — cmd/stagecoach/main.go:67
	// prepends "stagecoach: " to every error, so a prefixed literal would double to
	// "stagecoach: stagecoach: empty commit message — aborted". The sentinel identity (errors.Is) is
	// unaffected; this pins the LITERAL TEXT (the user-facing string after main.go adds its prefix).
	if strings.HasPrefix(ErrEmptyMessage.Error(), "stagecoach: ") {
		t.Errorf("ErrEmptyMessage literal has a 'stagecoach: ' prefix (%q); main.go:67 would double it",
			ErrEmptyMessage.Error())
	}
	const want = "empty commit message — aborted"
	if got := ErrEmptyMessage.Error(); got != want {
		t.Errorf("ErrEmptyMessage.Error() = %q, want %q", got, want)
	}
}
```
(This needs `"strings"` imported in generate_test.go — verify it's already imported; if not, add it.)

## 5. The godoc on finalize.go:41-44 is ALREADY correct — NO doc change needed

Lines 41-44:
```go
// ErrEmptyMessage is the §9.22 FR-E1 abort signal: ... The CLI
// maps it to exit 1 with "empty commit message — aborted" (NOT exit 3/124 — no manual-recovery recipe).
```
The godoc ALREADY describes the message WITHOUT the `"stagecoach: "` prefix — because it documents what
the USER sees (after main.go:67 adds the prefix). So the godoc is correct as-is; the literal on line 45 is
the only thing out of sync. After the fix, the literal matches the godoc. **Do NOT edit the godoc.**

## 6. Scope boundaries (no overlap)

- **P1.M3.T1.S2** (parallel, Implementing) — Issue 4 (token gate irreducible floor). Touches
  `internal/git/tokengate.go` + `internal/git/tokengate_test.go` + `internal/git/difftokenlimit_test.go`.
  ZERO overlap with `internal/generate/finalize.go`. No shared files.
- **P1.M3.T3.S1** (planned) — Issue 6 (auto-stage "(1 files)" grammar). Touches `internal/cmd/default_action.go`.
  Zero overlap.
- **P1.M4.T1** (planned) — docs sync (README/how-it-works/cli). This item: DOCS none (contract: "internal
  error message formatting, no user-facing config/API surface"). The fix changes a user-visible STRING but
  not any config/API/flag — it's a polish fix, not a feature doc.
- This item touches ONLY: `internal/generate/finalize.go` (1 literal) + OPTIONALLY `internal/generate/generate_test.go`
  (1 small pinning test). NO edit to main.go, exitcode.go, generate.go, decompose/*, pkg/stagecoach/*, or any PRD/task file.

## 7. Validation commands (verified against the codebase)

```bash
go build ./...                              # the literal edit compiles
go vet ./internal/generate/...
gofmt -l internal/generate/finalize.go internal/generate/generate_test.go   # empty
go test ./internal/generate/ ./internal/decompose/ -v   # the contract's command — all errors.Is tests GREEN + new pinning test
go test ./internal/exitcode/ -v                          # ErrEmptyMessage → exit 1 mapping GREEN
go test ./pkg/stagecoach/ -v                             # the public-API twins' errors.Is tests GREEN
make test ; make lint
git status --porcelain                                    # ONLY finalize.go (+ optionally generate_test.go)
```

`internal/generate` IS in the coverage-gate list (Makefile:77 gates `internal/{git,provider,generate,config}`).
The change is a one-token literal swap (no new branch) so coverage is unaffected; the optional pinning test
ADDS coverage of the sentinel literal. No coverage-threshold risk.

## 8. Why `errors.Is` is invariant under the literal edit (the safety argument)

`errors.New(s)` returns `&errors.errorString{s}`. `errors.Is(err, target)` first does `err == target`
(direct pointer compare) before any Unwrap chain. `ErrEmptyMessage` is a package-level var — its ADDRESS
does not change when you edit the string it holds. So:
- Before: `errors.Is(ErrEmptyMessage, ErrEmptyMessage)` → true (same pointer).
- After editing the literal: `errors.Is(ErrEmptyMessage, ErrEmptyMessage)` → STILL true (same pointer;
  only the string field's content changed).

Every propagation site returns the sentinel BARE (`return ErrEmptyMessage` / `return generate.ErrEmptyMessage`)
or propagates a bare `err` that IS the sentinel (`return Result{}, err` where err == ErrEmptyMessage). No
site does `errors.New(strings.TrimPrefix(...))` or reconstructs the sentinel. So the identity is preserved
end-to-end, and `exitcode.For`'s `errors.Is(err, generate.ErrEmptyMessage)` → `Error` (exit 1) holds.
