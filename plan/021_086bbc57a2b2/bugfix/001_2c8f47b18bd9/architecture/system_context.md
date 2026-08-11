# System Context: Bug Fix Suite 001

## Project: stagecoach
**Language**: Go 1.22 (`github.com/dabstractor/stagecoach`)
**Key Dependencies**: cobra (CLI), pelletier/go-toml/v2 (config), spf13/pflag, yaml.v3

## Architecture Overview

stagecoach is a Git commit-message generation tool that wraps coding-agent CLIs (pi, claude, opencode, etc.) to produce conventional-commit-style messages. It has four main subsystems relevant to this bugfix:

### 1. Work-Description Mode (`internal/generate/workdesc.go`)
- **Purpose**: A description-first commit mode where the model leads with a user-supplied description + file skeleton and pulls file diffs on demand via a `READ <path>` text protocol.
- **Key types/functions**:
  - `RunWorkDescription(ctx, deps, cfg, manifest, sysPrompt, payload, skeleton, msgModel, msgReasoning) (msg, ok, cause)` — line 67, the main read/answer loop driver
  - `readState{rounds, N, offsets map[string]int}` — per-session state (round count + per-file byte cursor)
  - `parseReadLines(response, skeleton string) []string` — line 145, extracts READ <path> requests, filters to staged paths
  - `stripReadLines(response string) string` — line 189, removes READ lines from a response
  - `buildReadAnswer(ctx, g, cfg, excludes, paths, st) string` — line ~260, builds the diff-chunk answer turn
  - `nextChunk(diff, offset) (chunk, total, advance)` — line ~300, returns chunk + total + byte advance
  - `chunkRuneBudget()` — returns `readChunkTokenCap * 4` (64000 runes)
  - `skeletonPaths(skeleton) map[string]bool` — parses the numstat skeleton into staged path set
- **Loop control flow** (RunWorkDescription):
  1. Turn 1: system prompt + description-first payload → Execute → `out`
  2. For turns 2+: `paths := parseReadLines(out, skeleton)`
     - If `len(paths) == 0` → FR-W7: parse `out` as the commit message via `ParseOutput(out, manifest)` → **BUG-001 is here**
     - If round cap reached → forced conclusion: `stripReadLines(out2)` then ParseOutput
     - Otherwise: `buildReadAnswer(...)` → Execute → next iteration
- **Provider interaction**: Uses `manifest.RenderMultiTurn` + `provider.Execute` per turn; session_mode="append"
- **External deps**: `git.StagedFileDiff` (cursor-unaware, returns FULL diff every call), `provider.ParseOutput`

### 2. Decompose Pipeline (`internal/decompose/`)
- **Purpose**: Multi-commit decomposition (planner → stager → message → arbiter pipeline)
- **Key files**:
  - `roles.go` — `ResolveRoles(cfg, reg) (RoleManifests, RoleModels, error)` resolves four agent roles
  - `decompose.go` — `Decompose(ctx, deps) (DecomposeResult, error)` main orchestrator
- **Call chain** (CLI): `cmd/default_action.go:452` → `decompose.ResolveRoles(*cfg, reg)` → builds `Deps` → `decompose.Decompose(ctx, deps)` at line 466
- **Also**: `pkg/stagecoach/stagecoach.go:203` → `decompose.ResolveRoles(cfg, reg)` → `decompose.Decompose(ctx, deps)` at line 219
- **FR-M13 fast-path**: `isFileDisjoint(out.Commits)` at `decompose.go:243` → `runLoopFastPath` (no stager agent needed)
- **CRITICAL ORDERING**: `ResolveRoles` runs BEFORE the planner, BEFORE disjointness is determined. The stager requirement in `ResolveRoles` (roles.go:~132) gates the fast-path.

### 3. Upgrade Subsystem (`internal/upgrade/`, `internal/cmd/upgrade_run.go`)
- **Purpose**: Self-update detection and binary swap
- **Key files**:
  - `detect.go` — `Detector.Detect(ctx)` with tier-(a)/(b)/(c) cascade; `osRunner` (line ~121, unexported) has 3s timeout + WaitDelay
  - `releases.go` — GitHub API client for releases
  - `internal/cmd/upgrade_run.go` — CLI dispatch + `cmdRunner` (line ~152, the PRODUCTION runner, NO timeout/WaitDelay)
- **Root context**: `cmd/stagecoach/main.go:62` — `signal.Install(context.Background(), ...)` — signal-cancelable but NO deadline

### 4. Lock/Watchdog (`internal/lock/`, `internal/watchdog/`)
- **Purpose**: File-based locking with CAS + orphaned-run reclamation
- **Key files**:
  - `lock.go` — `reapStaleLocks(dir)` (line ~128), `Acquire`, `Release`
  - `lock_windows.go` — `processAlive(pid, hostname) bool` always returns true
  - `lock/orphan_unix.go` — `appearsOrphaned(pid) bool` uses ppid==1 test
  - `watchdog/arm_unix.go` — `armImpl(originalPpid, interval, n)` uses parent-pid CHANGE detection (CORRECT)

## Bug Summary (all 12 confirmed against source code)

| BUG | Severity | Subsystem | File:Line | Status |
|-----|----------|-----------|-----------|--------|
| BUG-001 | Major | Work-desc | workdesc.go:99 | Confirmed — non-staged READ becomes commit subject |
| BUG-002 | Minor | Work-desc | workdesc.go:281 | Confirmed — exhausted cursor emits empty body |
| BUG-003 | Major | Decompose | roles.go:132 | Confirmed — eager stager blocks fast-path |
| BUG-004 | Major | Upgrade | upgrade_run.go:152 | Confirmed — cmdRunner has no timeout/WaitDelay |
| BUG-005 | Minor | Work-desc | workdesc.go:306 | Confirmed — chunks anchor to newlines not @@ edges |
| BUG-006 | Minor | Work-desc | workdesc.go:284 | Confirmed — part label mixes byte/rune units |
| BUG-007 | Minor | Upgrade | detect.go:368 | Confirmed — no Linuxbrew Cellar entry |
| BUG-008 | Minor | Upgrade | releases.go:179 | Confirmed — c.Repo unescaped |
| BUG-009 | Minor | Lock | lock.go:128 | Confirmed — TOCTOU in reapStaleLocks |
| BUG-010 | Minor | Lock | lock_windows.go:19 | Confirmed — processAlive always true on Windows |
| BUG-011 | Minor | Lock | orphan_unix.go:42 | Confirmed — ppid==1 under-reports under subreapers |
| BUG-012 | Minor | Decompose | decompose.go:981 | Confirmed — stale comment about arbiter staging |

## Test Infrastructure
- Work-desc tests: `internal/generate/generate_workdesc_test.go` — 3 layers (prompt builders, pure parser tests, e2e with stub agent via `stubtest` package)
- Decompose tests: `internal/decompose/*_test.go` (roles_test.go, decompose_test.go, etc.)
- Upgrade tests: in `internal/cmd/` and `internal/upgrade/`
- Lock tests: in `internal/lock/` and `internal/e2e/orphan_reclaim_scenarios_test.go`
- Pattern: tests use injectable seams (function vars overridden in tests with t.Cleanup restore)

## Relevant FR Specifications
- **FR-W3** (spec/01-product.md:524): non-staged READ paths "ignored with a short note" — NOT treated as commit message
- **FR-W5** (spec/01-product.md:526): "After the final chunk, a re-request returns '<path> — end of diff'"; "Chunk boundaries hug @@ hunk edges"
- **FR-W7** (spec/01-product.md:528): "A response with no valid READ line is the commit-message candidate"
- **FR-M13** (spec/03-generation.md:120): no-tooled provider "can now decompose a disjoint tree"
- **FR-U2(b)** (spec/01-product.md:562): tier-(b) PM DB queries; external_deps §7: "these queries must not hang"
- **FR-K2** (spec/SPEC.md:19): parent-pid CHANGE detection, not getppid()==1