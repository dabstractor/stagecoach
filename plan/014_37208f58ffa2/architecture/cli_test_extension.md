# CLI Structure + E2E Test Architecture — FR-K4 + §20.5

## Subcommand registration pattern

Every subcommand group in stagecoach follows the same pattern: a cobra command with leaf
subcommands, registered via `init()` on `rootCmd`, with NO edit to root.go.

### Template: internal/cmd/hook.go (best match for `lock status`)

```go
// hook.go:38-58
var hookCmd = &cobra.Command{
    Use:               "hook",
    Short:             "Manage the per-repo prepare-commit-msg hook",
    SilenceErrors:     true,
    SilenceUsage:      true,
    PersistentPreRunE: func(*cobra.Command, []string) error { return nil }, // OVERRIDES root's config.Load
}

var hookStatusCmd = &cobra.Command{
    Use:           "status",
    Short:         "Report the prepare-commit-msg hook state (none|stagecoach (v1)|foreign)",
    Args:          cobra.NoArgs,
    SilenceErrors: true,
    SilenceUsage:  true,
    RunE:          runHookStatus,
}

// hook.go:90-103
func init() {
    hookCmd.AddCommand(hookInstallCmd, hookUninstallCmd, hookStatusCmd)
    rootCmd.AddCommand(hookCmd)
}
```

### New file: internal/cmd/lock.go

```go
var lockCmd = &cobra.Command{
    Use:               "lock",
    Short:             "Inspect the per-repo run lock (FR52/§9.27)",
    SilenceErrors:     true,
    SilenceUsage:      true,
    PersistentPreRunE: func(*cobra.Command, []string) error { return nil }, // diagnostic: skip config.Load
}

var lockStatusCmd = &cobra.Command{
    Use:           "status",
    Short:         "Print the run lock holder's pid/host/repo/liveness/orphan-status",
    Args:          cobra.NoArgs,
    SilenceErrors: true,
    SilenceUsage:  true,
    RunE:          runLockStatus,
}

func init() {
    lockCmd.AddCommand(lockStatusCmd)
    rootCmd.AddCommand(lockCmd)
}
```

### runLockStatus implementation shape

```go
func runLockStatus(cmd *cobra.Command, _ []string) error {
    repoDir, err := os.Getwd()
    if err != nil { return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err)) }

    path, contents, alive, orphan, err := lock.Status(repoDir)
    if err != nil { return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach lock status: %w", err)) }

    out := cmd.OutOrStdout()
    if path == "" {
        fmt.Fprintf(out, "no run lock for %s\n", repoDir)
        return nil
    }

    fmt.Fprintf(out, "Lock: %s\n", path)
    fmt.Fprintf(out, "  pid:       %s\n", contents.Pid)
    fmt.Fprintf(out, "  hostname:  %s\n", contents.Hostname)
    fmt.Fprintf(out, "  repo:      %s\n", contents.Repo)
    fmt.Fprintf(out, "  timestamp: %s\n", contents.Timestamp)
    if contents.Snapshot != "" {
        fmt.Fprintf(out, "  snapshot:  %s\n", contents.Snapshot)
    }
    fmt.Fprintf(out, "  alive:     %v\n", alive)
    if orphan {
        fmt.Fprintf(out, "  orphaned:  true (holder reparented — launcher has exited)\n")
    } else if alive {
        fmt.Fprintf(out, "  orphaned:  false\n")
    } else {
        fmt.Fprintf(out, "  orphaned:  unknown (holder is dead)\n")
    }
    return nil
}
```

### shouldSkipConfigLoad consideration

The `lock` group uses a no-op `PersistentPreRunE` (matching `hook`/`integrate`), which overrides
root's config.Load. This means `lock status` works outside a git repo and does NOT trigger the
first-run bootstrap write (FR-B3). NO edit to root.go's `shouldSkipConfigLoad` is needed.

## Exit codes (internal/exitcode/exitcode.go)

```go
const (
    Success         = 0
    Error           = 1
    NothingToCommit = 2
    Rescue          = 3
    Busy            = 5
    Timeout         = 124
)
```

`lock status` returns `nil` (→ exit 0) on success (even "no lock held"), `exitcode.Error` on
failure (path resolution error). Never `os.Exit` — only `main.go` does that.

## SIGHUP exit code mapping

On the pre-snapshot path, SIGHUP should exit 129 (128+1). This is added to
`signal_unix.go`'s `exitCodeForSignal`:
```go
case syscall.SIGHUP: return 129
```
Post-snapshot SIGHUP routes through `handle()` which always exits 3 (rescue).

## E2E test scenarios (§20.5)

The existing E2E harness lives in `internal/e2e/`:
- `harness_test.go` — the shared harness (temp git repo, stub agent, etc.)
- `lock_scenarios_test.go` — existing lock contention tests
- `scenarios_test.go` — decompose + single-commit scenarios
- `hook_scenarios_test.go` — hook mode scenarios

### New scenarios required (§20.5 v2.7)

1. **Parent-death watchdog self-exit**: launch stagecoach from a short-lived parent that exits
   mid-generation (e.g. `sh -c 'stagecoach' &` whose shell returns). Assert:
   - The holder self-exits via the parent-death watchdog
   - The lock file is removed
   - HEAD + index are unchanged (no commit landed)

2. **SIGHUP rescue path**: drive a launcher that delivers SIGHUP on close. Assert:
   - The rescue path fires (exit 3 if snapshot armed, or 129 if pre-snapshot)
   - The lock file is removed

3. **lock status diagnostics**: assert `stagecoach lock status` reports correctly for:
   - A live holder (alive=true, orphaned=false)
   - A dead holder (alive=false)
   - A reparented/orphaned holder (alive=true, orphaned=true)
   - No lock held ("no run lock for <repo>")

4. **no_parent_watchdog opt-out**: assert the opt-out (`STAGECOACH_NO_PARENT_WATCHDOG=1` or
   `stagecoach.noParentWatchdog=true`) suppresses the watchdog without affecting SIGHUP or lock status.

### Test implementation considerations

- The parent-death test is the hardest to E2E: it requires a real parent process that exits while
  stagecoach is generating. The polling approach (getppid change) makes this testable: spawn
  stagecoach from a parent, kill the parent, wait for stagecoach to detect and self-exit.
- The getppid interval is 1s; tests must allow ~2s for detection + exit.
- SIGHUP test: send `syscall.Kill(stagecoachPid, syscall.SIGHUP)` mid-generation.
- Lock status test: write a lock file with known contents, then run the subcommand.
