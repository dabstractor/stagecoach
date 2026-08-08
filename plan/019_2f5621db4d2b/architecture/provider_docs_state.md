# Provider Stager-Capability State (for the docs/providers.md sync)

Source of truth for CAPABILITY = `internal/provider/builtin.go` `TooledFlags` (non-nil ⇔ stager-capable).
Verified by direct read 2026-08-08.

## builtin.go — the capability matrix (authoritative)

| # | Provider | func @line | TooledFlags | TooledRepoDirFlag | Stager-capable? | Scope model |
|---|----------|-----------|-------------|-------------------|-----------------|-------------|
| 1 | pi       | L50  | non-nil (`--no-extensions --no-skills ...` L97-103) | nil | **YES** | UNSCOPED |
| 2 | claude   | L124 | non-nil (`--allowed-tools Bash(git ...),Read,Edit` L149-153) | nil | **YES** | **STRUCTURALLY-scoped** (git allowlist) |
| 3 | agy      | L223 | non-nil (`--mode accept-edits --dangerously-skip-permissions` L244-247) | `--add-dir` (L248) | **YES** | UNSCOPED |
| 4 | qwen-code| L286 | **NIL** (L303 comment) | nil | **NO** | — |
| 5 | opencode | L331 | non-nil (`--agent build` L351-353) | nil | **YES** | UNSCOPED |
| 6 | codex    | L393 | non-nil (`--sandbox danger-full-access` L415-417) | nil | **YES** | UNSCOPED |
| 7 | cursor   | L445 | **NIL** (absent) | nil | **NO** | — |

**Result: 5 stager-capable (pi, claude, agy, opencode, codex); 2 not (qwen-code, cursor).**
Per spec §12.7.1: claude is the ONLY STRUCTURALLY-scoped stager; pi/agy/codex/opencode are UNSCOPED
(safety = §17.6 stager prompt + HEAD-movement guard + verifyFreezeSubset, NOT flag-scoping).

## docs/providers.md — stale regions (the file Phase 2 edits)

### Region A — main provider table "Stager?" column (L80-86)
| Line | Provider | Current cell | Correct (per builtin.go) | Stale? |
|------|----------|--------------|--------------------------|--------|
| L80  | pi       | `✓ yes`       | `✓ yes`                   | OK |
| L81  | claude   | `✓ yes`       | `✓ yes` (scoped)          | OK |
| L82  | opencode | `— no`        | `✓ yes (unscoped)`        | **STALE** |
| L83  | codex    | `— no`        | `✓ yes (unscoped)`        | **STALE** |
| L84  | cursor   | `— no`        | `— no`                    | OK |
| L85  | agy      | `— no`        | `✓ yes (unscoped)`        | **STALE** |
| L86  | qwen-code| `— no ⚠️`     | `— no`                    | OK |
THREE stale cells: **opencode, codex, agy** (all wrongly say "no").

### Region B — prose note (L88)
> "...`agy` is **experimental** ... pending the remaining §12.5.1.1 checklist items ... and cannot
> serve as a stager (empty `tooled_flags`)."
STALE: agy HAS tooled_flags now; §12.5.1.1 checklist is cleared (agy stager verified 2026-07-09,
v1.1.11). Drop the "cannot serve as a stager" clause. Leave the qwen-code sentence (still non-stager).

### Region C — "Tools-disable asymmetry" section (L90-100)
Currently frames bare-mode tool-disable. Add: agy + codex + opencode join pi as stager-capable via the
UNSCOPED tooled profile; claude is the ONLY STRUCTURALLY-scoped stager. Cross-ref §12.7.1 + §17.6
stager prompt + HEAD-guard + verifyFreezeSubset.

### Region D — per-role models table stager column + footer (L130-142) — ⚠️ COUPLED TO STALE CODE
The per-role table shows `*(cannot)*` for agy (L133), opencode (L134), codex (L135). This table
MIRRORS `internal/config/role_defaults.go` `roleDefaults`. The footer (L142) says:
> "FR-D4 fallback — currently pi or claude."
BOTH are stale, but see `role_defaults_drift.md` — role_defaults.go is CODE that is ALSO stale
(stager="" for agy/opencode/codex). This is a doc-vs-code-vs-code tangle; resolution documented in
the residual-risk file. The footer's CAPABILITY claim ("currently pi or claude") is wrong regardless
(now 5 stager-capable) and can be fixed; the `*(cannot)*` cells depend on role_defaults.go.

## spec/02-providers.md cross-check
- **agy §12.5.1** — CORRECTLY stager-capable (L364 prose; L386 `tooled_flags=["--mode","accept-edits",
  "--dangerously-skip-permissions"]`; L387 `tooled_repo_dir_flag="--add-dir"`; §12.5.1.1 item 4 L414
  RESOLVED). ✅ matches code.
- **codex §12.7** — ⚠️ **SPEC GAP**: the §12.7 codex manifest (L470-500) has NO `tooled_flags`, old
  `bare_flags` (`--ask-for-approval`), `prompt_delivery="positional"`. Code (builtin.go L415-417) is
  AHEAD of spec. Per AGENTS.md hard rule 1, DO NOT edit `spec/` — surface it. The docs/providers.md
  edit for codex is grounded in the CODE, with the spec gap flagged as a tracked item.
- **opencode §12.6** — not inspected for tooled_flags in the spec this pass; code says stager-capable.