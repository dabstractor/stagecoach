# External Dependencies & Cross-Cutting Concerns

## Go Standard Library (used in bugfixes)

### `os/exec` (BUG-004, BUG-009)
- `exec.CommandContext(ctx, name, args...)` — creates a command bound to a context
- `cmd.WaitDelay` — bounds how long `Run()` waits for the process to release I/O AFTER context cancellation (Go 1.20+). Critical for forked grandchildren holding the stdout pipe.
- `context.WithTimeout(parent, timeout)` — per-query deadline derivation
- **Pattern**: `internal/upgrade/detect.go:osRunner.Run` (line ~121) is the reference implementation with both timeout + WaitDelay

### `net/url` (BUG-008)
- `url.PathEscape(s)` — percent-escapes a path segment
- **Pattern**: `internal/upgrade/releases.go:ReleaseByTag` (line ~198) correctly escapes `tag` via `url.PathEscape(tag)`

### `strings` (BUG-001, BUG-002, BUG-005, BUG-006)
- `strings.IndexByte(s, b)` — finds first occurrence of byte
- `strings.Split(s, sep)` — splits string
- `strings.Fields(s)` — splits on whitespace
- `strings.HasPrefix`, `strings.TrimSpace`, etc.

## Internal Package Dependencies

### `internal/git` (Work-desc subsystem)
- `git.StagedFileDiff(ctx, path, opts) (string, error)` — returns the FULL staged diff for a single path. **Cursor-unaware** — returns the same diff every call. The cursor is tracked externally in `readState.offsets`.
- `git.StagedDiffOptions` — { MaxDiffBytes, MaxMDLines, BinaryExtensions, Excludes, DiffContext }
- `git.StagedNumstatSkeleton(ctx) (string, error)` — the numstat block that doubles as the READ-able path menu

### `internal/provider` (Work-desc + Decompose subsystems)
- `provider.Manifest` — the merged-but-unresolved provider manifest (has `TooledFlags`, `ProviderFlag`, `Name`, etc.)
- `provider.ParseOutput(out, manifest) (msg string, ok bool, err error)` — parses a model response for the commit message
- `provider.Execute(ctx, spec, timeout, verbose) (out string, stderr string, err error)` — runs the provider
- `provider.Registry` — `Get(name)`, `IsInstalled(manifest)`, `FirstTooledProvider(installed)`, `DefaultProvider(installed)`
- `manifest.RenderMultiTurn(model, sysPrompt, payload, reasoning, sessionID, turn) (*provider.Spec, error)`

### `internal/config` (Decompose subsystem)
- `config.Config` — the fully-resolved config struct
- `config.ResolveRoleModel(role, cfg) (provider, model, reasoning)` — per-role field merge
- `config.DefaultModelsForProvider(provider) map[string]string` — default model table
- `config.RoleConfig` — {Provider, Model, Reasoning}

## Provider Manifest: `TooledFlags` (FR-D4)
- `TooledFlags` is a `[]string` on `provider.Manifest` — flags passed to the provider to enable tool-calling/staging
- Only built-in agents with tool-calling support have non-empty TooledFlags: `pi`, `claude`
- Providers without TooledFlags: `opencode`, `agy`, `cursor`, `codex`, and any user-defined providers
- `FirstTooledProvider(installed)` scans `preferredBuiltins` (pi first, then claude) for the first installed, stager-capable provider. **Only built-ins are candidates** (registry.go:127).

## Existing Test Patterns

### Work-Description Tests (`generate_workdesc_test.go`)
- **Pure unit tests**: `TestParseReadLines_*`, `TestStripReadLines`, `TestSkeletonPaths_*`, `TestNextChunk_*`
- **E2E with stub agent**: `TestCommitStaged_WorkDescription_HappyPath`, `...RoundBudgetForcesConclusion`, `...NoCascadeToMultiTurn`, `...NonAppendProviderRescues`
- Uses `stubtest` package for stub-agent integration
- Pattern: set up a fake git.Git + config, call `CommitStaged` or `RunWorkDescription` directly

### Decompose Tests (`internal/decompose/*_test.go`)
- `roles_test.go` — tests `ResolveRoles` with various provider configurations
- Pattern: build a `*provider.Registry` with overridden providers, call `ResolveRoles(cfg, reg)`

### Upgrade Tests
- `internal/cmd/` tests use seam overrides (function vars with t.Cleanup restore)
- Pattern: override `upgradeDetect`, `upgradeSwap`, etc. to return canned results

### Lock/Watchdog Tests
- `internal/lock/` — tests `Acquire`/`Release` with temp dirs
- `internal/e2e/orphan_reclaim_scenarios_test.go` — e2e orphan reclamation scenarios

## Key Design Constraints
1. **No spec changes** — all bugs are code-fixing to match existing FRs
2. **No PRP needed** — bug fixes per AGENTS.md rule 3
3. **Tests implied** — every subtask implies TDD workflow
4. **Commit as user** — never as "stagecoach" or agent identity (AGENTS.md rule 4)
5. **CI must pass** — after change is complete (AGENTS.md "When a change is complete")