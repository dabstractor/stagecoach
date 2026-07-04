# E2E Test Strategy — Decompose Contention Regression Scenario (Issue 1)

## Build & run

All e2e files carry `//go:build e2e` (line 1). Run the whole package:
`go test -tags e2e ./internal/e2e/...`. Default `go test ./...` skips it. Binaries are
built once per process via `sync.Once` (`buildStagehand`, `buildStub`).

## Harness primitives to reuse (`internal/e2e/harness_test.go`)

| Helper | Purpose |
|--------|---------|
| `buildStagehand(t)` | Builds `cmd/stagehand` → cached bin path. |
| `buildStub(t)` | Builds `cmd/stubagent` → cached bin path. |
| `newRepo(t)` | `t.TempDir()` + `git init -q` + local user.name/email. |
| `seedCommit(t, repo, name, body)` | Write file + `git add` + `git commit -m "seed: <name>"`. |
| `writeFile(t, repo, name, body)` | Write a working-tree file (does NOT stage). |
| `stageFile(t, repo, name)` | `git add <name>`. |
| `writeStubConfig(t, stubBin, extras)` | Writes base TOML (`config_version=3`, `[provider.stub]`, output=raw, strip_code_fence, default_model=stub, tooled_flags). `extras` appended for extra sections. Returns path. |
| `stubEnv(map)` | `os.Environ()` + the given `STAGEHAND_STUB_*` knobs as `K=V`. |
| `runStagehand(t, bin, repo, cfg, env, args...)` | Runs the compiled binary (cwd=repo, 60s timeout, `--config cfg --no-color` + args). Returns `e2eResult{Stdout, Stderr, ExitCode}`. |
| `waitForMarker(t, path, timeout)` | Polls a marker file's existence at 20ms cadence. The deterministic gate for two-process coordination. |
| `commitCount(t, repo)` / `headSHA(t, repo)` / `statusPorcelain(t, repo)` | Git inspection helpers. |

## The canonical two-process contention skeleton (from scenario B)

```go
readiness := t.TempDir() + "/ready.marker"
holderEnv := stubEnv(map[string]string{
    "STAGEHAND_STUB_OUT":      "feat: a",
    "STAGEHAND_STUB_MARKER":   readiness,
    "STAGEHAND_STUB_SLEEP_MS": "3000",   // holder holds the lock ~3s
})
resCh := make(chan e2eResult, 1)
go func() { resCh <- runStagehand(t, bin, repo, cfg, holderEnv, "--provider", "stub") }()
waitForMarker(t, readiness, 10*time.Second) // holder drained stdin + published snapshot + sleeping
contenderEnv := stubEnv(map[string]string{"STAGEHAND_STUB_OUT": "feat: a"})
res2 := runStagehand(t, bin, repo, cfg, contenderEnv, "--provider", "stub")
// assert res2.ExitCode + res2.Stderr
res := <-resCh // drain holder; assert holder + commit count
```

The stub (`cmd/stubagent/main.go`) ordering contract: **drain stdin → write marker → sleep
(STAGEHAND_STUB_SLEEP_MS) → write stdout (STAGEHAND_STUB_OUT) → exit**. So `waitForMarker`
returning means the holder has consumed the prompt, published its snapshot, and is now
sleeping with the lock held.

## New scenario F: decompose accidental double-run → Busy(5)

### Why this is feasible with the stub (the key insight)

Decompose normally needs a tooled planner (scenarios S1/S5 use `skipIfNotReal` for full
multi-commit decompose). BUT the contention scenario only needs the holder to reach
`lock.SetSnapshot(tStart)` at `decompose.go:169` and then SLEEP — it does NOT need to
complete the decompose. Two facts make this reachable with the stub:

1. **The FR-M2b one-file shortcut bypasses the planner entirely.** With EXACTLY one
   untracked file and auto mode (`Commits=0`, the default), `Decompose` takes
   `runOneFileShortcut` (`decompose.go:187`) which uses ONLY the MESSAGE role (the same agent
   the single-commit path uses). The stub's single-response `STAGEHAND_STUB_OUT` satisfies
   the message role. So the stub IS sufficient for a one-file decompose holder.

2. **`SetSnapshot(tStart)` at line 169 runs BEFORE the message-role agent call.** The order
   in `Decompose` is: `baseTree` (git ops) → `FreezeWorkingTree` (line 165) →
   `SetSnapshot(tStart)` (line 169) → one-file check (line 178-187) → `runOneFileShortcut`
   (message role → stub invoked → marker written → sleeps). So when `waitForMarker` returns,
   the snapshot `T_start` is ALREADY published in the lock file. The marker is written by the
   stub DURING the message-generation call, which is AFTER `SetSnapshot`.

### Decompose config activation (default config already enables it)

The base `writeStubConfig` TOML does NOT set `auto_stage_all`/`commits`/`single`, but the
config defaults (`internal/config/config.go:163-167`) are `AutoStageAll: true`,
`Commits: 0` (auto), `Single: false`. So `shouldDecompose(cfg, false, false)` returns true
with the BASE stub config when nothing is staged. **No `extras` needed for activation** —
but verify `ResolveRoles` succeeds with only `[provider.stub]` (it should: all roles inherit
the global default provider; the planner role is resolved but never invoked on the one-file
shortcut).

### Scenario F skeleton

```go
t.Run("F_DecomposeAccidentalDoubleRun_Busy", func(t *testing.T) {
    repo := newRepo(t)
    seedCommit(t, repo, "readme.md", "init")
    writeFile(t, repo, "feature.txt", "new work\n") // ONE untracked file, NOT staged → decompose activates

    readiness := t.TempDir() + "/ready.marker"
    holderEnv := stubEnv(map[string]string{
        "STAGEHAND_STUB_OUT":      "feat: add feature",
        "STAGEHAND_STUB_MARKER":   readiness,
        "STAGEHAND_STUB_SLEEP_MS": "4000",
    })
    // base config: defaults enable decompose (auto_stage_all=true, commits=0, single=false)
    cfg := writeStubConfig(t, stub, "")

    resCh := make(chan e2eResult, 1)
    go func() { resCh <- runStagehand(t, bin, repo, cfg, holderEnv, "--provider", "stub") }()
    waitForMarker(t, readiness, 10*time.Second) // holder: FreezeWorkingTree → SetSnapshot(tStart) → message-gen sleep

    // contender: same dirty tree, still nothing staged → handleLockContention:
    //   WriteTree() returns baseTree (index reset to baseTree by holder); snap = tStart ≠ baseTree → Busy(5)
    contenderEnv := stubEnv(map[string]string{"STAGEHAND_STUB_OUT": "feat: add feature"})
    res2 := runStagehand(t, bin, repo, cfg, contenderEnv, "--provider", "stub")

    if res2.ExitCode != 5 {
        t.Fatalf("contender exit = %d, want 5 (Busy) — decompose no-op fast path is structurally impossible; stderr:\n%s", res2.ExitCode, res2.Stderr)
    }
    if !strings.Contains(res2.Stderr, "already in progress") {
        t.Errorf("stderr missing busy message; got:\n%s", res2.Stderr)
    }
    // contender must NOT have exited 0 (the no-op fast path must not fire on the decompose path)
    if strings.Contains(res2.Stderr, "nothing to do") {
        t.Errorf("decompose path must NOT hit the no-op fast path; got:\n%s", res2.Stderr)
    }

    res := <-resCh // drain holder
    if res.ExitCode != 0 {
        t.Fatalf("holder exit = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
    }
})
```

### Assertion robustness

The assertion `ExitCode == 5` holds **regardless of snapshot timing**:
- If snapshot is published (`T_start`) → `WriteTree()` returns `baseTree` ≠ `T_start` → Busy.
- If snapshot were empty (race) → `snap == ""` skips the no-op arm → falls through to Busy.
Either way → Busy(5). This makes the scenario robust to the microsecond timing of
`SetSnapshot` vs the stub marker, while still pinning the *documented* behavior (decompose
double-run = Busy, NOT 0).

### Risks / verification the implementing agent must do
1. Confirm the base stub config makes `shouldDecompose` true (defaults: AutoStageAll=true,
   Commits=0, Single=false). If the loaded config does NOT default AutoStageAll to true when
   the TOML omits it, add `auto_stage_all = true` to the config extras.
2. Confirm `runOneFileShortcut` is reached with exactly one untracked file (it should: one
   changed path between baseTree and tStart via `DiffTreeNames`).
3. Confirm the holder's `FreezeWorkingTree` resets the shared `.git/index` to `baseTree`
   BEFORE the stub sleeps (so the contender's `WriteTree` returns `baseTree`). This is
   guaranteed by the marker-after-SetSnapshot ordering, but verify if flaky.
4. If the holder errors before sleeping (e.g. `ResolveRoles` fails with stub-only config),
   the holder exits non-zero and the contention window never opens → the goroutine result
   will show the error. Adjust the config (extras for role sections) if needed — see
   `internal/e2e/scenarios_test.go` S2 for the `[role.planner]` extras pattern.

## Test gaps this scenario closes
The e2e suite currently covers the no-op fast path ONLY on the single-commit (staged) path
(scenario B). There is NO e2e scenario for the decompose contention path — this is exactly
where Issue 1 hid. Scenario F pins the documented behavior so it cannot regress silently.
