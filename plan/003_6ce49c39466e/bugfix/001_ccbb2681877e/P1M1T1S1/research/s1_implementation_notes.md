# S1 Implementation Notes — claude ReasoningLevels (verified --effort tokens)

> Scope: P1.M1.T1.S1 — populate `builtinClaude()`'s `ReasoningLevels` with the verified `claude --help`
> `--effort` tokens. claude ONLY (pi is S2 / P1.M1.T1.S2). Verified against live source 2026-07-02.

## 1. Verified tokens (architecture/external_deps.md §claude, lines 27-35)

`claude --help` exposes `--effort <level>` with values `low|medium|high` — NOT `--thinking-effort`
(the PRD Suggested Fix guessed wrong). Map (external_deps.md §claude):
- `"high"`   → `["--effort", "high"]`
- `"medium"` → `["--effort", "medium"]`
- `"low"`    → `["--effort", "low"]`
NO `off` key — `--effort` has no "off" value; `off` stays a graceful no-op (absent key → nil slice).
(pi uses `--thinking` — external_deps.md §pi; that is S2, NOT this subtask.)

## 2. The Render guard is already correct (render.go:124-127) — NO change

```go
// FR-R6: append the resolved reasoning level's tokens if the manifest declares them. Absent level,
// nil map, or empty token list ⇒ SILENT no-op — never an error.
if reasoning != "" && len(r.ReasoningLevels[reasoning]) > 0 {
    args = append(args, r.ReasoningLevels[reasoning]...)
}
```
Tokens append AFTER the model flag (render.go:127). Resolve() (manifest.go:180) leaves ReasoningLevels
as-is (nil stays nil); Validate() imposes no constraint. So ONLY the manifest DATA is missing.

## 3. THREE parity surfaces must stay in sync (CRITICAL — two DeepEqual tests)

Adding `ReasoningLevels` to `builtinClaude()` ripples to TWO fixtures that `reflect.DeepEqual` against
it. ALL THREE must carry the identical `[reasoning_levels]` table or `go test ./internal/provider/` fails:

| surface | file | enforced by |
|---|---|---|
| source of truth | internal/provider/builtin.go `builtinClaude()` (~line 134) | — |
| test fixture | internal/provider/builtin_test.go `claudeTOML` const (line 45) | TestBuiltinManifests_DecodeParity (line 366) `reflect.DeepEqual(tc.got, decoded)` |
| shipped reference file | providers/claude.toml | TestProviderReferenceFiles_DecodeParity (referencefiles_test.go:39) `reflect.DeepEqual(decoded, want)` |

The contract's DOCS note ("docs/providers.md line 35") is INCOMPLETE — `providers/claude.toml` MUST
also get the table (it is parity-tested, not just docs). `claudeTOML` const MUST also get it (else
DecodeParity breaks). These are not optional.

## 4. The exact edit to builtinClaude() (builtin.go)

Insert AFTER the `TooledFlags: []string{...},` block and BEFORE `Output: strPtr("raw"),`:
```go
		// REASONING LEVELS (v3; §12.1, FR-R6). claude exposes `--effort low|medium|high` (verified vs
		// `claude --help`, external_deps.md §claude — NOT --thinking-effort). off has no entry ⇒ no-op.
		ReasoningLevels: map[string][]string{
			"high":   {"--effort", "high"},
			"medium": {"--effort", "medium"},
			"low":    {"--effort", "low"},
		},
```
Update the doc comment (line 101): "(2) ReasoningLevels is nil — §12.4 OMITS the key entirely." →
"(2) ReasoningLevels is populated (claude `--effort`, verified — external_deps.md §claude).".
Update the trailing comment (line 135): remove `ReasoningLevels` from
"Subcommand, PromptFlag, JsonField, RetryInstruction, Env, ReasoningLevels: nil (absent in §12.4).".

## 5. The matching [reasoning_levels] table (claudeTOML const + providers/claude.toml)

Append at the END of both (after `strip_code_fence = true`; top-level keys MUST precede any [table]):
```toml

[reasoning_levels]
high = ["--effort", "high"]
medium = ["--effort", "medium"]
low = ["--effort", "low"]
```
go-toml decodes this into `map[string][]string` matching the builtin literal; `reflect.DeepEqual`
passes (map order-independent; slice element-wise). In providers/claude.toml add a comment block
explaining the tokens (mirrors the file's comment style) — but the FIELD lines must match byte-for-byte
modulo comments (the parity test decodes, so comments are stripped; only the data must match).

## 6. Tests (builtin_test.go + render_test.go)

- EXTEND `TestBuiltinManifests_ClaudeFields` (builtin_test.go:295) — add:
  ```go
  if m.ReasoningLevels == nil || len(m.ReasoningLevels["high"]) == 0 {
      t.Errorf("ReasoningLevels missing 'high': %v", m.ReasoningLevels)
  }
  ```
- ADD a focused render test using the REAL built-in (per contract — not a synthetic manifest like
  TestRender_ReasoningTokensAppended at render_test.go:387). Place in render_test.go (package provider,
  has containsPair/containsToken helpers at lines 437/447):
  ```go
  func TestRender_ClaudeReasoningEffortTokens(t *testing.T) {
      m := builtinClaude()                       // the REAL built-in
      s, err := m.Render("sonnet", "", "", "high")
      if err != nil { t.Fatalf("high: %v", err) }
      if !containsPair(s.Args, "--effort", "high") {
          t.Errorf("claude high: want --effort high in %v", s.Args)
      }
      for _, lvl := range []string{"medium", "low"} {
          ss, _ := m.Render("sonnet", "", "", lvl)
          if !containsPair(ss.Args, "--effort", lvl) {
              t.Errorf("claude %s: want --effort %s in %v", lvl, lvl, ss.Args)
          }
      }
      // off / "" → no-op (no --effort token), never an error
      so, err := m.Render("sonnet", "", "", "off")
      if err != nil { t.Fatalf("off: %v", err) }
      if containsToken(so.Args, "--effort") {
          t.Errorf("claude off: want no --effort token in %v", so.Args)
      }
      se, _ := m.Render("sonnet", "", "", "")
      if containsToken(se.Args, "--effort") {
          t.Errorf("claude empty reasoning: want no --effort token in %v", se.Args)
      }
  }
  ```
  Trace of `claude.Render("sonnet","","","high")` (ProviderFlag="" → no split): Args =
  `["--model","sonnet","--effort","high","--tools","","--setting-sources","","--no-session-persistence","-p"]`.

## 7. docs/providers.md line 35 (Mode A)

The `reasoning_levels` row currently: "| `reasoning_levels` | table | nil (none) | Per-level reasoning-
effort token lists (off/low/medium/high); nil/empty ⇒ graceful no-op (FR-R6). Appended after the model
flag at render. |". Update the DESCRIPTION cell to note claude populates high/medium/low via `--effort`
(e.g. append: "claude populates high/medium/low via `--effort` (verified `claude --help`); other built-
ins nil."). The DEFAULT column stays "nil (none)" (it's the schema default, not claude-specific).

## 8. Scope discipline — claude ONLY

- NOT pi (pi `--thinking` tokens = S2 / P1.M1.T1.S2).
- NOT render.go / manifest.go / merge.go (the guard + schema already exist and are correct).
- NOT the other 6 providers (gemini/agy/qwen-code/opencode/codex/cursor leave nil — external_deps.md §38-42).
- NOT the message-role routing (Issue 2 = P1.M2) or the index-sync (Issue 3 = P1.M3).
