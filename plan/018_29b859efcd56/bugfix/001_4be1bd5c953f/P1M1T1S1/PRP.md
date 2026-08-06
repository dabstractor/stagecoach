name: "P1.M1.T1.S1 — preservedDefaultProvider helper (extract validated provider from existing config)"
description: >
  Add a pure helper `preservedDefaultProvider(path string) string` in internal/cmd/config.go that reads
  an existing config file, extracts the active `[defaults] provider` value (via config.ActiveSettings,
  which stores values verbatim WITH quotes), strips the surrounding TOML quotes, and validates the name
  against the built-in provider registry — returning the validated built-in name, or "" to signal
  fall-back to auto-detection. This is the BUG-001 fix's foundation: S2 wires it into runConfigInit so
  `config init --force` re-targets the template to the PRESERVED provider instead of auto-detecting pi
  (which injects an inconsistent [role.stager] that breaks decomposition). S1 = helper + unit test + doc
  comment ONLY; no runConfigInit change.

---

## Goal

**Feature Goal**: Provide the pure extraction+validation helper that lets `config init --force`
(S2) re-target the regenerated template to the user's preserved `[defaults] provider` instead of
always auto-detecting pi — the foundation of the BUG-001 fix (FR-B2/FR-B8).

**Deliverable**: (1) `preservedDefaultProvider(path string) string` in `internal/cmd/config.go` with a
BUG-001/FR-B2/FR-B8 doc comment; (2) a table-driven unit test in `internal/cmd/config_test.go` covering
all edge cases (absent/inert/unknown → ""; known built-in → the name).

**Success Definition**:
- `preservedDefaultProvider` returns the validated built-in provider name for an existing config with an
  active `[defaults] provider = "<built-in>"`, and `""` for absent/unreadable/inert/custom-provider files.
- It uses `config.ActiveSettings` (verbatim values) + `strings.Trim(v, "\"")` + `provider.NewRegistry(nil).Get`.
- NO new imports (os/strings/config/provider already imported in config.go).
- runConfigInit is UNCHANGED (S2 wires it in).
- `go build ./...`, `go test ./internal/cmd/...`, `make test`, `make lint` pass.

## Why

- **BUG-001 (Major)**: `config init --force` calls `GenerateBootstrapConfig("")` (auto-detect → pi) and
  then merges the user's preserved `[defaults] provider = "claude"`, but the freshly-generated
  `[role.stager] provider = "pi"` survives the merge (the user's file has no `[role.stager]` section to
  reconcile it). The result is an inconsistent config (default=claude, stager=pi) that hard-fails
  decomposition under FR-R5b (`model "sonnet" on pi must be inference/model`). The fix (landed in S2) is
  to re-target the template to the PRESERVED provider before generating it; this subtask provides the
  helper that extracts+validates that provider.
- **Why validate against the built-in registry**: a custom/unknown provider must NOT be passed to
  `GenerateBootstrapConfig` (which builds role models from the built-in FR-D4 table and would mis-handle
  an unknown name). Unknown → "" → auto-detect, preserving today's behavior for custom providers.

## What

**User-visible behavior**: None (S1 is an internal helper not yet wired in).

**Technical change (one helper + one test + a doc comment):**
- `preservedDefaultProvider(path string) string` in config.go: read file → `config.ActiveSettings` →
  `["defaults"]["provider"]` → `strings.Trim(v, "\"")` → `provider.NewRegistry(nil).Get` → return name or "".

### Success Criteria
- [ ] `preservedDefaultProvider` returns the validated built-in name for a known built-in
- [ ] Returns "" for absent/unreadable/inert/custom-provider files
- [ ] Uses only already-imported deps (os/strings/config/provider) — no new import
- [ ] runConfigInit UNCHANGED
- [ ] Table-driven unit test covers all edge cases
- [ ] `go build ./...`, `go test ./internal/cmd/...`, `make test`, `make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact signature, the verbatim-value quirk (and the Trim fix), the registry API, the mirror helper (mergeExistingActiveSettings), the no-new-import confirmation, and the edge-case matrix are all enumerated below (verified by reading source).

### Documentation & References

```yaml
- file: internal/config/inert.go
  why: "config.ActiveSettings(content) map[string]map[string]string (line 38). Stores values VERBATIM (WITH surrounding quotes) — the doc comment (36-37) says 'NOT unquoted — FR-B8/B9 carry values VERBATIM'. So provider = \"claude\" → out[\"defaults\"][\"provider\"] == \"\\\"claude\\\"\" (literal quotes)."
  pattern: "ActiveSettings is PURE (no I/O). Body: activeKVRe match → val := strings.TrimSpace(m[2]) (TrimSpace only, NO unquoting)."
  gotcha: "MUST strings.Trim(v, \"\\\"\") to recover the bare name. Do NOT assume ActiveSettings unquotes — it deliberately does not."

- file: internal/provider/registry.go
  why: "provider.NewRegistry(nil) (line 35) → *Registry (built-ins only; nil = no user overrides). reg.Get(name) (line 57) → (Manifest, bool); ok==true ⇒ known built-in."
  pattern: "Same call shape runConfigInit already uses to validate --provider (config.go ~474)."

- file: internal/cmd/config.go
  why: "THE home file. mergeExistingActiveSettings (446) is the EXACT pattern to mirror (os.ReadFile → on-error fallback → config.<helper>(string(data))). runConfigInit (462) is the S2 call site (DO NOT touch in S1). Imports (15-28): os, strings, config, provider ALL present."
  pattern: "func mergeExistingActiveSettings(path, freshContent string) string { data, err := os.ReadFile(path); if err != nil { return freshContent }; return config.MergeActiveSettings(freshContent, string(data)) }"
  gotcha: "NO new import — os/strings/config/provider are already imported. Place preservedDefaultProvider near mergeExistingActiveSettings (co-locate the two file-reading helpers)."

- file: internal/cmd/config_test.go
  why: "Test home (package cmd — internal test ⇒ can call preservedDefaultProvider directly). TestConfigInit_Force_PreservesActiveSettings (581) is the closest precedent (writes a temp config, exercises the force path). The helper test is simpler: write temp file, call helper, assert string."
  pattern: "dir := t.TempDir(); path := filepath.Join(dir, \"c.toml\"); os.WriteFile(path, []byte(body), 0o644); got := preservedDefaultProvider(path); want := ...; if got != want { t.Errorf(...) }"

- docfile: plan/018_29b859efcd56/bugfix/001_4be1bd5c953f/architecture/bug001_analysis.md
  why: "The full BUG-001 root-cause trace + the Option-A fix design (re-target template to preserved provider) + the helper's 5-step spec + the edge-case table."
- docfile: plan/018_29b859efcd56/bugfix/001_4be1bd5c953f/P1M1T1S1/research/verification_deltas.md
  why: "The verified anchors, the no-new-import confirmation, the verbatim-value quirk, the edge-case matrix, and the scope boundaries. READ THIS before editing."
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/
  config.go           # THE home: add preservedDefaultProvider near mergeExistingActiveSettings(446). runConfigInit(462) = S2 call site (UNTOUCHED in S1)
  config_test.go      # +TestPreservedDefaultProvider (table-driven; package cmd)
internal/config/
  inert.go            # ActiveSettings(38) — verbatim values; the helper's core dependency
internal/provider/
  registry.go         # NewRegistry(nil)(35) + Get(57) — the validation dependency
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (verbatim values): config.ActiveSettings stores values WITH surrounding TOML quotes
//   (inert.go:36-37: "NOT unquoted — FR-B8/B9 carry values VERBATIM"). out["defaults"]["provider"]
//   for `provider = "claude"` is the literal "\"claude\"" (quotes included). MUST strings.Trim(v, "\"")
//   before validating. Do NOT assume unquoting.

// CRITICAL (no new import): os, strings, config, provider are ALL already imported in config.go.
//   Adding any of them again is a duplicate-import compile error. The helper needs nothing else.

// GOTCHA (validate, don't trust): strip quotes THEN validate against provider.NewRegistry(nil).Get.
//   An unknown/custom provider name must return "" (fall back to auto-detect) — never pass a custom
//   name to GenerateBootstrapConfig (S2), which builds role models from the built-in FR-D4 table.

// GOTCHA (section matters): look up ONLY ["defaults"]["provider"]. A provider key under [generation]
//   or a [role.*] table is not the default and must not be extracted. If the "defaults" section or the
//   "provider" key is absent (e.g. inert file), return "".

// GOTCHA (read-error = ""): mirror mergeExistingActiveSettings — on os.ReadFile error (absent/unreadable),
//   return "" (auto-detect), NOT a hard error. The helper is best-effort extraction.

// SCOPE: S1 is the helper + its test + its doc comment. Do NOT edit runConfigInit (S2), ActiveSettings,
//   MergeActiveSettings, bootstrap.go, or StagerFallback.
```

## Implementation Blueprint

### Data models and structure

No struct/type changes. One new unexported function returning a string. Pure file-I/O + registry lookup.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD preservedDefaultProvider in internal/cmd/config.go
  - PLACE: near mergeExistingActiveSettings (line 446) — co-locate the two file-reading helpers.
  - BODY:
        // preservedDefaultProvider reads the existing config at path and returns the active
        // [defaults] provider if it is a known BUILT-IN, else "". BUG-001 (FR-B2/FR-B8): config init
        // --force must re-target the regenerated template to the PRESERVED provider instead of
        // auto-detecting pi, which would inject an inconsistent [role.stager] that breaks decompose
        // under FR-R5b. Unknown/custom providers return "" (fall back to auto-detection — do not break
        // custom providers or pass an unknown name to GenerateBootstrapConfig). ActiveSettings stores
        // values VERBATIM (with quotes), so the value is unquoted before validation. Best-effort: an
        // absent/unreadable/inert file returns "".
        func preservedDefaultProvider(path string) string {
            data, err := os.ReadFile(path)
            if err != nil {
                return "" // absent/unreadable → auto-detect
            }
            sections := config.ActiveSettings(string(data))
            v, ok := sections["defaults"]["provider"]
            if !ok || v == "" {
                return "" // no active default provider (absent or inert) → auto-detect
            }
            name := strings.Trim(v, "\"") // ActiveSettings stores values verbatim (with quotes)
            reg := provider.NewRegistry(nil)
            if _, known := reg.Get(name); !known {
                return "" // custom/unknown → auto-detect (don't break custom providers)
            }
            return name
        }
  - NO new import (os/strings/config/provider already imported — confirmed).
  - DEPENDENCIES: none.

Task 2: ADD TestPreservedDefaultProvider in internal/cmd/config_test.go
  - PLACE: near TestConfigInit_Force_PreservesActiveSettings (581) — same concern area.
  - TABLE-DRIVEN over the edge cases. For each row: write a temp file (or none for the absent case),
    call preservedDefaultProvider(path), assert the returned string.
        func TestPreservedDefaultProvider(t *testing.T) {
            cases := []struct {
                name string
                body string  // "" + write=false ⇒ absent file
                want string
            }{
                {"absent file", "", ""},                                           // read error → ""
                {"inert all-commented", "# provider = \"claude\"\n", ""},          // no active setting → ""
                {"known built-in claude", "[defaults]\nprovider = \"claude\"\n", "claude"},
                {"known built-in codex", "[defaults]\nprovider = \"codex\"\n", "codex"},
                {"known built-in pi", "[defaults]\nprovider = \"pi\"\n", "pi"},
                {"custom unknown provider", "[defaults]\nprovider = \"myagent\"\n", ""}, // don't break custom
                {"provider under wrong section", "[generation]\nprovider = \"claude\"\n", ""}, // only [defaults] counts
                {"no provider key", "[defaults]\nmodel = \"sonnet\"\n", ""},
            }
            for _, tc := range cases {
                t.Run(tc.name, func(t *testing.T) {
                    dir := t.TempDir()
                    path := filepath.Join(dir, "c.toml")
                    if tc.name != "absent file" {
                        if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil { t.Fatal(err) }
                    }
                    if got := preservedDefaultProvider(path); got != tc.want {
                        t.Errorf("preservedDefaultProvider(%q) = %q; want %q", tc.body, got, tc.want)
                    }
                })
            }
        }
  - IMPORTS: os, path/filepath (both already used in config_test.go). Confirm with gofmt/goimports.
  - DEPENDENCIES: Task 1.

Task 3: VERIFY runConfigInit UNCHANGED + no behavior regression
  - CONFIRM runConfigInit (config.go:462) was NOT edited (S1 is helper-only).
  - CONFIRM existing TestConfigInit_* tests still pass (the helper is additive; nothing wired yet).
  - DEPENDENCIES: Tasks 1-2.
```

### Implementation Patterns & Key Details

```go
// PATTERN: mirror mergeExistingActiveSettings (config.go:446) — read file, on-error fallback, hand off to a config helper
data, err := os.ReadFile(path)
if err != nil {
    return "" // absent/unreadable → auto-detect (best-effort)
}
sections := config.ActiveSettings(string(data))

// PATTERN: ActiveSettings values are VERBATIM (with quotes) — unquote before use
v, ok := sections["defaults"]["provider"]
name := strings.Trim(v, "\"") // "claude" (quoted) → claude

// PATTERN: validate against the built-in registry (same shape runConfigInit uses for --provider)
reg := provider.NewRegistry(nil)
if _, known := reg.Get(name); !known {
    return "" // custom/unknown → fall back to auto-detection
}
return name
```

### Integration Points

```yaml
NO struct / API / config-schema / build changes. One unexported helper + its test.

CODE:
  - internal/cmd/config.go — +preservedDefaultProvider (near mergeExistingActiveSettings, line 446)
TESTS:
  - internal/cmd/config_test.go — +TestPreservedDefaultProvider (table-driven)

DOWNSTREAM (consumes this helper — do NOT implement in S1):
  - P1.M1.T1.S2: wires preservedDefaultProvider into runConfigInit
    (`if force && providerName == "" { providerName = preservedDefaultProvider(path) }` before
    GenerateBootstrapConfig; moves the `force` flag read up).

UNCHANGED: runConfigInit; ActiveSettings; MergeActiveSettings; bootstrap.go; StagerFallback; GenerateBootstrapConfig.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...
go vet ./...
gofmt -l internal/cmd/
# Expected: empty.
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new helper test
go test ./internal/cmd/ -run TestPreservedDefaultProvider -v
# Expected: all table rows pass (absent/inert/unknown → ""; known built-ins → the name).

# Full cmd package (existing TestConfigInit_* must still pass — helper is additive, nothing wired)
go test ./internal/cmd/... -v

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# (S1 is a pure helper not yet wired into any command — no end-to-end path exists until S2 lands.
#  The table-driven unit test IS the within-scope proof. The full BUG-001 end-to-end repro
#  [config init --force → decompose] is validated in S2's gate, not S1's.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: the helper exists with the BUG-001/FR-B doc comment
grep -n "func preservedDefaultProvider\|BUG-001\|FR-B8" internal/cmd/config.go
# Expected: the function + the doc-comment citations present.

# Grep guard: NO new import added (os/strings/config/provider were already there)
git diff internal/cmd/config.go | grep -E '^\+' | grep -E '^\+\s*"os"|"strings"|"/config"|"/provider"' | grep -v '^\+\+\+'
# Expected: empty (no newly-added import lines for already-present packages).

# Grep guard: runConfigInit UNCHANGED in S1 (S2 owns the wiring)
git diff internal/cmd/config.go | grep -nE '^\+.*providerName = preservedDefaultProvider|^\-.*GenerateBootstrapConfig'
# Expected: empty (S1 adds the helper + test only; no runConfigInit edit).

# Scope-boundary guard: ActiveSettings/MergeActiveSettings/bootstrap untouched
git diff --name-only
# Expected: only internal/cmd/config.go + internal/cmd/config_test.go.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `gofmt -l internal/cmd/` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) all pass

### Feature Validation
- [ ] `preservedDefaultProvider` returns the built-in name for `[defaults] provider = "<builtin>"`
- [ ] Returns "" for absent/unreadable/inert/no-provider-key/wrong-section/custom-unknown
- [ ] Strips surrounding TOML quotes (ActiveSettings stores verbatim)
- [ ] Validates via `provider.NewRegistry(nil).Get`
- [ ] Table-driven test covers all edge cases

### Scope-Boundary Validation
- [ ] runConfigInit UNCHANGED (S2 wires it)
- [ ] NO new import (os/strings/config/provider already imported)
- [ ] ActiveSettings/MergeActiveSettings/bootstrap.go/StagerFallback untouched
- [ ] Doc edit limited to the helper's Go doc comment (docs/cli.md is P1.M1.T3.S1)

### Code Quality
- [ ] Helper co-located with mergeExistingActiveSettings (file-reading helpers grouped)
- [ ] Doc comment cites BUG-001 rationale + FR-B2/FR-B8
- [ ] Test follows the package's t.TempDir()+os.WriteFile idiom

---

## Anti-Patterns to Avoid

- ❌ Don't forget to `strings.Trim(v, "\"")` — ActiveSettings stores values VERBATIM (with quotes); validating `"\"claude\""` against the registry returns false (not a built-in) and you'd silently always return "".
- ❌ Don't add `os`/`strings`/`config`/`provider` imports — they're already imported; re-adding is a duplicate-import compile error.
- ❌ Don't edit runConfigInit in S1 — the wiring (`if force && providerName == ""`) is S2's job. S1 is helper + test only.
- ❌ Don't pass a custom/unknown provider through — validate against the built-in registry and return "" for unknowns (auto-detect preserves today's behavior for custom providers; GenerateBootstrapConfig would mis-handle an unknown name).
- ❌ Don't look up provider under any section other than `[defaults]` — only the global default is the re-target source.
- ❌ Don't return a hard error on read failure — mirror mergeExistingActiveSettings: best-effort, return "" (auto-detect).
- ❌ Don't touch ActiveSettings to make it unquote — the verbatim storage is a deliberate FR-B8/B9 contract shared by the load path and upgrade; the helper unquotes locally instead.

---

## Confidence Score: 9/10

One-pass success is very high: a small pure helper with a fully-specified body, the verbatim-value quirk
identified (and the Trim fix given), the registry API confirmed, the no-new-import fact established, and
a clear mirror helper (mergeExistingActiveSettings) to copy the read+ActiveSettings shape from. The -1 is
for the test's "wrong section" edge case — ActiveSettings keys top-level keys under "" and `[generation]`
under "generation", so a `provider` key under `[generation]` correctly does NOT populate
`["defaults"]["provider"]` (verified by the ActiveSettings body); the test asserts this, but an
implementer who mis-reads the section grouping could get it wrong. Mitigated by the explicit edge-case
row + the verified ActiveSettings body in the research notes.