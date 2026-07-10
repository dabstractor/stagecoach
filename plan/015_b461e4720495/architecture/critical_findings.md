# Critical Findings — Plan 015 (v2.8 Delta)

## Finding 1: RoleConfig(frc) conversion will BREAK (HIGHEST RISK)

**Location:** `internal/config/file.go:316`

The current materialize loop uses a **direct struct type conversion**:
```go
c.Roles[role] = RoleConfig(frc)
```

This works ONLY because `fileRoleConfig` and `RoleConfig` have identical field types (all `string`).
Adding `Timeout time.Duration` to `RoleConfig` while `fileRoleConfig.Timeout` stays `string` makes
this conversion non-compilable.

**Fix:** Rewrite the materialize loop (file.go:311-317) to construct `RoleConfig` field-by-field and
parse each role's timeout string. Mirror how the global timeout is parsed in `loadTOML` (file.go:179).

## Finding 2: No per-role git-config support exists today

**Location:** `internal/config/git.go`

The git-config layer reads NO per-role keys at all — not even for provider/model/reasoning.
The `--planner-model` flag help text says "git stagecoach.role.planner" but that git key is **not
actually read** today. Per-role overrides today come only from file, env, and CLI flags.

FR-R7 requires `stagecoach.role.<role>.timeout` — this is NEW infrastructure. A per-role loop over
`roleNames` must be added to `loadGitConfig`, reading `stagecoach.role.<role>.timeout` (and optionally
provider/model/reasoning for consistency).

## Finding 3: Execute already takes per-call timeout — NO executor change

**Location:** `internal/provider/executor.go:44-52`

```go
func Execute(ctx context.Context, spec CmdSpec, timeout time.Duration, vb *ui.Verbose) (...) {
    if timeout > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, timeout)
        defer cancel()
    }
    ...
}
```

The timeout is already a per-call parameter. FR-R7 only needs each caller to pass a role-resolved
duration instead of the flat `cfg.Timeout`. **Zero changes to `Execute`.**

## Finding 4: 13 Execute call sites must be updated

| Role | File:Line | Current timeout |
|------|-----------|-----------------|
| message (single) | generate.go:335 | `cfg.Timeout` |
| message (multi-turn) | multiturn.go:165, 176, 187 | `cfg.Timeout` |
| message (workdesc) | workdesc.go:75, 106, 122 | `cfg.Timeout` |
| message (hook) | hook/exec.go:182 | `cfg.Timeout` |
| planner | decompose/planner.go:124 | `deps.Config.Timeout` |
| stager | decompose/stager.go:110 | `deps.Config.Timeout` |
| message (decompose) | decompose/message.go:155 | `deps.Config.Timeout` |
| arbiter | decompose/arbiter.go:100 | `deps.Config.Timeout` |

## Finding 5: Default 480s→120s breaks ~4 tests

These tests pin the old 480s default and MUST be updated:
- `internal/config/config_test.go:20-21` — `if c.Timeout != 480*time.Second`
- `internal/config/file_test.go:113, 787` — pins `Timeout=480s`
- `internal/config/load_test.go:589-590` — `if cfg.Timeout != 480*time.Second`
- `internal/cmd/root.go:133` — `--timeout` help text says "default 480s"
- `internal/config/bootstrap.go:161` — commented template `# timeout = "480s"`

## Finding 6: FR-M1c errors ALREADY name paths

**Location:** `internal/decompose/stager.go:158-196`

Both `verifyFreezeSubset` error messages already include the offending paths via `strings.Join(extra, ", ")`.
The improvement per FR-M1c is:
- Make the phrasing less opaque (replace "not traceable to T_start" with clearer language)
- Include the concept TITLE (currently only numeric index `i` — `verifyFreezeSubset` doesn't receive it)
- Add a remedy line (e.g., "this indicates concurrent working-tree changes; stage them separately")

## Finding 7: Decompose() has an explicit doc-comment saying it does NOT re-check

**Location:** `internal/decompose/decompose.go:~L125`

> PRECONDITION (FR-M1, owned by the CLI router — P4.M1.T1.S1): the caller routed
> here because NOTHING is staged... Decompose does NOT re-check this; it assumes correct routing.

FR-M1e changes this contract: `Decompose()` gains a defense-in-depth re-check at the very top,
before `FreezeWorkingTree` (which calls `AddAll` and would sweep stale staged content into T_start).

## Finding 8: Multi-turn total-budget computation uses flat timeout

**Location:** `internal/generate/generate.go:~L423-426`

```go
totalMin := int((cfg.Timeout * time.Duration(turns)).Minutes())
```

After FR-R7, this must use the resolved message-role timeout (FR-T5: "per-turn timeout = the
message role's resolved timeout"). The progress line "falling back to multi-turn: N+1 turns, ~Mm
total" must reflect the correct per-role budget.

## Finding 9: parseTimeout exists and handles both "120s" and bare "120"

**Location:** `internal/config/load.go:615-625`

```go
func parseTimeout(s string) (time.Duration, error) {
    if d, err := time.ParseDuration(s); err == nil { return d, nil }
    if n, err := strconv.Atoi(s); err == nil { return time.Duration(n) * time.Second, nil }
    return 0, fmt.Errorf("invalid timeout %q ...", s)
}
```

This is the single parse helper to reuse for per-role timeout strings everywhere (env, flag, git-config).
NOTE: the file layer uses `time.ParseDuration` directly (only accepts "120s" form, NOT bare "120") —
this is a pre-existing inconsistency. Per-role file timeouts should use `parseTimeout` for consistency
with env/flag/git.

## Finding 10: Per-role timeout has different semantics per role

| Role | On timeout | Path |
|------|-----------|------|
| planner | `ErrPlannerFailed` (non-rescue, no retry) | Pre-staging; planning precedes all staging |
| stager | `ErrStagerFailed` | Mid-loop; retried once then empty-skip |
| message | `RescueError{ErrTimeout}` → exit 124 | Rescue path |
| arbiter | Graceful `null` (NOT an error) | Arbiter OWNS the null decision |

These distinct semantics are preserved by FR-R7 — only the timeout VALUE changes, not the error handling.
