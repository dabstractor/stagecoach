# P2.M1.T1.S1 — Verification Research Notes

Independent re-confirmation of the four contract checks (read-only; no code changes).
Companion to `plan/013_b8a415cc6e79/architecture/code_gemini_agy_audit.md`.

## Environment
- Repo: `/home/dustin/projects/stagecoach`
- HEAD at research time: `fd99358` (descendant of the contract-cited `cdbccf5`
  "Purge gemini-cli from PRD after built-in removal"; the actual code removal is in
  ancestor `010ecee` "Remove gemini-cli provider, switch opencode to stdin delivery").
  The code state at HEAD satisfies every contract item — the `cdbccf5` reference in the
  contract is the removal commit, and the removal persists to HEAD.

## Check (a) — `internal/provider/builtin.go` — PASS
- `BuiltinManifests()` (`func` at `:18`; return map `:19`; entries `:20-26`) returns **exactly 7**
  keys: `pi`, `claude`, `opencode`, `codex`, `cursor`, `agy`, `qwen-code`. No `gemini` key.
- `grep -rn 'func builtinGemini' internal/` → no match (no such factory).
- `grep -rn 'Name:.*"gemini"' internal/ providers/` → no match (no gemini Name entry).
- All remaining "gemini" hits in builtin.go are comment-only (lineage / model-label prose).

## Check (b) — `internal/provider/registry.go:15` — PASS
- `var preferredBuiltins = []string{"pi", "opencode", "cursor", "agy", "qwen-code", "codex", "claude"}`
  (exact, at `:15`). 7 entries, no `gemini`.
- Kept in sync with `BuiltinManifests()` keys by `TestPreferredBuiltins_MatchesBuiltinKeys`
  (`internal/provider/registry_test.go:15`), which also asserts the exact FR-D1 order.

## Check (c) — `providers/` TOML set — PASS
- `ls -1 providers/*.toml` → `agy.toml claude.toml codex.toml cursor.toml opencode.toml pi.toml
  qwen-code.toml` = **exactly 7 files**. `providers/gemini.toml` does NOT exist.
- Coverage enforced by `TestProviderReferenceFiles_AllBuiltinsCovered`
  (`internal/provider/referencefiles_test.go:68`): every builtin has a reference file and vice-versa.

## Check (d) — `internal/config/role_defaults.go` — PASS
- `var roleDefaults` declared at `:52`; exactly 7 map keys: `pi`(:53), `claude`(:59),
  `agy`(:65), `qwen-code`(:73), `opencode`(:79), `codex`(:85), `cursor`(:91). No `gemini` key.
- `grep -n 'gemini' internal/config/role_defaults.go` → no match.

## Non-drift (per contract §3 NOTE) — leave alone
- `internal/config/load_test.go` uses `"gemini"` as an **opaque provider name string** and
  `"gemini-2.5-pro"`/`"gemini-2.5-flash"` as **opaque model strings** to exercise config
  field-merge / precedence mechanics. These are NOT built-in lookups; renaming is out of scope.
- Historical `plan/00x/` + `PRD.md` snapshots reference gemini — immutable planning artifacts.

## Validation gates verified working
- `go build ./...` → exit 0 (clean).
- `go test ./internal/provider/... ./internal/config/...` → both `ok` (PASS).
- Regression guards that would FAIL if gemini were re-added:
  - `TestBuiltinManifests_KeysAndCount` (builtin_test.go:209) asserts `len == 7` + exact key set.
  - `TestPreferredBuiltins_MatchesBuiltinKeys` (registry_test.go:15) asserts exact FR-D1 order.
  - `TestProviderReferenceFiles_AllBuiltinsCovered` (referencefiles_test.go:68) asserts
    builtin↔reference-file parity.
