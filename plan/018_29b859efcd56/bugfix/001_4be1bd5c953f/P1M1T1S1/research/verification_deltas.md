# Research Notes — P1.M1.T1.S1 (preservedDefaultProvider helper)

Verification against the CURRENT working tree. The task description + `architecture/bug001_analysis.md`
are accurate. These notes record the exact verified anchors for a one-pass helper.

## VERIFIED — the helper's inputs/outputs (task contract)
- Signature: `preservedDefaultProvider(path string) string` in `internal/cmd/config.go`.
- Returns a validated BUILT-IN provider name (e.g. "claude"), or `""` (fall back to auto-detection)
  for: absent/unreadable file, inert file (no active [defaults].provider), or a custom/unknown name.
- Consumed by S2 (which wires it into runConfigInit). S1 = helper + unit test + doc comment ONLY.

## VERIFIED — no new imports needed (all four deps already imported in config.go)
config.go's import block already has: `os`, `strings`, `config` (internal/config), `provider`
(internal/provider). So `os.ReadFile`, `strings.Trim`, `config.ActiveSettings`,
`provider.NewRegistry` are all reachable with ZERO import edits.

## VERIFIED — ActiveSettings stores values VERBATIM (WITH surrounding quotes)
`config.ActiveSettings(content string) map[string]map[string]string` (internal/config/inert.go:38).
The doc comment (inert.go:36-37) states: "NOT unquoted — FR-B8/B9 carry values VERBATIM". So for
`provider = "claude"`, `out["defaults"]["provider"]` == the literal `"claude"` (WITH the double-quote
characters). MUST `strings.Trim(v, "\"")` to recover `claude`. Confirmed by reading the body: it does
`val := strings.TrimSpace(m[2])` only (no unquoting).

## VERIFIED — the provider registry API
- `provider.NewRegistry(nil)` (registry.go:35) → `*Registry` (built-ins only; nil arg = no user overrides).
- `reg.Get(name string)` (registry.go:57) → `(Manifest, bool)`. `ok==true` ⇒ name is a known built-in.
- This is the SAME call shape runConfigInit already uses to validate `--provider` (config.go ~474-478).

## VERIFIED — the pattern to mirror: mergeExistingActiveSettings (config.go:446)
```go
func mergeExistingActiveSettings(path, freshContent string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return freshContent // absent or unreadable → nothing to merge
	}
	return config.MergeActiveSettings(freshContent, string(data))
}
```
`preservedDefaultProvider` mirrors this read+ActiveSettings shape exactly: `os.ReadFile(path)` → on
error return "" → `config.ActiveSettings(string(data))` → look up `["defaults"]["provider"]`.

## VERIFIED — the call site it will feed (S2 wires it; S1 does NOT touch runConfigInit)
runConfigInit (config.go:462) currently:
- `providerName, _ := cmd.Flags().GetString("provider")` (line ~471)
- validates providerName (if != "") via `provider.NewRegistry(nil).Get` (~474)
- `content = config.GenerateBootstrapConfig(providerName)` (~480)  ← providerName="" ⇒ auto-detect pi
- `force, _ := cmd.Flags().GetBool("force")` (~486); `if force { content = mergeExistingActiveSettings(path, content) }` (~489)

S2 will insert `if force && providerName == "" { providerName = preservedDefaultProvider(path) }` BEFORE
the GenerateBootstrapConfig call. S1 ONLY adds the helper; it must NOT edit runConfigInit.

## VERIFIED — test home + idiom
internal/cmd/config_test.go is `package cmd` (internal test) ⇒ can call `preservedDefaultProvider(path)`
directly. The closest precedent is `TestConfigInit_Force_PreservesActiveSettings` (config_test.go:581),
which writes a temp config and exercises the force path. The helper test is SIMPLER: write a temp file,
call the helper, assert the string. Use `t.TempDir()` + `os.WriteFile`. Table-driven over the edge cases.

## EDGE CASES the helper + test must cover
| Input file state | Expected return |
|------------------|-----------------|
| absent (path does not exist) | "" (read error → auto-detect) |
| unreadable (os.ReadFile err) | "" |
| inert (all-commented, no active settings) | "" (no defaults.provider key) |
| active `[defaults] provider = "claude"` (known built-in) | "claude" |
| active `provider = "codex"` (known built-in) | "codex" |
| active `provider = "myagent"` (custom/unknown) | "" (don't break custom providers) |
| active `provider = "pi"` (known built-in, multi-backend) | "pi" |
| provider key under a DIFFERENT section (e.g. [generation]) | "" (only [defaults].provider counts) |
| value with single quotes or no quotes (degenerate TOML) | "" or the trimmed token if it happens to validate; ActiveSettings keeps verbatim so strings.Trim only strips `"` |

## SCOPE BOUNDARIES (sibling subtasks — do NOT implement here)
- **P1.M1.T1.S2**: wire `preservedDefaultProvider` into runConfigInit (the `if force && providerName==""`
  insertion + moving the `force` flag read up). S1 is the helper ONLY — do NOT touch runConfigInit.
- **P1.M1.T2.S1**: BUG-002 (newer-than-binary version notice). Different function (configVersionNotice).
- **P1.M1.T3.S1**: docs/cli.md update. S1's doc edit is ONLY the Go doc comment on the helper.
- Do NOT: change ActiveSettings/MergeActiveSettings/bootstrap.go/StagerFallback; change commit/render
  behavior; or add a new import.