# Research Findings — P1.M1.T1.S3 (load.go: extend validateFormat to the <base>[+body] grammar)

Verified by direct source read of `internal/config/{load,load_test}.go`, `internal/prompt/format.go`,
the S1/S2 PRPs, and `plan/021_086bbc57a2b2/architecture/format_subsystem.md`. No files modified.

## 1. S1 CONTRACT — splitFormat is LANDED but UNEXPORTED in package `prompt`

S1 ("Implementing") added to `internal/prompt/format.go:58`:
```go
func splitFormat(format string) (base string, forceBody bool) {
	if strings.HasSuffix(format, "+body") {
		return strings.TrimSuffix(format, "+body"), true
	}
	return format, false
}
```
- Pure, case-sensitive on "+body", NO base validation (S3 owns that).
- Edge behavior (well-defined, documented by S1): `splitFormat("+body")` → `("", true)`; `splitFormat("conventional+body+body")` → `("conventional+body", true)`; `splitFormat("Conventional+Body")` → `("Conventional+Body", false)`.

## 2. ⚠️ CRITICAL — validateFormat CANNOT call splitFormat (cross-package, unexported)

`validateFormat` lives in package `config` (`internal/config/load.go:577`). `splitFormat` is an
**unexported** func in package `prompt` (`internal/prompt/format.go`). Go forbids the cross-package call.

Package graph (verified):
- `internal/config/load.go` imports ONLY: `context, fmt, os, strconv, strings, time, github.com/spf13/pflag`. **ZERO `stagecoach/internal/*` imports** — config is a foundational layer.
- `internal/prompt/*.go` does NOT import `internal/config` (grep clean) → no existing cycle, but a new `config → prompt` import would be a NEW one-way dependency that inverts the layering (config is loaded before/underneath prompt; prompt builds on config concepts, not vice-versa).

**Resolution (recommended): INLINE the trivial split in validateFormat.** The grammar split is 2 lines
(`strings.HasSuffix(format, "+body")` + `strings.TrimSuffix`). Inlining keeps `config` self-contained,
adds no cross-package dependency, and is byte-for-byte behavior-identical to splitFormat (same stdlib
calls, same case-sensitivity). The item's literal `base, forceBody := splitFormat(format)` is the LOGIC
to implement, not a mandate to call the prompt-package function. `forceBody` is UNUSED in validation
(only `base` is checked against validFormats), so the inline form drops it: `base, _ := <split>`.

Rejected alternative: **export `prompt.SplitFormat` + import it in config.** This couples the config
loader to the prompt builder (an inverted dependency) for 2 lines of trivial string logic, AND it would
require editing S1's just-landed unexported function (a cross-subtask churn). Not worth it. (If the team
later wants a single source of truth, a shared `internal/grammar` package is the right home — out of
scope for this 1-point subtask.)

## 3. Current validateFormat (load.go:569-584) — the function to MODIFY

```go
// validFormats is the closed set of --format modes (PRD §9.19 FR-F1). Validation-only; ...
var validFormats = []string{"auto", "conventional", "gitmoji", "plain"}   // L571 — STAYS the 4 bases

// validateFormat returns nil iff format is one of validFormats, else an error ... PURE (no I/O); called
// ONCE at the tail of Load() on the FULLY RESOLVED cfg.Format (not per-layer ...). Locale is NOT validated.
func validateFormat(format string) error {                                 // L577
	for _, m := range validFormats {
		if format == m {
			return nil
		}
	}
	return fmt.Errorf("invalid format %q (valid: %s)", format, strings.Join(validFormats, ", "))   // L583
}
```

**Call site:** `load.go:208` — `if err := validateFormat(cfg.Format); err != nil { return nil, fmt.Errorf("format: %w", err) }`.
It runs at the tail of `Load()` on the FULLY RESOLVED `cfg.Format`, right before `validateTemplate`.
The error is wrapped as `"format: <err>"`. validateFormat stays PURE (no I/O) — directly unit-testable.

## 4. The grammar fix — validate base, ignore forceBody, surface the grammar in the message

Replace the `format == m` exact-match with a base-match after stripping an optional `+body`:
```go
func validateFormat(format string) error {
	base := format
	if strings.HasSuffix(format, "+body") {
		base = strings.TrimSuffix(format, "+body")
	}
	for _, m := range validFormats {
		if base == m {
			return nil
		}
	}
	return fmt.Errorf("invalid format %q (valid: <base>[+body], base ∈ %s)", format, strings.Join(validFormats, ", "))
}
```
- `validFormats` STAYS the 4 bases (the suffix is grammar, not a mode — item contract point 2/3).
- Message surfaces the grammar: `<base>[+body], base ∈ auto, conventional, gitmoji, plain`. The
  substring `"auto, conventional, gitmoji, plain"` is PRESERVED (existing test asserts it verbatim).

## 5. Edge-case truth table (verified against the inline logic)

| input | base after strip | in validFormats? | result |
| --- | --- | --- | --- |
| `"auto"` | `"auto"` | yes | nil ✓ (existing) |
| `"conventional"` | `"conventional"` | yes | nil ✓ (existing) |
| `"plain"` | `"plain"` | yes | nil ✓ (existing) |
| `"gitmoji"` | `"gitmoji"` | yes | nil ✓ (existing) |
| `"auto+body"` | `"auto"` | yes | nil ✓ **NEW valid** |
| `"conventional+body"` | `"conventional"` | yes | nil ✓ **NEW valid** |
| `"gitmoji+body"` | `"gitmoji"` | yes | nil ✓ **NEW valid** |
| `"plain+body"` | `"plain"` | yes | nil ✓ **NEW valid** |
| `"+body"` | `""` | no | error ✓ (reject +body alone) |
| `"bogus+body"` | `"bogus"` | no | error ✓ (reject unknown base) |
| `"conventional+body+body"` | `"conventional+body"` | no | error ✓ (reject doubled suffix) |
| `"conventional+Body"` | `"conventional+Body"` (case mismatch, no strip) | no | error ✓ (reject case variant) |
| `""` | `""` | no | error ✓ (existing) |
| `"emoji"` / `"AUTO"` / `" auto"` / `"gitmojii"` | unchanged | no | error ✓ (existing invalid set) |

The `<base>+body+body` rejection is elegant: HasSuffix strips ONE `+body` → base = `"<base>+body"` → not
in validFormats → error. No special-casing needed. Case-sensitivity is free (`strings.HasSuffix` is
case-sensitive), so `+Body`/`+BODY` don't strip and the un-stripped base fails the membership test.

## 6. Existing TestValidateFormat (load_test.go:1355) — must still pass; EXTEND for +body

```go
func TestValidateFormat(t *testing.T) {
	validModes := []string{"auto", "conventional", "gitmoji", "plain"}      // bare bases → nil
	invalidModes := []string{"", "emoji", "Conventional", "AUTO", "gitmojii", " auto"}
	// invalid asserts: err != nil AND err.Error() contains mode AND contains "auto, conventional, gitmoji, plain"
}
```
- The 4 bare bases still return nil (base-match with no suffix). ✓
- All 6 existing invalid modes still error (none has a valid base after strip). ✓
- The `"auto, conventional, gitmoji, plain"` substring is preserved in the new message. ✓
- ⇒ **Existing TestValidateFormat passes UNCHANGED.** (Behavior-preserving for the pre-existing set.)
- S3 EXTENDS it: add the 4 `+body` valid cases + the new invalid cases (`+body`, `bogus+body`,
  `conventional+body+body`, `conventional+Body`). The comprehensive cross-package +body suite
  (format_test.go/system_test.go) is P1.M2.T1.S1; S3 owns only the load.go validation tests.

## 7. S2 is system.go-only — NO overlap with S3

Verified from S2's PRP: "internal/config/load.go # S3 (validateFormat) — UNCHANGED in S2". S2 threads
forceBody through `BuildSystemPrompt`/`BuildFallbackPrompt` in `internal/prompt/system.go` only. S3
touches only `internal/config/load.go` (+ its test). Zero file overlap; both consume S1's splitFormat
concept (S2 calls it in-package; S3 inlines the logic). No conflict.

## 8. Scope boundaries (do NOT do in S3)
- Do NOT export `splitFormat` / import `internal/prompt` in `internal/config` (inline instead — §2).
- Do NOT touch `validFormats` (stays the 4 bases).
- Do NOT touch system.go (S2), format.go/planner.go (S1), config.go/bootstrap.go/root.go (S4).
- Do NOT validate locale (FR-F6: free-form, verbatim).
- Do NOT add the comprehensive cross-package +body suite (P1.M2.T1.S1) — S3 adds only load_test.go cases.
- Do NOT change validateFormat's signature or its call site (load.go:208) or purity.

## 9. Validation
Edit `internal/config/load.go` (validateFormat body + doc comment) + `internal/config/load_test.go`
(extend TestValidateFormat). Gates: `go build ./...`, `go vet ./...`, `gofmt -l internal/config/`,
`go test ./internal/config/ -run TestValidateFormat -v`, `go test ./internal/config/...`,
`make test`, `make lint`. No external libs; no new imports (strings already imported). Pure function —
no race concern, no integration step.