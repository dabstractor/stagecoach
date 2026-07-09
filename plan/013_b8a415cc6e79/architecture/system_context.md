# System Context — Provider Lineup Correction (session 013)

## Delta Summary

This session is a **provider lineup correction** delta against the Stagecoach PRD (v2.6 spec).
Two related content corrections with no new product surface:

1. **Remove the `gemini` (Gemini CLI) built-in provider** — superseded by `agy` (Antigravity CLI)
   on 2026-06-18. The built-in count drops from 8 → 7.
2. **Correct the `agy` manifest** against real end-to-end verification (agy v1.1.0, 2026-07-08).
   agy diverged from the gemini-cli lineage: `-p` is value-taking (bare `-p` fails),
   `--approval-mode` was removed (replaced by `--mode plan`), and `--model` replaces `-m`.

## Current Codebase State (verified by research)

**Build + tests pass.** `go build ./...` exits 0, `go test ./...` all packages pass.

### Code-side: FULLY CORRECT (no code changes needed)

| Area | File | Status |
|------|------|--------|
| Built-in count | `internal/provider/builtin.go:22-34` | ✅ Exactly 7: pi, claude, agy, qwen-code, opencode, codex, cursor |
| No gemini built-in | `internal/provider/builtin.go` | ✅ No `builtinGemini()` function; comments note EOL |
| agy manifest fields | `internal/provider/builtin.go:199-217` | ✅ PrintFlag="", ModelFlag="--model", BareFlags=["--mode","plan"], etc. |
| Registry preferredBuiltins | `internal/provider/registry.go:15` | ✅ ["pi","opencode","cursor","agy","qwen-code","codex","claude"] |
| agy role defaults | `internal/config/role_defaults.go:55-61` | ✅ Display labels ("Gemini 3.5 Flash (High/Med/Low)") |
| providers/gemini.toml | N/A | ✅ Does NOT exist |
| providers/agy.toml | `providers/agy.toml` | ✅ Matches corrected manifest, dated 2026-07-08 |

All three implementation commits (`2f77bd0`, `010ecee`, `cdbccf5`) are in place.

### Documentation-side: RESIDUAL DRIFT (9 items)

See `critical_findings.md` for the full drift table. Summary:

- **`docs/providers.md`** (7 items): agy table row cells (print flag, tool-disable approach),
  verification date (2026-07-03 → 2026-07-08), and provider count "8" → "7" in 4 places.
- **`docs/README.md`** (2 items): provider count "8" → "7" and field count "21" → "22".
- **Clean files** (no drift): `README.md`, `docs/cli.md`, `docs/configuration.md`,
  `docs/how-it-works.md`.

## Key Architecture: Provider System

The provider system (`internal/provider/`) is the heart of agent-agnosticism:
- `builtin.go` — compiled-in manifests (7 providers); each factory returns a `Manifest` struct
- `registry.go` — name→manifest map with override merge; `preferredBuiltins` slice for FR-D1
- `manifest.go` — the 22-field `Manifest` struct (TOML-serializable)
- `render.go` — `Render()` command rendering algorithm (§12.2)
- `executor.go` — runs the command, feeds stdin, captures stdout, timeout
- `parse.go` — output parsing pipeline (§12.9)

The agy correction is confined to `builtin.go` (already correct). Documentation must follow.

## Dependencies & Constraints

- **No core changes**: snapshot/CAS/rescue/lock/decompose/config-precedence cores untouched.
- **No new CLI flags, env vars, git-config keys, or config schema changes.**
- **Test fixtures** in `internal/config/*_test.go` and `internal/cmd/models_test.go` use "gemini"
  as arbitrary string fixtures — these test config mechanics, not provider lookups. They are
  stale-but-harmless; renaming is optional polish, out of scope.
