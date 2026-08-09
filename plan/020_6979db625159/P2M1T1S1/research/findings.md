# Research Findings — P2.M1.T1.S1: Provider manifest + config code comments/strings (glm→claude-haiku cleanup)

## §0 Scope & disjointness

**S1 owns 6 files** (Category A — the manifest + config CODE comments/strings; non-behavioral):
`providers/pi.toml`, `internal/cmd/config_init_interactive.go`, `internal/cmd/default_action.go`,
`internal/cmd/config.go`, `internal/config/bootstrap.go`, `internal/provider/builtin.go`.

**File-disjoint from siblings + the parallel task:**
- **S2** (P2.M1.T1.S2) = runtime error messages (`manifest.go`, `render.go`, `roles.go`) + user-facing docs
  (`README.md`, `docs/cli.md`, `docs/configuration.md`, `docs/providers.md`). NO overlap with S1's 6 files.
- **T2.S1/S2** = test fixtures (Category B/C/D). NO overlap — S1 touches NO `_test.go` files.
- **Parallel task P1.M3.T1.S2** (winget→chocolatey) = `docs/cli.md` + `README.md` only. NO overlap with S1.

The contract's S1 acceptance: `rg -n 'zai/glm|glm-5\.2|glm-5-turbo' <the 6 S1 files>` → **zero hits**.
(Full-repo zero-hit requires S2 + T2 too; S1 is scoped to its 6 files per the contract.)

## §1 Replacement strategy (per model_cleanup_scope.md + spec §12.3)

- `zai/glm-5.2` → `anthropic/claude-haiku` (spec §12.3 / §16.3 / FR-R5b chosen illustrative token)
- `zai/glm-5-turbo` → `anthropic/claude-haiku`
- bare `glm-5.2` → `claude-haiku` (in the FR-R5b *bare-model-error* example only — default_action.go L234)
- `glm-5-turbo` (in comments) → genericized ("a personal-override model") per contract (f)

**NOT touched (intentionally):**
- `bootstrap.go:261` = `zai/gpt-5.4` (NOT glm; doesn't match the grep; contract lists only 201/205/229).
  FLAGGED residual — it's an incoherent example (z.ai backend + gpt model) but out of S1's glm scope.
- `config.go:698` = `# default_provider   = "zai"` in the generic myagent template (bare zai; doesn't match
  the grep; contract lists only 94/687). FLAGGED residual (and `default_provider` is a removed v2 field —
  a separate stale-field issue, not glm).

## §2 The exact edits (current text → new text), all verified by direct read

### providers/pi.toml
| Line | Current | New |
|------|---------|-----|
| 18 | `#       default_model = "glm-5.2"          # e.g. override only the model` | `#       default_model = "anthropic/claude-haiku"   # e.g. override only the model` |
| 24 | `#   (Personal-override example, NOT the shipped default: <backend>=zai, <m>=glm-5-turbo = commit-pi.)` | `#   (Personal-override example, NOT the shipped default: <backend>=anthropic, <m>=claude-haiku.)` (drop "= commit-pi" — no longer accurate once the model changes) |
| 53 | `                                    # The zai/glm-5-turbo setup is a personal override (NOT the shipped default).` | `                                    # An override like "anthropic/claude-haiku" is a personal choice (NOT the shipped default).` |
| 59 | `provider_flag = "--provider"        # pi routes to backends: zai \| anthropic \| google \| ...` | `provider_flag = "--provider"        # pi routes to backends: anthropic \| google \| zai \| ...` (reorder so zai is NOT first — contract: "does not imply zai is a default") |
| 61 | `# (e.g. "zai/glm-5.2" → --provider zai --model glm-5.2). FR-R5b enforces this at Render.` | `# (e.g. "anthropic/claude-haiku" → --provider anthropic --model claude-haiku). FR-R5b enforces this at Render.` |

### internal/cmd/config_init_interactive.go
| Line | Current | New |
|------|---------|-----|
| 184 | `			prompt = fmt.Sprintf("%s model [%s]; include the inference/ prefix, e.g. zai/glm-5.2: ",` | `			prompt = fmt.Sprintf("%s model [%s]; include the inference/ prefix, e.g. anthropic/claude-haiku: ",` |
| 200 | `				fmt.Fprintf(w, "multi-backend provider: include the inference backend as a prefix, e.g. zai/glm-5.2\n")` | `				fmt.Fprintf(w, "multi-backend provider: include the inference backend as a prefix, e.g. anthropic/claude-haiku\n")` |

### internal/cmd/default_action.go
| Line | Current | New |
|------|---------|-----|
| 234 | `	// misconfiguration (e.g. bare "glm-5.2" on pi — FR-R5b) is rejected up front instead of` | `	// misconfiguration (e.g. bare "claude-haiku" on pi — FR-R5b) is rejected up front instead of` (BARE model = the FR-R5b error case — keep it bare, do NOT add a slash) |

### internal/cmd/config.go
| Line | Current | New |
|------|---------|-----|
| 94 | `…` + "`model = \"glm-5.2\"`" + ` + ` + "`default_provider = \"zai\"`" + ` becomes ` + "`model = \"zai/glm-5.2\"`" + `),` | same with `glm-5.2`→`claude-haiku`, `zai`→`anthropic`, `zai/glm-5.2`→`anthropic/claude-haiku` (COUPLED migration illustration — all 3 tokens in lockstep so the fold is coherent) |
| 687 | `# default_model    = "glm-5.2"` | `# default_model    = "claude-haiku"` |
| 688 | `# default_provider = "zai"` | `# default_provider = "anthropic"` (COUPLED with 687 — the override-pi example pair) |
| 698 | `# default_provider   = "zai"` | UNCHANGED (out of scope — generic myagent template; bare zai; doesn't match grep; see §1 residual) |

### internal/config/bootstrap.go
| Line | Current | New |
|------|---------|-----|
| 201 | `b.WriteString("# e.g. model = \"zai/glm-5.2\". A bare model (no '/') on pi is a config error (FR-R5b).\n")` | `b.WriteString("# e.g. model = \"anthropic/claude-haiku\". A bare model (no '/') on pi is a config error (FR-R5b).\n")` |
| 205 | `b.WriteString("# slash-prefix (e.g. model = \"zai/glm-5.2\"). A bare model (no '/') on pi is a config\n")` | `b.WriteString("# slash-prefix (e.g. model = \"anthropic/claude-haiku\"). A bare model (no '/') on pi is a config\n")` |
| 229 | `stagerAnnotation += " pi is a multi-backend provider — prefix the model with your inference backend, e.g. model = \"zai/glm-5.2\". A bare model (no '/') on pi is a config error (FR-R5b)."` | same with `zai/glm-5.2`→`anthropic/claude-haiku` |
| 261 | `b.WriteString("# e.g. model = \"zai/gpt-5.4\". …")` | UNCHANGED (out of scope — zai/**gpt**-5.4, not glm; see §1 residual) |

### internal/provider/builtin.go
| Line | Current | New |
|------|---------|-----|
| 34 | `// (provider=zai, model=glm-5-turbo) is a documented PERSONAL OVERRIDE, not the shipped default —` | `// (e.g. provider=anthropic, model=claude-haiku) is a documented PERSONAL OVERRIDE, not the shipped default —` |
| 59 | `		DefaultModel:      strPtr(""), // FR-D2: was glm-5-turbo; decoupled from any one subscription` | `		DefaultModel:      strPtr(""), // FR-D2: was a personal-override model; decoupled from any one subscription` (VALUE `strPtr("")` UNCHANGED — FR-D2 blank; only the comment changes) |

## §3 Test-safety verification (CRITICAL — the contract's "tests stay green" claim)

**Confirmed: S1's edits break NO tests.** The risk was that tests assert on the prompt/template STRINGS S1
changes. Direct verification of the coupling:

- **config_init_interactive_test.go:116** asserts `strings.Contains(out, "include the inference backend as
  a prefix")` — the STATIC prefix of the L200 prompt. S1 changes only the trailing `e.g. zai/glm-5.2` →
  `e.g. anthropic/claude-haiku`; the asserted substring is PRESERVED. ✓ PASS after edit.
- **config_init_interactive_test.go:105-120** uses `zai/gpt-5.4` (NOT glm) as wizard input — unaffected.
- **config_init_interactive_test.go:589,593** use `zai/glm-5.2` as the planner OVERRIDE VALUE (test INPUT →
  written-config assertion). This is Category D (T2.S2's scope). It does NOT assert on the PROMPT string
  (L184/200) S1 changes. So S1's prompt edit is decoupled from this test. ✓ (The test's own glm fixture is
  T2.S2's job — S1 does NOT touch `_test.go` files.)
- **bootstrap_test.go:131,194** assert `strings.Contains(content/piBlock, "multi-backend provider")` — the
  STATIC NOTE prefix. S1 changes only `zai/glm-5.2`→`anthropic/claude-haiku` inside those WriteStrings; the
  asserted "multi-backend provider" substring is PRESERVED. ✓ PASS after edit.
- **config.go L94 (docstring) / L687 (commented template)** — NOT asserted by config_test.go (whose glm hits
  at L1350-1505 are inline v2→v3 migration FIXTURES, Category C / T2.S2 — they use their own input/expected
  strings, not the docstring/template text). ✓
- **default_action.go L234 / pi.toml / builtin.go L34** — pure comments; no test asserts on comment text.
- **builtin.go L59** — `DefaultModel: strPtr("")` VALUE unchanged; builtin_test.go asserts on the VALUE
  (blank), not the trailing comment. ✓

∴ `go test ./internal/cmd/... ./internal/config/... ./internal/provider/...` stays GREEN after S1's edits.

## §4 Validation gates (verified executable)

- **S1-scoped zero-hit grep** (the contract's OUTPUT criterion 4):
  `rg -n 'zai/glm|glm-5\.2|glm-5-turbo' providers/pi.toml internal/cmd/config_init_interactive.go
  internal/cmd/default_action.go internal/cmd/config.go internal/config/bootstrap.go internal/provider/builtin.go`
  → **zero hits**.
- **No test regressions**: `go test ./internal/cmd/... ./internal/config/... ./internal/provider/...`
- **Build/format**: `go build ./...` ; `gofmt -l <the edited files>` → empty.
- **Vet**: `go vet ./internal/cmd/... ./internal/config/... ./internal/provider/...`
- **Scope guard**: `git status --porcelain` == the 6 S1 files ONLY (no `_test.go`, no docs, no spec/, no PRD/plan/tasks).

## §5 Residuals flagged (NOT fixed by S1 — out of scope)

1. `bootstrap.go:261` — `"zai/gpt-5.4"` (z.ai backend + gpt model = incoherent; not glm, doesn't match the
   grep; contract lists only 201/205/229). A separate consistency fix.
2. `config.go:698` — `# default_provider   = "zai"` in the generic myagent template (bare zai; doesn't match
   the grep; `default_provider` is a removed v2 field — a stale-field issue, not glm).
3. The spec `spec/02-providers.md §12.3` personal-override paragraph STILL says `zai/glm-5-turbo` (the spec
   intro says `anthropic/claude-haiku` — the spec is internally inconsistent). Per AGENTS.md hard rule 1,
   spec/ is NEVER edited outside an interactive session. S1 grounds its edits in the spec's INTENDED token
   (`anthropic/claude-haiku`); the spec §12.3 personal-override gap is flagged, not fixed.
4. Test fixtures (Category B/C/D) + runtime error messages (manifest.go/render.go/roles.go) + the 4 user-
   facing docs — all are S2/T2's scope. S1's grep is scoped to its 6 files; the full-repo zero-hit lands
   when S2+T2 land.