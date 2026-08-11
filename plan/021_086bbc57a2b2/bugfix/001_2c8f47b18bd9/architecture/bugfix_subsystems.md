# Bug Fix Analysis: Decompose & Upgrade & Lock/Watchdog Subsystems

## BUG-003 (MAJOR): Eager stager resolution blocks FR-M13 file-disjoint fast-path

### Root Cause
In `ResolveRoles` (`internal/decompose/roles.go`, line ~95), the function iterates over all four roles `["planner", "stager", "message", "arbiter"]` and for EACH role resolves the provider, validates, and checks installation. For the "stager" role specifically (line ~132):

```go
if role == "stager" && len(m.TooledFlags) == 0 {
    fb := reg.FirstTooledProvider(installed)
    if fb == "" {
        return RoleManifests{}, RoleModels{}, fmt.Errorf(
            "role %q: provider %q cannot stage (tooled_flags empty) and no other installed "+
                "provider is stager-capable", role, prov)
    }
    // ... fallback logic ...
}
```

This hard-errors BEFORE the planner runs and BEFORE disjointness is determined. But FR-M13 (v3.2) says a no-tooled provider can decompose a disjoint tree WITHOUT needing a stager (the fast-path at `decompose.go:243` uses `isFileDisjoint` and calls `runLoopFastPath` which needs NO stager agent).

The problem: `ResolveRoles` is called at `decompose.go:203` (in `pkg/stagecoach/stagecoach.go`) or `default_action.go:452` BEFORE `Decompose(ctx, deps)` at line 219/466. So the stager check is a gate that blocks reaching the fast-path.

Additionally, `FirstTooledProvider` (registry.go:127) only considers `preferredBuiltins` (pi, claude), never user-defined tooled providers.

### Fix Strategy (two options, PRD recommends both)
1. **Defer/lazy the stager requirement**: In `ResolveRoles`, when the stager has no TooledFlags and no fallback is found, do NOT error. Instead, set the stager manifest to a sentinel/nil and return successfully. Then in `Decompose`, if the partition is file-disjoint → use the fast-path (no stager needed). If the partition is NOT disjoint → the tooled-stager path runs and THEN errors if no stager is available.

2. **Let `FirstTooledProvider` also consider user-defined tooled providers**: Currently it only scans `preferredBuiltins`. Add a secondary scan over user-defined providers with non-empty TooledFlags.

The PRD recommends option 1 (defer the stager requirement) as the primary fix, with option 2 as an enhancement.

### Implementation Details
- `ResolveRoles` returns `(RoleManifests, RoleModels, error)`. The `RoleManifests.Stager` field and `RoleModels.Stager` would need to carry a sentinel or flag indicating "no stager available, fast-path only."
- The `Decompose` function's tooled-stager path (`runLoop`) calls `invokeStager` → `stageConcept` which would need to check for the sentinel and error with a clear message like "this partition requires a tooled stager, but none is available."
- The fast-path (`runLoopFastPath`) does NOT use the stager at all — it stages deterministically with `git add`.

### Test Strategy
- Unit test in `roles_test.go`: Build a registry with a no-tooled custom provider + all built-ins bogus. `ResolveRoles` should NOT error.
- Integration test in `decompose_test.go`: A disjoint partition with a no-tooled provider should succeed via the fast-path.
- Integration test: A NON-disjoint partition with a no-tooled provider should error with a clear message at the `runLoop` stage.

### Files to Modify
- `internal/decompose/roles.go` — `ResolveRoles` (defer the stager error)
- `internal/decompose/decompose.go` — `runLoop` or `invokeStager` (error if stager needed but unavailable)
- Possibly `internal/provider/registry.go` — `FirstTooledProvider` (add user-defined tooled scan)

---

## BUG-004 (MAJOR): Upgrade cmdRunner has no per-query timeout or WaitDelay

### Root Cause
`internal/cmd/upgrade_run.go` defines `cmdRunner` (line ~147) as the production runner for tier-(b) PM DB queries. Its `Run` method (line ~162):

```go
func (cmdRunner) Run(ctx context.Context, name string, args ...string) (string, int, error) {
    var buf bytes.Buffer
    cmd := exec.CommandContext(ctx, name, args...)
    cmd.Stdout = &buf
    if err := cmd.Run(); err != nil {
        // ...
    }
    return buf.String(), 0, nil
}
```

NO `context.WithTimeout` and NO `cmd.WaitDelay`. The comment claims "the command's ctx already bounds the whole upgrade," but the root context from `main.go:62` is `signal.Install(context.Background(), ...)` — signal-cancelable with NO deadline.

The reference implementation in `internal/upgrade/detect.go:osRunner.Run` (line ~121) correctly has:
- `context.WithTimeout(ctx, timeout)` where `timeout = defaultQueryTimeout` (3 seconds)
- `cmd.WaitDelay = timeout`

But `osRunner` is UNEXPORTED, so package `cmd` created a timeout-less twin.

### Fix Strategy
**Option A (preferred)**: Export `upgrade.osRunner` (or its timeout constant + construction) so package `cmd` can reuse it directly. This eliminates code duplication.

**Option B**: Replicate the timeout + WaitDelay in `cmdRunner.Run`:
```go
func (cmdRunner) Run(ctx context.Context, name string, args ...string) (string, int, error) {
    ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
    defer cancel()
    var buf bytes.Buffer
    cmd := exec.CommandContext(ctx, name, args...)
    cmd.Stdout = &buf
    cmd.WaitDelay = 3 * time.Second
    // ...
}
```

Either way, the fix must match `osRunner`'s contract: non-zero exit → `(stdout, code, nil)`; deadline/start failure → `("", 0, err)`.

### Test Strategy
- Test that a hung command (e.g., `sleep 30`) is killed within ~3s+epsilon and returns an error.
- Verify the error type is `context.DeadlineExceeded` (or the `cerr` from `ctx.Err()`).

### Files to Modify
- `internal/cmd/upgrade_run.go` — `cmdRunner.Run` (add timeout + WaitDelay)
- Possibly `internal/upgrade/detect.go` — export `osRunner` or `defaultQueryTimeout`

---

## BUG-007 (MINOR): Linuxbrew Cellar root missing from pathHeuristics

### Root Cause
`internal/upgrade/detect.go:368` pathHeuristics table:
```go
var pathHeuristics = []pathHeuristic{
    {prefix: "/opt/homebrew/Cellar/", channel: ChannelBrew},       // Apple Silicon
    {prefix: "/usr/local/Cellar/", channel: ChannelBrew},           // Intel macOS
    {prefix: `\scoop\shims\`, channel: ChannelScoop},
    {prefix: "/nix/store/", channel: ChannelNix},
    {prefix: "ProgramData/chocolatey/", channel: ChannelChocolatey},
}
```
Missing: `/home/linuxbrew/.linuxbrew/Cellar/` (Linuxbrew on Linux).

### Fix
Add: `{prefix: "/home/linuxbrew/.linuxbrew/Cellar/", channel: ChannelBrew}`

### Files to Modify
- `internal/upgrade/detect.go` — `pathHeuristics` table

---

## BUG-008 (MINOR): c.Repo unescaped in releases.go URLs

### Root Cause
`internal/upgrade/releases.go`:
- Line ~179: `c.do(ctx, "/repos/"+c.Repo+"/releases/latest")` — c.Repo UNESCAPED
- Line ~223: `c.do(ctx, "/repos/"+c.Repo+"/releases")` — c.Repo UNESCAPED
- Line ~198: `path := "/repos/" + c.Repo + "/releases/tags/" + url.PathEscape(tag)` — tag IS escaped, but c.Repo is NOT

### Fix
Use `url.PathEscape(c.Repo)` in all three locations. Since `c.Repo` is "owner/repo", the slash must be preserved. Use `url.PathEscape` on each segment, or since GitHub repos are well-formed (no special chars in practice), this is defense-in-depth.

Actually, `url.PathEscape` escapes `/` to `%2F`, which would break "owner/repo". The correct approach is to escape each segment separately: `url.PathEscape(owner) + "/" + url.PathEscape(repo)`. Or since `c.Repo` is validated and normally just "owner/repo", the safest minimal fix is to keep the string as-is but document that Repo is trusted input. However, for consistency with the tag escaping and defense-in-depth, we should validate/escape appropriately.

**Simplest correct fix**: Keep the raw c.Repo for the path segment (GitHub API expects `owner/repo` in the path, and `url.PathEscape` would break the `/`), but ensure the Repo is validated at construction time (Client constructor).

### Files to Modify
- `internal/upgrade/releases.go` — validate c.Repo or document trust

---

## BUG-009 (MINOR): TOCTOU window in reapStaleLocks

### Root Cause
`internal/lock/lock.go:128` — `reapStaleLocks` checks `processAlive(pid, hostname)`, then `os.Remove(f)`. A contender could read the pid, find it dead, and reap a file whose inode is now held by a different live process that reused the pid.

Mitigated by: the CAS-backed acquire (flock + write+rename) and the hostname check. Not exploitable in practice.

### Fix Strategy
Defense-in-depth: After reaping, verify the file we removed is the one we checked (by reading contents before remove, then re-checking). Or accept the current mitigation as sufficient (the PRD says "bounded by the CAS-backed acquire"). The recommendation is to acknowledge/document the TOCTOU as acceptable given the flock/CAS mitigation.

### Files to Modify
- `internal/lock/lock.go` — `reapStaleLocks` (optional defense-in-depth improvement or documentation)

---

## BUG-010 (MINOR): Windows processAlive always returns true

### Root Cause
`internal/lock/lock_windows.go:19` — `processAlive(pid, hostname) bool` unconditionally returns `true`. So stale lock files are never reaped on Windows.

Per FR-K7: "flock is a no-op on Windows (the CAS is the guarantee)." Lock files are not created on the Windows commit path, so this is effectively non-functional rather than harmful.

### Fix Strategy
This is documented behavior (FR-K7). The fix is to improve the Windows `processAlive` to use the Windows API (e.g., `OpenProcess` + `WaitForSingleObject`) if we want reaping to work, OR to document this as accepted behavior. Given the CAS guarantee, this is low priority.

### Files to Modify
- `internal/lock/lock_windows.go` — optionally implement real processAlive using Windows API

---

## BUG-011 (MINOR): Orphan hint uses ppid==1, under-reports under subreapers

### Root Cause
`internal/lock/orphan_unix.go:42` — `appearsOrphaned(pid) bool` returns `ppid == 1`. Under subreapers (systemd, Docker), an orphan's ppid is the subreaper pid, not 1. So the hint reports "not orphaned" for genuinely orphaned holders.

NOTE: The ACTUAL watchdog (`internal/watchdog/arm_unix.go:40`) correctly uses parent-pid CHANGE detection (`osGetppid() != originalPpid`), so no live lock is ever force-broken. This is cosmetic/informational ONLY.

### Fix Strategy
The simplest improvement: also check if the ppid matches common subreaper patterns (systemd-run, containerd, etc.), or check if the ppid process name indicates a subreaper. However, this is complex and the current conservative approach (never false-positive) is correct per the design. The fix might just be to document the limitation more prominently, or to add a note in the status output ("orphan detection may miss subreaper-reparented processes").

### Files to Modify
- `internal/lock/orphan_unix.go` — `appearsOrphaned` (documentation or heuristic improvement)

---

## BUG-012 (MINOR): Stale doc comment about arbiter staging

### Root Cause
`internal/decompose/decompose.go:~981` — Comment says:
"The arbiter's STAGING (resolveArbiter via AddAll/Add) is UNCHANGED — it stages from the working tree"

But FR-M1d extended the freeze boundary into the arbiter: gate, diff, AND staging all derive from the frozen T_start via OverlayTreePaths, never the live working tree. The code is correct (`resolveArbiter` uses `tStart`, not `AddAll`); only the comment is stale.

### Fix
Update the comment to reflect that the arbiter stages from the frozen T_start (via OverlayTreePaths), not the working tree.

### Files to Modify
- `internal/decompose/decompose.go` — comment at ~line 981