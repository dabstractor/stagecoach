# P2.M1.T1.S2 — Verified hit inventory + test-safety analysis

## Replacement token (matches PRD §12.3 + sibling S1)

- Prefixed: `zai/glm-5.2` → `anthropic/claude-haiku` (the canonical multi-backend example).
- Bare: `glm-5.2` → `claude-haiku` (an error-case / incidental value stays bare — it is the WRONG/no-slash form
  in the FR-R5b illustration, or an incidental `--model` value in docs).

## EXACT current hits in S2's 7 files (verified by rg at HEAD; line numbers beat the contract's stale cites)

| File | Line | Hit (oldText anchor) | New |
|------|------|----------------------|-----|
| internal/provider/manifest.go | 139 | `// e.g. "glm-5.2" on pi instead of "zai/glm-5.2")` | `// e.g. "claude-haiku" on pi instead of "anthropic/claude-haiku")` |
| internal/provider/manifest.go | 156 | `e.g. \"zai/glm-5.2\"` (in ValidateModel fmt.Errorf) | `e.g. \"anthropic/claude-haiku\"` |
| internal/provider/render.go | 127 | `e.g. \"zai/glm-5.2\"` (Render fmt.Errorf) | `e.g. \"anthropic/claude-haiku\"` |
| internal/provider/render.go | 249 | `e.g. \"zai/glm-5.2\"` (RenderMultiTurn fmt.Errorf) | `e.g. \"anthropic/claude-haiku\"` |
| internal/decompose/roles.go | 163 | `// ("zai/glm-5.2");` (comment) | `// ("anthropic/claude-haiku");` |
| internal/decompose/roles.go | 169 | `e.g. \"zai/glm-5.2\"` (role fmt.Errorf) | `e.g. \"anthropic/claude-haiku\"` |
| README.md | 310 | `git config stagecoach.model zai/glm-5.2` | `git config stagecoach.model anthropic/claude-haiku` |
| README.md | 315 | `> (`zai/glm-5.2`). A bare model ...` | `> (`anthropic/claude-haiku`). A bare model ...` |
| README.md | 316 | `git config stagecoach.model zai/glm-5.2` (or `[defaults] model = "zai/glm-5.2"` ...) | `git config stagecoach.model anthropic/claude-haiku` (or `[defaults] model = "anthropic/claude-haiku"` ...)  [TWO occ on one line] |
| README.md | 319 | `set `model = "zai/glm-5.2"` to pin ...` | `set `model = "anthropic/claude-haiku"` to pin ...` |
| docs/cli.md | 70 | `stagecoach --model glm-5.2` (bare, shadowing example) | `stagecoach --model claude-haiku` |
| docs/cli.md | 172 | `e.g. model = "zai/glm-5.2") to pin a backend` | `e.g. model = "anthropic/claude-haiku") to pin a backend` |
| docs/configuration.md | 40 | `e.g. `model = "zai/glm-5.2"`, FR-R5b` | `e.g. `model = "anthropic/claude-haiku"`, FR-R5b` |
| docs/configuration.md | 232 | `    model = glm-5.2` (bare, config example) | `    model = claude-haiku` |
| docs/providers.md | 72 | `the model is `inference/model` (e.g. `zai/glm-5.2`)` | `... (e.g. `anthropic/claude-haiku`)` |

NOTE: README.md line numbers DRIFTED ~6 from the contract (contract 304/309/310/313 → actual 310/315/316/319);
the other 6 files match the contract cites. Use ACTUAL current lines above (re-run the acceptance grep at
impl to re-anchor — never trust the contract's bare line numbers blindly).

## TEST-SAFETY analysis (CRITICAL — determines S2's gate)

`go test ./internal/provider/... ./internal/decompose/...` STAYS GREEN after S2's message-text change, because:

1. **No test asserts on the glm portion of the error message.** The only message-text assertions are on the
   STATIC substring `"must be inference/model"` (preserved by S2 — that phrase is unchanged):
   - internal/provider/manifest_test.go:329 `strings.Contains(err.Error(), "must be inference/model")`
   - internal/decompose/roles_test.go:332/333/391/392 `strings.Contains(errMsg, "must be inference/model")` + role name.
2. **glm as test INPUT/fixture data** (render_test.go, manifest_test.go:323/325/338/340/350/383, merge_test.go,
   registry_test.go) does NOT break from S2: those fixtures pass `zai/glm-5.2` as VALID input (which does not
   trigger the error) or a bare `glm-5.2` (which triggers the error, asserted only on the static phrase).
   Refreshing those fixtures to the new token is **Category B/C = P2.M1.T2 scope (independent, separate task)**,
   not a S2 dependency.

=> S2's gate (provider + decompose suites green) is achievable STANDALONE. The contract's hedge ("tests that
assert on exact error MESSAGE text are updated in P2.M1.T2.S1") is conservative; in practice NO test asserts on
the glm example text, so S2 introduces zero red.

## Scope boundaries (from sibling S1 + parallel tasks)

- S1 (P2.M1.T1.S1) owns the OTHER 6 Category-A code files (pi.toml, config_init_interactive.go,
  default_action.go, config.go, bootstrap.go, builtin.go). S2 does NOT touch those.
- T2 (P2.M1.T2.S1/S2) owns ALL test-fixture glm refresh (Category B/C/D). S2 does NOT touch test files.
- **Parallel collision (informational, not blocking):** P1.M3.T1.S2 edits docs/cli.md + README.md for
  winget→chocolatey on DIFFERENT lines. S2 edits the glm lines. Text anchors are disjoint (no merge conflict),
  but re-read those 2 files at impl time (winget edits may shift line numbers).
- **spec/ is human-owned** (AGENTS.md rule 1) — never edit. PRD §12.3's personal-override para still says
  zai/glm-5-turbo (known, flagged, NOT this task's scope).