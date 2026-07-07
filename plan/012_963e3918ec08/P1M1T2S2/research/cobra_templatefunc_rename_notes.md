# Research: Rename cobra template function `stagehandFlagUsages` → `stagecoachFlagUsages`

> **Purpose:** Pin the exact edit for P1.M1.T2.S2 — the cobra usage-template function rename in the
> stagehand→stagecoach project rename. S1 (P1.M1.T2.S1) explicitly defers this token to S2 (gotcha G5:
> "do NOT rename stagehandFlagUsages — that cobra template func is P1.M1.T2.S2's territory"). All line
> numbers verified live on 2026-07-07.

---

## 1. The exact target (verified — `internal/cmd/root.go`)

The token `stagehandFlagUsages` appears **exactly 5 times**, ALL in `internal/cmd/root.go` (grep-confirmed:
no other .go file references it). It is a STRING LITERAL name under which a cobra template func is
registered — NOT a Go identifier. The Go function passed in (`flagUsagesWrapped`) does NOT contain
"stagehand" and is UNCHANGED.

```go
// root.go:240 (comment)
//   ... We swap that call for the stagehandFlagUsages template func (registered
// root.go:246 (the registration — the string IS the func's template-namespace name)
	cobra.AddTemplateFunc("stagehandFlagUsages", flagUsagesWrapped)
// root.go:247-250 (the usage-template swap via strings.NewReplacer)
	rootCmd.SetUsageTemplate(strings.NewReplacer(
		".LocalFlags.FlagUsages ", "stagehandFlagUsages .LocalFlags ",       // :248 — NEW value
		".InheritedFlags.FlagUsages ", "stagehandFlagUsages .InheritedFlags ", // :249 — NEW value
	).Replace(rootCmd.UsageTemplate()))
// root.go:253 (comment)
// flagUsagesWrapped is the stagehandFlagUsages template func: ...
// root.go:258 (the Go function — UNCHANGED, no "stagehand" in its name)
func flagUsagesWrapped(fs *pflag.FlagSet) string { return fs.FlagUsagesWrapped(helpWrapWidth()) }
```

### Why the two `.Replace` targets MUST change with the registration
`strings.NewReplacer` swaps cobra's default template substring `.LocalFlags.FlagUsages ` →
`stagehandFlagUsages .LocalFlags ` (i.e. the rendered template becomes `{{stagehandFlagUsages .LocalFlags}}`).
cobra resolves that template-func name at RENDER time via the `AddTemplateFunc` registration. If the
registration is renamed to `stagecoachFlagUsages` but the replacer target still emits `stagehandFlagUsages`,
the rendered template references an UNDEFINED function → `text/template` errors at `--help` time. So all
three code sites (registration + 2 replacer targets) + the 2 comments must rename IN LOCKSTEP.

## 2. The single safe edit

One sed, scoped to root.go, renames all 5 occurrences of the precise token:
```bash
sed -i 's/stagehandFlagUsages/stagecoachFlagUsages/g' internal/cmd/root.go
```

**Token-boundary safety (verified):** the pattern `stagehandFlagUsages` does NOT match:
- `.LocalFlags.FlagUsages ` / `.InheritedFlags.FlagUsages ` — cobra's DEFAULT template substring (the
  replacer's SEARCH side; must stay so the swap still finds cobra's default). No "stagehand" prefix.
- `flagUsagesWrapped` — the Go function (the value passed to AddTemplateFunc). No "stagehand" prefix.
- `pflag.FlagUsagesWrapped(...)` — the pflag method. No "stagehand" prefix.

So the sed touches exactly the 5 intended tokens and nothing else. The replacer's SEARCH side
(`.FlagUsages `) is preserved; only the NEW-value side (which contains `stagehandFlagUsages`) changes.

## 3. Why this is the whole task (no other site)

`grep -rn 'stagehandFlagUsages' --include='*.go' .` (excl. plan/) → only `internal/cmd/root.go` (5 hits).
There is no second registration, no test that references the token by name, no doc. The rename is fully
contained in one file.

## 4. Validation — the help-render test is the functional proof

`internal/cmd/root_test.go:428` defines **`TestHelp_FlagsWrappedWithinWidth`** which does:
```go
rootCmd.SetArgs([]string{"--help"})
... err := Execute(ctx) ...  // renders the usage template → calls the stagecoachFlagUsages func
```
This test EXERCISES the usage template end-to-end. If the registration (`stagecoachFlagUsages`) and the
template token emitted by the replacer mismatch, cobra's `text/template` fails with
`function "stagecoachFlagUsages" not defined` → Execute returns an error → the test fails. So this test is
the deterministic functional gate for "the cobra help rendering works correctly with the new name."

### ⚠️ The contract's `-run TestRoot` filter is TOO NARROW
The contract's gate is `go test ./internal/cmd/... -run TestRoot -count=1`. That filter matches only
`TestRoot_*` tests — it does NOT match `TestHelp_FlagsWrappedWithinWidth` (no `TestRoot` prefix). A broken
template would PASS the contract's narrow gate and only fail at real `--help` time. **Recommendation:
broaden the gate to `-run 'TestRoot|TestHelp'` (or run the whole `./internal/cmd/` suite).** This is the
one substantive correction to the contract.

## 5. Scope boundaries (do NOT do)

- Do NOT rename the Go function `flagUsagesWrapped` (no "stagehand" in it; it's the value, not the name).
- Do NOT touch `.LocalFlags.FlagUsages ` / `.InheritedFlags.FlagUsages ` (cobra's default template
  substring — the replacer's search side; must stay).
- Do NOT rename `STAGEHAND_*` env-var literals or `stagehand.*` git-config keys (those are P1.M2.T1,
  still present in root.go's flag help text — out of scope here).
- Do NOT rename user-facing strings / `.stagehandignore` raw literals (P1.M2.T2/T3).
- Do NOT touch any other file (the token is root.go-only).
- Do NOT edit `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

## 6. Decisions log

| # | Point | Decision | Why |
|---|---|---|---|
| D1 | One sed or 5 manual edits? | One sed `s/stagehandFlagUsages/stagecoachFlagUsages/g` on root.go | The token is precise (won't match `.FlagUsages`/`flagUsagesWrapped`); 5/5 occurrences are the exact token; sed guarantees lockstep consistency (a manual miss on one of the 3 code sites breaks the template). |
| D2 | Test gate | Broaden `-run TestRoot` → `-run 'TestRoot\|TestHelp'` (or full `./internal/cmd/`) | `TestHelp_FlagsWrappedWithinWidth` renders `--help` and is the only test that catches a registration/template-token mismatch. The contract's `-run TestRoot` filter excludes it. |
| D3 | Rename comments (lines 240, 253)? | Yes (the same sed does it) | Consistency; comments reference the token by name. Cosmetic but the sed handles them for free. |
| D4 | Touch the replacer search side `.FlagUsages `? | NO | That's cobra's DEFAULT template substring the replacer searches FOR; renaming it would break the swap. Only the NEW-value side (containing `stagehandFlagUsages`) changes. |
