# P2.M1.T1.S2 Research Findings — qwen-code tier row + FR-D5 token refresh + docs

Verified by reading the live codebase (role_defaults.go, bootstrap.go, builtin.go, builtin_test.go,
docs/providers.md, providers/*.toml) + the S1 PRP contract + the PRD FR-D4 table. Load-bearing for the PRP.

## §1. CRITICAL CONFLICT — qwen-code `stager` cell must be `""`, NOT `qwen3-coder-flash`

The S2 contract literally says "stager qwen3-coder-flash". The PRD FR-D4 table also lists it. BUT both
conflict with the codebase's stager-capability model:

- **S1's manifest (the CONTRACT)**: `builtinQwenCode()` sets `TooledFlags` to **nil** (S1 PRP §2: "qwen-code
  CANNOT serve as a stager until the scoped tool combo is verified"). So qwen-code is NOT stager-capable.
- **`stagerFallback` (bootstrap.go:75-86)**: `if m := models["stager"]; m != "" { return target, m }` — a
  NON-EMPTY stager cell is the signal "this provider IS the stager". If qwen-code stager were
  "qwen3-coder-flash", the bootstrap would route the stager role to qwen-code, then `RenderTooled` would
  **ERROR** (TooledFlags nil) at invocation ⇒ the stager crashes. **This is a hard correctness break.**
- **Code convention + tests**: every non-stager-capable provider (gemini, agy, opencode, codex, cursor) has
  `stager: ""` in role_defaults.go; `TestDefaultModelsForProvider_StagerCapability` PINS this; the
  role_defaults.go doc comment states: *"Stager cells: non-empty IFF the provider's manifest has non-empty
  TooledFlags (pi, claude); "" otherwise."*

**RESOLUTION**: qwen-code `stager = ""` (matching the 5 other non-stager-capable providers). The PRD FR-D4
table's `qwen3-coder-flash` is the "ideal model IF it could stage" — but the PRD's own note ("A platform
whose tooled_flags is empty ... cannot serve as the stager; for those, the stager role falls back") means
the runtime cell is `""` and the bootstrap applies the FR-D4 fallback (→ pi/claude). Following the literal
contract text here would break the stager. The qwen-code row is therefore:
`planner: qwen3-coder-plus, stager: "", message: qwen3-coder-flash, arbiter: qwen3-coder-plus` (all `# TO CONFIRM`).

## §2. gemini/agy planner refresh: `gemini-3.5-pro` → `gemini-3.1-pro`

The PRD FR-D4 table (selected_prd_content, authoritative) + the S2 contract both say agy/gemini flagship =
`gemini-3.1-pro`. The CURRENT role_defaults.go has `gemini-3.5-pro` (P1.M3.T3.S1 wrote 3.5-pro claiming
"PRD baseline 2026-07" — but the actual PRD FR-D4 table says 3.1-pro). This is a real refresh:

- role_defaults.go: gemini/agy `planner` `gemini-3.5-pro` → `gemini-3.1-pro`.
- role_defaults_test.go `TestDefaultModelsForProvider_PerProvider`: hardcodes `gemini-3.5-pro` for
  gemini+agy → update to `gemini-3.1-pro`.
- message (`gemini-3.1-flash-lite`) + arbiter (`gemini-3.5-flash`) are ALREADY correct → no change.

## §3. builtin.go default_model refresh: gemini/agy `gemini-2.5-pro` → `gemini-3.1-pro`

The manifest `default_model` (single-value fallback) is currently `gemini-2.5-pro` for both gemini + agy
(builtin.go builtinGemini/builtinAgy). The flagship tier is now `gemini-3.1-pro`. Refresh both. Ripple:

- builtin.go: `builtinGemini()`/`builtinAgy()` `DefaultModel: strPtr("gemini-2.5-pro")` → `gemini-3.1-pro`;
  update the doc-comment mentions of `gemini-2.5-pro`.
- builtin_test.go: `geminiTOML` const (L77) + `agyTOML` const (L135) `default_model = "gemini-2.5-pro"` →
  `gemini-3.1-pro`; `TestBuiltinManifests_GeminiFields` (L444) + `_AgyFields` (L669) `assertStr
  DefaultModel "gemini-2.5-pro"` → `gemini-3.1-pro`; `TestBuiltinManifests_RenderedCommand_Gemini` (L510,512)
  + `_Agy` (L707,709) `gemini-2.5-pro` → `gemini-3.1-pro`.
- providers/gemini.toml: field line `default_model = "gemini-2.5-pro"` (L51) + rendered-command comment
  (L21) → `gemini-3.1-pro`.
- providers/agy.toml: field line `default_model = "gemini-2.5-pro"` (L51) + rendered-command comment (L21) →
  `gemini-3.1-pro`.

Other providers' default_model UNCHANGED: claude="sonnet" (current bare alias; not mandated to change);
codex/cursor/opencode="" (user/config-set); pi="" (FR-D2); qwen-code="qwen3-coder-plus" (S1 ships it; it IS
the flagship, already correct, # TO CONFIRM — S2 does NOT change it).

## §4. bootstrap.go has a STALE local `preferredBuiltins` (consistency fix)

`internal/config/bootstrap.go:15` declares its OWN `preferredBuiltins` (a local copy, documented as
"mirrors internal/provider/registry.go's unexported preferredBuiltins"). It is currently the 7-element
`[pi, opencode, cursor, agy, gemini, codex, claude]` — NO qwen-code. S1 updates registry.go's copy to 8
(with qwen-code), but S1 does NOT touch bootstrap.go (out of S1's scope). So bootstrap.go's copy goes STALE.

Impact: `buildBootstrapConfig` iterates bootstrap.go's preferredBuiltins to emit COMMENTED [role.*] blocks
for OTHER installed providers + `stagerFallback` iterates it. With qwen-code absent: (a) `stagerFallback`
still works (pi is in the list and stager-capable — qwen-code being absent doesn't break the fallback for a
qwen-code target); (b) BUT a qwen-code install would NOT get a commented [role.*] block in `config init`
output. That makes the qwen-code tier row INVISIBLE through config init — half-finishing the feature.

**RESOLUTION**: S2 adds `"qwen-code"` to bootstrap.go's local preferredBuiltins between `"gemini"` and
`"codex"` (keeping the two copies in sync, as the local copy's own doc comment promises). Verified
bootstrap_test.go does NOT assert on the preferredBuiltins slice directly (grep found no such assertion — it
installs specific subsets like pi-only / pi+claude), so the update is safe. This is a borderline-scope
consistency fix but is necessary for "qwen-code has a complete tier row" to surface in the bootstrap.

## §5. role_defaults_test.go — FOUR tests need qwen-code + the gemini refresh

1. `TestDefaultModelsForProvider_PerProvider` — hardcoded `want` table: gemini+agy planner
   `gemini-3.5-pro`→`gemini-3.1-pro`; ADD a `"qwen-code"` entry
   `{planner:"qwen3-coder-plus", stager:"", message:"qwen3-coder-flash", arbiter:"qwen3-coder-plus"}`.
2. `TestDefaultModelsForProvider_AllRolesPresent` — the provider loop `[pi,claude,gemini,agy,opencode,
   codex,cursor]` → ADD `"qwen-code"`.
3. `TestDefaultModelsForProvider_StagerCapability` — the `incapable` slice
   `[gemini,agy,opencode,codex,cursor]` → ADD `"qwen-code"` (PINS stager="" — §1).
4. `TestRoleDefaults_KeySanity` — `expectedProviders` set (7 entries) → ADD `"qwen-code"` (8); the
   `len(roleDefaults) != len(expectedProviders)` guard then expects 8.

## §6. docs/providers.md — qwen-code rows + token refresh + count/order (NARROW scope)

S2's docs scope (item §5): "docs/providers.md reference table (add the qwen-code row, note experimental +
DashScope; refresh the refreshed tokens)". The broader v3-schema doc inconsistencies (default_provider
field, "18 fields", agent→provider terminology) are P4.M2.T1.S1's — OUT OF SCOPE here. S2 touches ONLY:

- "the 7 built-in providers" / "Seven providers are compiled in" → "8" (directly tied to adding the row).
- Auto-detection order line "pi, opencode, cursor, agy, gemini, codex, claude" → insert "qwen-code" between
  gemini and codex.
- Built-in providers TABLE: ADD qwen-code row (stdin / -p / -m / qwen3-coder-plus ⚠️ / prepended /
  --approval-mode default / no); refresh gemini+agy Default-model cell `gemini-2.5-pro`→`gemini-3.1-pro`;
  note qwen-code experimental + DashScope + # TO CONFIRM.
- Per-role (FR-D4) TABLE: ADD qwen-code row (`qwen3-coder-plus` ⚠️ / *(cannot)* / `qwen3-coder-flash` ⚠️ /
  `qwen3-coder-plus` ⚠️); refresh gemini+agy planner `gemini-3.5-pro`→`gemini-3.1-pro`.
- A ⚠️ footnote: qwen-code tokens are # TO CONFIRM per FR-D5.

## §7. S1/S2 file-overlap coordination (NON-overlapping edits within shared files)

S1 (parallel) and S2 BOTH edit `internal/provider/builtin.go` + `internal/provider/builtin_test.go`. The
edits are NON-OVERLAPPING:
- S1: ADDS `builtinQwenCode()` + the map entry + count 7→8 + qwen-code tests (NEW code).
- S2: MODIFIES `builtinGemini()`/`builtinAgy()` DefaultModel + their doc comments + the geminiTOML/agyTOML
  constants + the Gemini/Agy Fields/RenderedCommand tests (EXISTING values).

S2 must NOT touch qwen-code's manifest value (`qwen3-coder-plus` is already the flagship — correct) or any
qwen-code test (S1 owns those). S1 must NOT touch gemini/agy DefaultModel (S2 owns that refresh). The
implementer applies both; the line regions are disjoint so there is no semantic conflict (only ordinary
merge mechanics if applied to the same base).

## §8. Verification-date discipline (FR-D5)

The role_defaults.go verification block is dated "2026-07". S2 updates it to record: the gemini/agy
flagship refresh to `gemini-3.1-pro`, the qwen-code row addition (with `# TO CONFIRM`), the FR-D3
message-tier-cheapest rationale note, and a verification date of 2026-07-02 (today). builtin.go's
gemini/agy doc comments get a brief verified-on note. qwen-code/cursor/codex tokens stay `# TO CONFIRM`
(no live CLI lookup available) per the contract.

## §9. FR-D4 target table (authoritative — from selected_prd_content §9.16)

| Provider | planner | stager | message | arbiter |
|----------|---------|--------|---------|---------|
| pi | gpt-5.4 | gpt-5.4-mini | gpt-5.4-nano | gpt-5.4-mini |
| opencode | openai/gpt-5.4 | openai/gpt-5.4-mini | openai/gpt-5.4-nano | openai/gpt-5.4-mini |
| cursor | gpt-5.4 ⚠️ | (cannot) | gpt-5.4-nano ⚠️ | gpt-5.4-mini ⚠️ |
| agy | gemini-3.1-pro | (cannot) | gemini-3.1-flash-lite | gemini-3.5-flash |
| gemini | gemini-3.1-pro | (cannot) | gemini-3.1-flash-lite | gemini-3.5-flash |
| qwen-code | qwen3-coder-plus ⚠️ | (cannot) | qwen3-coder-flash ⚠️ | qwen3-coder-plus ⚠️ |
| codex | gpt-5.1-codex-max | (cannot) | gpt-5.4-nano | gpt-5.1-codex-mini |
| claude | opus | sonnet | haiku | sonnet |

⚠️ = # TO CONFIRM per FR-D5. "(cannot)" = nil TooledFlags ⇒ stager="" ⇒ bootstrap FR-D4 fallback. This is
the TARGET state for role_defaults.go after S2 (gemini/agy planner was 3.5-pro, now 3.1-pro; qwen-code added).
