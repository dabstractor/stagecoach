# Research Findings — P1.M1.T1.S2 (E2E integration tests for *bool false propagation)

## 1. Dependency on P1.M1.T1.S1 (in-flight, treated as CONTRACT)

S1 converts `AutoStageAll` and `MultiTurnFallback` to `*bool` and adds accessors. Verified the
accessors ALREADY EXIST in the working tree (S1 underway):
- `config.go:241` `func (c Config) AutoStageAllValue() bool`
- `config.go:253` `func (c Config) MultiTurnFallbackValue() bool`
- (precedent: `config.go:228` `DiffContextValue()`)

This PRP's tests consume S1's outputs:
- `cfg.AutoStageAllValue()` (bool) — used by e2e behavioral assertions indirectly via exit codes.
- `cfg.MultiTurnFallbackValue()` (bool) — asserted directly in the (c) in-process test.

## 2. e2e harness (`internal/e2e/harness_test.go`, `//go:build e2e`) — the subsystem for (a)/(b)/(d)

Helpers available (all t.Helper, throwaway repos via t.TempDir):
- `buildStagecoach(t)` — compiles `github.com/dustin/stagecoach/cmd/stagecoach` ONCE (cached).
- `buildStub(t)` → `stubtest.Build(t)` — compiles `cmd/stubagent` ONCE.
- `newRepo(t)` — `git init -q` + user.name/email in t.TempDir; returns repo dir.
- `seedCommit(t, repo, name, body)` — write+add+commit.
- `writeFile(t, repo, name, body)` — write a file, UN-STAGED (this is the dirty-tree lever).
- `writeStubConfig(t, stubBin, extras string) string` — writes a TOML with base `[provider.stub]`
  (stdin delivery, raw output, strip_code_fence) + `extras` appended. **extras is how we inject
  `[defaults] auto_stage_all = false` and `[generation] multi_turn_fallback = false`.** TOML sections
  may appear in any order, so extras can add `[defaults]`/`[generation]` after the provider block.
- `runStagecoach(t, bin, repo, cfg, env, args...) e2eResult` — subprocess; cmd.Dir=repo;
  prepends `--config cfg --no-color`; returns `{Stdout, Stderr, ExitCode}`. 60s ctx timeout.
- `commitCount(t, repo)`, `headSHA(t, repo)`, `diffTreeNames(t, repo, sha)`, `statusPorcelain(t, repo)`.
- `stubEnv(knobs)` — os.Environ() + STAGECOACH_STUB_* knobs.

**Stub knobs** (cmd/stubagent, env-driven): `STAGECOACH_STUB_OUT` (canned stdout), `STAGECOACH_STUB_EXIT`,
`STAGECOACH_STUB_SLEEP_MS`, `STAGECOACH_STUB_MARKER`, `STAGECOACH_STUB_STDERR`, `STAGECOACH_STUB_SCRIPT`
(call-varying, line-indexed) + `STAGECOACH_STUB_COUNTER`.

**CRITICAL — stub self-hosting guard** (load.go ~196-209): AMBIENT stub selection (via
`$STAGECOACH_PROVIDER` env or `stagecoach.provider` git key) is REJECTED. EXPLICIT `--provider stub`
on the CLI is ALLOWED. → Every e2e test MUST pass `--provider stub` as a CLI arg (matches S2/S3/S4).

**Existing positive-control precedent**: `S2_OneFile_NoPlannerCall` (scenarios_test.go) writes ONE
un-staged file, stub OUT="feat: ...", `--provider stub` → exit 0, commitCount==2. This is exactly the
"positive control" shape for (b). The negative case (a) is the inverse assertion (exit 2, no commit).

## 3. The exit-2 path under test (internal/cmd/default_action.go)

When nothing is staged:
- `--no-auto-stage` + not work-desc → exit 2 "Nothing staged."
- `cfg.AutoStageAllValue() || forceAutoStage` → `git add -A`, then if still 0 staged → exit 2
  "Nothing to commit."; else proceed to commit (exit 0).
- **`default` branch (AutoStageAllValue()==false, no --no-auto-stage) → exit 2 "Nothing to commit."**

So (a): dirty tree (un-staged b.txt) + `[defaults] auto_stage_all = false` + `--provider stub` ⇒
the default branch ⇒ **exit 2, no new commit, working tree STILL dirty** (`statusPorcelain` shows b.txt).
(b): same but true/no-setting ⇒ auto-stage ⇒ **exit 0, +1 commit, working tree clean**.

## 4. git-config layer (Layer 4) for the precedence test (d) — T2 (env) NOT landed

`internal/config/git.go` reads `stagecoach.autoStageAll` (camelCase — git forbids underscores in the
final segment; Issue 2 is a DOCS-only fix in T3). loadGitConfig(opts.RepoDir) runs in the subprocess
because cmd.Dir=repo and stagecoach auto-detects the repo. So:
- `runGit(t, repo, "config", "stagecoach.autoStageAll", "true")` is read by the subprocess as Layer 4.
- (d): global TOML (via `--config`) sets `auto_stage_all=false` (Layer 2); repo git-config sets
  `stagecoach.autoStageAll=true` (Layer 4) ⇒ Layer 4 wins ⇒ exit 0 + commit. Proves overlay propagates
  BOTH a Layer-2 false AND a higher-layer true override.
- Bonus standalone: repo git-config `stagecoach.autoStageAll=false` alone (Layer 4, no TOML setting) ⇒
  exit 2. Proves the git layer propagates false end-to-end (the working key once Issue 1's *bool overlay
  is in). Mirrors git_test.go:177-185 (which already asserts loadGitConfig sets *false).

**Env-var override (STAGECOACH_AUTO_STAGE_ALL) is sibling T2 (P1.M1.T2.S1) — NOT landed. The task
explicitly says use the git-config layer to test override when T2 hasn't landed.** Do NOT add the env
case here. (A guarded t.Run that skips if the env var is unsupported could be added later by T2's tests.)

## 5. multi_turn_fallback (c) — IN-PROCESS generate integration test is the right tool (NOT e2e)

The FR-T1 trigger gate is INLINE in generate.go:394:
`if cfg.MultiTurnFallbackValue() && !workDescActive && resolved.SessionMode != nil && *resolved.SessionMode == "append"`.

Why NOT e2e for (c): when one-shot exhausts (unparseable output) on a large diff + session_mode=append,
BOTH multi_turn_fallback=true (if chunked calls also fail) AND =false yield exit 3 (Rescue). Exit code
alone cannot distinguish. The robust distinguisher is the **verbose trigger line "multi-turn fallback"**
in the captured buffer — observable deterministically in-process, fragile via subprocess stderr.

**Proven in-process precedent**: `internal/generate/multiturn_test.go` `TestMultiTurnTriggerGate_TruthTable`
already has row `skip_cond_c_multiturn_off` (`MultiTurnFallback=false`) asserting trigger ABSENT +
`*RescueError{ErrRescue}`. BUT it sets `cfg.MultiTurnFallback = tc.multiTurn` DIRECTLY (a Go assignment).
S1 Task 7 will convert that to `boolPtr(tc.multiTurn)`. So the existing test proves the CONSUMER respects
a directly-set *bool false — it does NOT prove a TOML FILE propagates false to the consumer.

**S2's (c) value-add**: source `cfg` from a real TOML file via the PUBLIC `config.Load` API (exercises
loadTOML→materialize→overlay→accessor together), assert `cfg.MultiTurnFallbackValue()==false`, then hand
that SAME cfg to `CommitStaged` and assert the gate short-circuits (trigger absent + rescue). End-to-end
TOML→consumer proof. Distinct from S1's white-box materialize/overlay unit test (package config).

config.Load API (load.go:70): `Load(ctx, LoadOpts{ConfigPathOverride, RepoDir, Flags, DisableBootstrap})`.
For the test: ConfigPathOverride=<toml file>, RepoDir=<repo>, DisableBootstrap=true, clean CWD.

Helper already used by config tests: `writeTempTOML(t, body)` (file_test.go:14). Minimal body:
`config_version = 3\n[generation]\nmulti_turn_fallback = false\n`.

In-process setup mirrors skip_cond_c: `stubAppendManifest(t, bin, []string{""}, false)` (append,
unparseable one-shot), `cfg.MultiTurnChunkTokens=4` (small ⇒ ~24-token diff exceeds one chunk ⇒ cond b
true), `cfg.MaxDuplicateRetries=0`, a `strings.Repeat("change line\n", 8)` staged file. With false the
gate short-circuits despite (a)(b)(d) all true → rescue. Compare: a TRUE control row fires the trigger.

## 6. How tests RUN (validation gates)

- e2e tests carry `//go:build e2e`; `make test` = `go test -race ./...` does NOT include them (Makefile
  line 45: "intentionally excluded. Runnable locally and in CI."). Gate: `go test -tags e2e ./internal/e2e/...`.
- In-process generate tests: `go test ./internal/generate/...` (covered by `make test`).
- Whole suite incl. race: `make test`.

## 7. Scope boundaries (do NOT do)
- Do NOT add `STAGECOACH_AUTO_STAGE_ALL`/`STAGECOACH_MULTI_TURN_FALLBACK` env cases (T2 / P1.M1.T2.S1).
- Do NOT fix docs git-config snake_case spelling (T3 / P1.M1.T3.S1).
- Do NOT modify config struct/accessors/consumers (S1 owns; tests only consume them).
- Do NOT duplicate S1's `TestMaterializeOverlay_AutoStageAll_MultiTurnFallback` (file_test.go). (c) is a
  DIFFERENT layer (generate-package black-box via config.Load + CommitStaged).
