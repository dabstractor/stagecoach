# S1 Implementation Notes — SessionMode manifest field

> Scope: P1.M1.T1.S1. Add `SessionMode *string` (`toml:"session_mode"`) to the Manifest struct between
> ProviderFlag and BareFlags; Resolve() default `""`; Validate() enum (`""|"append"`, nil passes). Pure
> schema addition — nothing reads it yet (consumed by S2 MergeManifest, S3 RenderMultiTurn, S4 pi value).
> Verified against live source 2026-07-05.

## 1. SessionMode is ABSENT repo-wide (confirmed)

`grep -rn "SessionMode\|session_mode" --include="*.go" internal/ pkg/ providers/` → zero matches. Genuinely
new field. The arch research-provider.md §1 is the authoritative placement/convention source.

## 2. The `*string` pointer-scalar convention (manifest.go:14-24 doc block)

go-toml/v2 has NO omitempty → optional SCALAR fields are `*string`/`*bool`: ABSENT in a user override
decodes to nil (→ inherit the built-in on merge); PRESENT (even `""`/`false`) decodes non-nil (→ override).
This is the ONLY way a field-by-field merge can honor an explicit override. SessionMode follows this
convention (it's a config-overridable scalar, default-empty). Mirrors ProviderFlag/PrintFlag/SystemPromptFlag.

## 3. Exact edit targets (3 spots in internal/provider/manifest.go)

### a. Struct — insert between ProviderFlag (line 59) and BareFlags (line 62)
PRD §12.1 fixes the TOML ordering: `session_mode` sits between `provider_flag` and `bare_flags`. New block:
```go
	// --- session continuation (multi-turn fallback, §9.24) ---
	// "" (default): provider cannot append turns across one-shot calls → multi-turn fallback unavailable
	//   for this provider (one-shot → rescue, unchanged). "append": re-invoking the same session id
	//   appends a turn the model can recall (pi: `--session-id <id> ... -p`, repeated). REQUIRES a
	//   verified append rendering (FR-T9); never set speculatively. nil => Resolve→"".
	SessionMode *string `toml:"session_mode"`
```

### b. Validate() — add the enum check after the Output block (line 110-112), before `return nil`
Mirrors the PromptDelivery/Output nil-tolerant enum pattern:
```go
	if m.SessionMode != nil {
		if *m.SessionMode != "" && *m.SessionMode != "append" {
			return fmt.Errorf("provider manifest %q: session_mode %q must be \"\" or \"append\"", m.Name, *m.SessionMode)
		}
	}
```
(nil passes — the absent case. Non-nil must be "" or "append".)

### c. Resolve() — add after the ProviderFlag default (line 162-164), before the Output block
```go
	if out.SessionMode == nil {
		out.SessionMode = strPtr("")
	}
```
Mirrors the ProviderFlag/PrintFlag/SystemPromptFlag clauses. Guarantees `*r.SessionMode` is safe after
Resolve (S3's RenderMultiTurn + S3's trigger gate deref it).

## 4. Defaults (per PRD §12.1 / FR-T8)

- Resolve default: `""` (NO session support — multi-turn unavailable). NOT "append".
- Only pi (S4 / P1.M1.T1.S4) sets `SessionMode: strPtr("append")` (FR-T9 verified). The other 7 builtins
  leave it nil → Resolve→"". S1 does NOT set any builtin value (S4's job).

## 5. Tests (internal/provider/manifest_test.go — extend existing functions)

Reuse same-package `strPtr`/`boolPtr` + the existing `Manifest{...}` literal style. Mirror:
- TestValidate_BadOutput_Errors (:285): `Manifest{Name:"x", Command:strPtr("x"), Output:strPtr("xml")}`
  → err non-nil + contains message. ADD TestValidate_BadSessionMode_Errors (SessionMode:strPtr("bogus")).
- TestValidate_NilEnumsAreOK (:294): `Manifest{Name:"x", Command:strPtr("x")}` → err==nil. SessionMode
  nil already passes (no edit needed, but the case covers it).
- TestResolve_AppliesDefaultsToNilOptionals (:337): ADD assertion `r.SessionMode != nil && *r.SessionMode == ""`.
- TestResolve_PreservesExplicitValues (:352): ADD `SessionMode: strPtr("append")` to the input → assert
  `*r.SessionMode == "append"` (explicit value preserved, NOT clobbered to "").

## 6. Scope discipline (S1 vs S2/S3/S4/S5)

S1 = the struct field + Resolve default + Validate enum + the 4 test extensions. NOTHING ELSE.
- NOT S1: MergeManifest scalar clause (S2 = P1.M1.T1.S2).
- NOT S1: RenderMultiTurn (the capability gate + multi-turn render variant) (S3 = P1.M1.T1.S3).
- NOT S1: setting SessionMode="append" on the pi builtin + providers/pi.toml (S4 = P1.M1.T1.S4).
- NOT S1: the manifest-schema doc update / providers.md / configuration.md (S5 = P1.M1.T1.S5, Mode A).
- The field is DEAD (unconsumed) after S1 — `go build` compiles, tests green, no behavior change. S3/S4
  wire the consumers.

## 7. gofmt

The struct fields are column-aligned (the type/tag column pads to the longest field). SessionMode is 11
chars (shorter than SystemPromptFlag/RetryInstruction); gofmt realigns the block. RUN `gofmt -w` after.
