# Research Findings — P1.M1.T1.S2 (Wire preservedDefaultProvider into runConfigInit)

## 1. S1 status + the contract being consumed

S1 ("Implementing" in parallel) has NOT landed yet (`grep -c preservedDefaultProvider internal/cmd/config.go` == 0).
Treat S1's PRP as the CONTRACT. It produces:
- `func preservedDefaultProvider(path string) string` in internal/cmd/config.go (near mergeExistingActiveSettings,
  line 446). Reads the existing config, extracts `[defaults] provider` via `config.ActiveSettings`, strips the
  verbatim TOML quotes (`strings.Trim(v, "\"")`), validates against `provider.NewRegistry(nil).Get`, and returns
  the built-in name — or "" for absent/inert/unknown/custom (fall back to auto-detect).
- `TestPreservedDefaultProvider` in config_test.go (unit test for the helper — S1 owns it; S2 must NOT duplicate).

S2 CONSUMES `preservedDefaultProvider` and wires it into `runConfigInit`. S2 does NOT add or modify the helper.

## 2. The current runConfigInit (internal/cmd/config.go:459-506) — verified structure

```go
func runConfigInit(cmd *cobra.Command, args []string) error {
	if interactive, _ := cmd.Flags().GetBool("interactive"); interactive {   // interactive wizard — TOP, unaffected
		return runConfigInitInteractive(cmd, args)
	}
	path := config.ResolveConfigPath(flagConfig)        // :465  ← in scope for the whole func
	tmpl, _ := cmd.Flags().GetBool("template")           // :466
	var content string                                   // :467
	if tmpl {                                            // :468  -- --template branch (unaffected)
		content = exampleConfigTemplate
	} else {                                             // :470  -- populated-config branch
		providerName, _ := cmd.Flags().GetString("provider")   // :471
		if providerName != "" {                                 // :472  -- validation block
			reg := provider.NewRegistry(nil)                    // :473
			if _, ok := reg.Get(providerName); !ok {            // :474
				return exitcode.New(exitcode.Error, fmt.Errorf("unknown provider %q ...", ...))  // :475-477
			}
		}
		content = config.GenerateBootstrapConfig(providerName) // :479  ← BUG: "" on --force w/o --provider → pi
	}
	force, _ := cmd.Flags().GetBool("force")             // :482  ← READ AFTER GenerateBootstrapConfig (too late)
	if force {                                           // :490  -- FR-B8 merge
		content = mergeExistingActiveSettings(path, content)   // :491
	}
	if err := writeBootstrapFile(cmd, path, content, force); err != nil { return err }  // :493
	... print path; return nil
}
```

## 3. THE FIX (3 edits, all in runConfigInit)

**(a) Move the `force` read UP** from :482 (after GenerateBootstrapConfig) to just before the `if tmpl` block
(after the `tmpl` read at :466). `force` stays in scope for its later uses (:490 merge, :493 writeBootstrapFile).

**(b) Add the re-targeting guard** in the `else` branch, AFTER `GetString("provider")` (:471) and BEFORE the
validation block (:472):
```go
providerName, _ := cmd.Flags().GetString("provider")
// BUG-001 (FR-B2/FR-B8): --force re-targets the regenerated template to the PRESERVED [defaults] provider
// instead of auto-detecting pi, so the [role.*] blocks are structurally consistent with the preserved
// default (otherwise an inconsistent [role.stager]=pi + preserved default=claude hard-fails decompose
// under FR-R5b). --provider still wins (the && providerName == "" guard). Unknown/custom preserved
// providers fall back to "" (auto-detect) — never passed to GenerateBootstrapConfig.
if force && providerName == "" {
    providerName = preservedDefaultProvider(path)
}
```

**(c) Update the runConfigInit doc comment** (config.go:456-458) with the BUG-001 re-targeting sentence (DOCS, Mode A).

## 4. Why each path stays correct

| Scenario | providerName after the guard | GenerateBootstrapConfig arg | Result |
|---|---|---|---|
| `--force` (no `--provider`), preserved=claude | "claude" (from helper) | "claude" | template re-targeted → consistent; merge clean ✓ |
| `--force --provider pi` | "pi" (--provider wins; `&& providerName==""` false) | "pi" | unchanged from today ✓ |
| no `--force` (no `--provider`) | "" (force false → guard skipped) | "" | auto-detect pi (first-run) — UNCHANGED ✓ |
| `--force` (no `--provider`), custom provider | "" (helper returns "" for unknown) | "" | auto-detect — custom providers NOT broken ✓ |
| `--template` / `--interactive` | n/a (branches before/around) | n/a | UNAFFECTED ✓ |

The existing validation block (`if providerName != "" { reg.Get(providerName) }`) STAYS and is defense-in-depth:
preservedDefaultProvider already validated against the same registry, so the block always passes for a preserved
name — no change needed (item point 1/3 confirms this).

## 5. The existing test is UNAFFECTED (no conflict)

`TestConfigInit_Force_PreservesActiveSettings` (config_test.go:581) runs `config init --force --provider pi`
(EXPLICIT --provider). S2's guard is `if force && providerName == ""` — providerName is "pi" (non-empty), so
preservedDefaultProvider is NOT called. The existing test passes UNCHANGED. (S1's TestPreservedDefaultProvider
tests the helper in isolation; S2's NEW test tests the wiring — different concerns, no overlap.)

## 6. S2's regression test (mirror TestConfigInit_Force_PreservesActiveSettings, DROP --provider)

The harness: `saveRootState`/`restoreRootState`/`resetFlags`, `setupNoRepo(t)` → globalDir,
`writeConfigFile(t, globalDir, "config.toml", body)`, `rootCmd.SetArgs([]string{"config","init","--force"})`,
`Execute(context.Background())`, then read `config.GlobalConfigPath()`.

NEW test `TestConfigInit_Force_ReTargetsToPreservedProvider`:
- Pre-create `[defaults] provider = "claude"`, `model = "sonnet"` (single-backend, stager-capable).
- Run `config init --force` (NO --provider).
- ASSERT the written config does NOT contain an active `[role.stager] provider = "pi"` (the BUG-001 symptom).
  Strongest content assertion: `!strings.Contains(content, "[role.stager]") || !strings.Contains(...)` —
  actually the cleanest: the config must NOT route stager to pi. Since claude is stager-capable, the
  re-targeted template's `[role.stager]` either has `provider = "claude"` or omits the line (inherits).
- ALSO assert `[defaults] provider = "claude"` is still preserved (FR-B8 — the merge still works).
- This test FAILS on the pre-fix code (which writes `[role.stager] provider = "pi"`) and PASSES post-fix.

## 7. Scope boundaries (do NOT do)
- Do NOT add/modify `preservedDefaultProvider` (S1 owns it — consume only).
- Do NOT touch `GenerateBootstrapConfig` / `buildBootstrapConfig` / `StagerFallback` / `MergeActiveSettings`
  / `ActiveSettings` (the fix is entirely at the runConfigInit call site — the upstream functions are correct
  for a correctly-targeted template).
- Do NOT modify `writeBootstrapFile` or the backup logic (FR-B8 — unrelated).
- Do NOT change docs/cli.md or docs/configuration.md (P1.M1.T3.S1 — S2's "DOCS" is the Go comment only).
- Do NOT touch BUG-002 (configVersionNotice — P1.M1.T2.S1).

## 8. Validation

Internal cmd-package edit + test. Gates: `go build ./...`, `go vet ./...`, `gofmt -l internal/cmd/`,
`go test ./internal/cmd/... -run 'ConfigInit_Force' -v` (the new test + the unaffected existing one),
`make test`, `make lint`. The new integration test is the BUG-001 regression guard. No external libs.