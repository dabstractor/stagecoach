# System Context — Plan 015 (v2.8 Delta)

## What this plan addresses

The v2.8 PRD revision adds two focused themes on top of the fully-implemented v2.7 codebase:

### Theme 1 — Per-role generation timeouts (FR-R7, §9.15)
Each role (planner, stager, message, arbiter) resolves its OWN timeout independently.
- **Global default changes 480s → 120s** (the fallback any role inherits)
- **Planner role built-in default: 480s** (the heavy role — reasoning over the entire frozen T_start diff)
- Stager/message/arbiter inherit 120s, each independently tunable
- Per-role overrides: `--<role>-timeout` / `STAGECOACH_<ROLE>_TIMEOUT` / `[role.<role>].timeout` / `stagecoach.role.<role>.timeout`
- Mirrors the existing per-role provider/model/reasoning resolution pattern (FR-R3)
- Motivated by a real `decompose: planner failed: context deadline exceeded` with thinking-high planner over large diff

### Theme 2 — Decomposition freeze hardening (FR-M1e + amended FR-M1c, §9.14)
- **FR-M1e (new):** The `Decompose()` entrypoint re-asserts FR-M1's empty-index precondition and
  refuses with a clear, actionable error if the index is non-empty (defense-in-depth — the trigger
  is the primary guard, but a stale/buggy trigger now fails loudly)
- **FR-M1c (amended):** The hard error now names the offending paths AND provides a remedy, replacing
  the opaque `freeze violation: … not traceable to T_start` message

## Current codebase state (verified by research)

The codebase is mature (~68K lines of Go). v1.0 single-commit core + v2.0–v2.7 features (decompose,
hooks, integrate, lock, orphan reclamation, multi-turn, work-description) are all implemented and
tested. Build + tests pass today.

### Timeout handling today
- **One** timeout: `Config.Timeout` (time.Duration, default **480s**) — used by ALL roles
- `provider.Execute(ctx, spec, timeout, vb)` already takes a per-call timeout parameter — **no executor change needed**
- 13 Execute call sites all pass `cfg.Timeout` or `deps.Config.Timeout` (the same flat value)
- `RoleConfig` struct has `{Provider, Model, Reasoning}` — **no Timeout field**
- Git-config layer has **NO per-role support** (not even for provider/model/reasoning)
- Global timeout is configured across 7 precedence layers: defaults → global file → repo file → git config → env → flags

### Decompose freeze today
- `Decompose()` does **NOT** re-check the empty-index precondition — trusts the CLI router
- `FreezeWorkingTree` calls `AddAll` first — staged content gets swept into T_start silently
- `verifyFreezeSubset` (stager.go:158) already names offending paths in both checks (path + content)
- BUT: phrasing is opaque ("not traceable to T_start"), concept identified by numeric index not title, no remedy provided
- No `StagedNames()` git method exists — `StagedFileCount` runs `git diff --cached --name-only` but discards paths

## Key files impacted

### Theme 1 (per-role timeouts)
| File | Change |
|------|--------|
| `internal/config/config.go` | Add `Timeout time.Duration` to `RoleConfig` (L36); change `Defaults().Timeout` 480s→120s (L197) |
| `internal/config/roles.go` | Add `ResolveRoleTimeout(role, cfg) time.Duration` |
| `internal/config/role_defaults.go` | Add `defaultRoleTimeouts` map (planner=480s) |
| `internal/config/load.go` | Add `setRoleTimeout` helper; `_TIMEOUT` in env loop; `-timeout` in flag loop |
| `internal/config/file.go` | Add `Timeout string` to `fileRoleConfig`; **rewrite materialize loop** (breaks `RoleConfig(frc)` conversion); field-merge in overlay |
| `internal/config/git.go` | Add per-role git-config reading (NEW infrastructure — `stagecoach.role.<role>.timeout`) |
| `internal/cmd/root.go` | Add 4 `--<role>-timeout` StringVar flags |
| `internal/generate/generate.go` | Resolve message-role timeout (L335 + multi-turn budget L426) |
| `internal/generate/multiturn.go` | Lines 165/176/187: resolved message-role timeout |
| `internal/generate/workdesc.go` | Lines 75/106/122: message-role timeout |
| `internal/hook/exec.go` | Line 182: message-role timeout |
| `internal/decompose/planner.go` | Line 124: resolved planner timeout |
| `internal/decompose/stager.go` | Line 110: resolved stager timeout |
| `internal/decompose/message.go` | Line 155: resolved message timeout |
| `internal/decompose/arbiter.go` | Line 100: resolved arbiter timeout |
| 4 test files | Fix pinned 480s default tests (config_test, file_test, load_test) |

### Theme 2 (freeze hardening)
| File | Change |
|------|--------|
| `internal/git/git.go` | Add `StagedNames() ([]string, error)` to interface + `gitRunner` |
| `internal/decompose/decompose.go` | FR-M1e: empty-index re-check at top of `Decompose()` before `FreezeWorkingTree` |
| `internal/decompose/stager.go` | FR-M1c: improve `verifyFreezeSubset` error messages (concept title + remedy) |

## Risks

1. **`RoleConfig(frc)` conversion break (file.go:316)** — adding `time.Duration` to `RoleConfig` while
   `fileRoleConfig.Timeout` stays `string` makes the direct type conversion non-compilable. The
   materialize loop must be rewritten to construct `RoleConfig` field-by-field + parse the timeout.
2. **Default change 480s→120s is a behavior change** for all existing users — every role currently gets 480s.
   Must fix ~4 tests that pin the old default.
3. **Git-config per-role is NEW infrastructure** — the git layer reads NO per-role keys today. FR-R7
   requires adding a per-role git-config loop in `loadGitConfig`.
4. **FR-M1e ordering is critical** — the empty-index check must run BEFORE `FreezeWorkingTree` (which
   calls `AddAll`), otherwise the check is meaningless.
5. **No `StagedNames()` primitive exists** — FR-M1e needs to name offending staged paths in the error.
