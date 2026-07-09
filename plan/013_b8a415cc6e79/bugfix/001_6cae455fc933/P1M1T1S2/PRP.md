name: "P1.M1.T1.S2 — End-to-end integration tests: TOML/git-config false disables auto-stage & multi-turn fallback"
description: >
  Add the end-to-end behavioral proof that Issue 1's *bool fix (from P1.M1.T1.S1) actually changes
  stagecoach's OBSERVABLE behavior through the full config-precedence chain: a TOML `[defaults]
  auto_stage_all = false` makes the compiled binary exit 2 "Nothing to commit." instead of silently
  auto-staging (the PRD h3.0 reproduction), a `git config stagecoach.autoStageAll false` likewise,
  a higher-layer override re-enables auto-stage, and a `[generation] multi_turn_fallback = false`
  reaches the generate-package FR-T1 trigger gate so multi-turn does NOT activate on a large diff.
  Pure tests — no production/config/doc changes.

---

## Goal

**Feature Goal**: Prove, via throwaway-repo integration tests, that `auto_stage_all = false` and
`multi_turn_fallback = false` propagate **end-to-end** from a config source (TOML file / git-config)
through the resolved `Config` to **consumer behavior**, once P1.M1.T1.S1's `*bool` overlay lands. The
tests must reproduce PRD Issue 1 (h3.0) exactly: dirty working tree + `[defaults] auto_stage_all =
false` ⇒ exit 2 "Nothing to commit.", no commit, tree still dirty.

**Deliverable**: Two new test files (no production code changes):
1. `internal/e2e/config_precedence_test.go` (`//go:build e2e`) — subprocess tests for `auto_stage_all`:
   (a) TOML false ⇒ exit 2/no-commit/dirty; (b) positive control ⇒ exit 0/commit/clean; (d) layer
   precedence (global TOML false overridden by repo git-config true ⇒ exit 0) + git-config-false-standalone ⇒ exit 2.
2. An in-process integration test in `internal/generate/` for `multi_turn_fallback` (c): source `cfg`
   from a TOML file via `config.Load`, assert `MultiTurnFallbackValue() == false`, then drive
   `CommitStaged` and assert the FR-T1 multi-turn gate does NOT fire (trigger absent) + rescue.

**Success Definition**:
- `go test -tags e2e ./internal/e2e/...` passes, including the new `config_precedence` scenario(s).
- `go test ./internal/generate/...` passes, including the new multi-turn-false-from-file test.
- `make test` (`go test -race ./...`) stays green (the e2e file is build-tagged out of the default run).
- Each test asserts the BEHAVIORAL outcome (exit code / commit count / working-tree state / verbose
  trigger), not just the accessor value — proving false reaches the consumer, not just the struct.
- Tests consume ONLY S1's contract (`AutoStageAllValue()`/`MultiTurnFallbackValue()` accessors, `*bool`
  fields); they do NOT modify config/production code or duplicate S1's white-box materialize/overlay unit test.

## User Persona (if applicable)

**Target User**: Stagecoach maintainers / CI (regression guard) — these tests lock Issue 1's fix so a
future "only-true-propagates" regression is caught before merge.

**Use Case**: A user writes `auto_stage_all = false` in `~/.config/stagecoach/config.toml` and expects
stagecoach to NOT auto-stage. The test encodes that exact expectation as an executable contract.

**User Journey**: `git init` → seed a commit → leave an un-staged `b.txt` → `stagecoach --config
<auto_stage_all=false>` → expect exit 2, `b.txt` still un-staged, no spurious commit.

**Pain Points Addressed**: The SILENT failure (Issue 1) — false ignored, opposite behavior, no warning.
These tests make the failure loud (test fails) if it ever regresses.

## Why

- **Issue 1 (Major, PRD h3.0)**: `auto_stage_all = false` was silently ignored; stagecoach committed
  anyway. S1 (P1.M1.T1.S1) fixes the `*bool` overlay mechanically. S2 proves the fix is REAL by
  observing behavior through the compiled binary + the public `config.Load` API — the unit tests in S1
  prove the merge math; these tests prove the user-visible outcome.
- **Bounded, complementary scope**: S1 = the `*bool` refactor (config struct + merge + accessors + unit
  test in package `config`). S2 = behavioral integration tests in packages `e2e` and `generate`. No
  overlap: S1's test is white-box materialize/overlay; S2's are black-box behavior via the binary /
  `config.Load` + `CommitStaged`.
- **Anchors the milestone**: P1.M1.T2 (env vars) and T3 (docs) build on the working disable path; S2 is
  the acceptance gate that the path works before those land.

## What

### Behavioral requirements (mirrors the item contract, items a–e)
- **(a)** Reproduce PRD h3.0: `git init`, seed a commit, dirty tree (un-staged file), config with
  `[defaults] auto_stage_all = false` + stub provider, run stagecoach ⇒ **exit 2, no new commit, working
  tree still dirty** (un-staged file remains in `statusPorcelain`).
- **(b)** Positive control: identical setup but `auto_stage_all = true` (or omitted) ⇒ **exit 0, +1
  commit, working tree clean**.
- **(c)** `multi_turn_fallback = false` in a TOML `[generation]` ⇒ loaded `cfg.MultiTurnFallbackValue()
  == false`, and the FR-T1 multi-turn fallback path does NOT activate on a large diff (the generate
  consumer respects false) ⇒ rescue, trigger line absent.
- **(d)** Layer precedence: `auto_stage_all = false` in the global TOML (passed via `--config`),
  overridden to **true** via repo `git config stagecoach.autoStageAll true` (Layer 4) ⇒ **true wins**
  ⇒ exit 0 + commit. (Env var `STAGECOACH_AUTO_STAGE_ALL` is sibling T2, NOT landed — use the
  git-config layer per the item instruction.)
- **(e)** Mocking: the stub provider (`cmd/stubagent`, deterministic output) + throwaway repos
  (`t.TempDir()` + `git init`). No real agent required.

### Success Criteria
- [ ] (a) `auto_stage_all = false` ⇒ exit 2, commitCount unchanged, `statusPorcelain` still shows the un-staged file.
- [ ] (b) `auto_stage_all = true`/omitted ⇒ exit 0, +1 commit, clean tree.
- [ ] (c) `multi_turn_fallback = false` from a TOML file ⇒ `MultiTurnFallbackValue()==false` AND the multi-turn trigger line is ABSENT in the verbose buffer AND `CommitStaged` returns `*RescueError{Kind:ErrRescue}`.
- [ ] (d) global TOML false + repo git-config true ⇒ exit 0 + commit (higher layer wins).
- [ ] (d-bonus) repo git-config `stagecoach.autoStageAll false` alone ⇒ exit 2 (git layer propagates false end-to-end).
- [ ] All new tests pass under `go test -tags e2e ./internal/e2e/...` and `go test ./internal/generate/...`.
- [ ] `make test` (race, default tag set) still green; `go vet` clean.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — exact harness helpers with signatures, the exact exit-code branch under test, the exact
multi-turn trigger-gate observation pattern (with a verbatim existing test to mirror), the TOML
section-injection lever (`writeStubConfig` extras), the stub self-hosting-guard gotcha, the `config.Load`
API for the in-process (c) test, and explicit scope boundaries vs. S1/T2/T3.

### Documentation & References

```yaml
# MUST READ — the e2e subsystem these tests extend
- file: internal/e2e/harness_test.go
  why: "Every helper these tests call is defined here. buildStagecoach/buildStub/newRepo/seedCommit/
        writeFile/writeStubConfig/runStagecoach/commitCount/statusPorcelain/headSHA/stubEnv."
  pattern: "runStagecoach(t, bin, repo, cfg, env, args...) e2eResult{Stdout,Stderr,ExitCode}; it
            prepends '--config cfg --no-color' and sets cmd.Dir=repo. writeStubConfig(t, stub, extras)
            writes base [provider.stub] + extras (append [defaults]/[generation] there)."
  critical: "ALL e2e tests pass '--provider stub' as a CLI arg — load.go's self-hosting guard REJECTS
             AMBIENT stub selection ($STAGECOACH_PROVIDER env / git key) but allows EXPLICIT --provider."

- file: internal/e2e/scenarios_test.go
  why: "S2_OneFile_NoPlannerCall is the EXACT positive-control shape for (b): one un-staged file,
        STAGECOACH_STUB_OUT='feat: ...', '--provider stub' → exit 0, commitCount==2."
  pattern: "writeFile(t, repo, 'solo.txt', 'solo\n') [UN-staged]; stubEnv(map{'STAGECOACH_STUB_OUT':...});
            runStagecoach(..., '--provider', 'stub'); assert res.ExitCode==0 + commitCount."

- file: internal/cmd/default_action.go
  why: "The exit-2 branch under test. Around line 121 the switch: --no-auto-stage→exit2; AutoStageAllValue()
        ||forceAutoStage→AddAll+commit; default(AutoStageAllValue()==false)→exit 2 'Nothing to commit.'."
  pattern: "After S1, the read is cfg.AutoStageAllValue() (was cfg.AutoStageAll)."
  critical: "(a) asserts the default branch (false). forceAutoStage is true only in work-description mode
             (cfg.WorkDescription!='') — our tests leave WorkDescription empty so it never forces."

- file: internal/generate/multiturn_test.go
  why: "TestMultiTurnTriggerGate_TruthTable is the test to MIRROR for (c). Its row 'skip_cond_c_multiturn_off'
        sets MultiTurnFallback=false and asserts trigger ABSENT + *RescueError{ErrRescue}."
  pattern: "stubAppendManifest(t, bin, script, omitAppend); cfg.MultiTurnChunkTokens=4; cfg.MaxDuplicateRetries=0;
            a strings.Repeat('change line\\n',8) staged file (~24 tokens > 4 ⇒ cond b true); observe
            strings.Contains(buf.String(), 'multi-turn fallback') for the trigger."
  critical: "That existing test sets cfg.MultiTurnFallback DIRECTLY (boolPtr after S1). S2's (c) must
             instead SOURCE cfg from a TOML file via config.Load to prove file→consumer end-to-end."

- file: internal/config/load.go
  why: "config.Load(ctx, LoadOpts{ConfigPathOverride, RepoDir, Flags, DisableBootstrap}) — the PUBLIC API
        for the in-process (c) test. Layer 2 = ConfigPathOverride file; Layer 4 = loadGitConfig(RepoDir)."
  pattern: "LoadOpts{ConfigPathOverride: tomlPath, RepoDir: repo, DisableBootstrap: true}."

- file: internal/config/file_test.go
  why: "writeTempTOML(t, body) helper (line 14) + S1's TestMaterializeOverlay_AutoStageAll_MultiTurnFallback.
        DO NOT duplicate the latter — it is the package-config white-box proof; S2's (c) is the
        package-generate black-box proof (different layer, complementary)."

- docfile: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/architecture/system_context.md
  why: "The 7-layer precedence model + the *bool fix rationale. §'Config Precedence Architecture'."
  section: "Config Precedence Architecture (the core of Issues 1–3)"

- docfile: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/P1M1T1S1/PRP.md
  why: "S1 is the CONTRACT these tests consume. Read its accessor signatures + scope boundaries."

- docfile: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/P1M1T1S2/research/findings.md
  why: "Verified line numbers, harness mechanics, the (c)-not-e2e rationale, and scope fences."
```

### Current Codebase tree (relevant slice)

```bash
internal/e2e/
  harness_test.go        # //go:build e2e — helpers (buildStagecoach, runStagecoach, writeStubConfig, ...)
  scenarios_test.go      # //go:build e2e — TestE2EScenarios (S2_OneFile_NoPlannerCall = positive-control shape)
  hook_scenarios_test.go # //go:build e2e
  lock_scenarios_test.go # //go:build e2e
internal/generate/
  multiturn_test.go      # TestMultiTurnTriggerGate_TruthTable (the (c) pattern to mirror); helpers initRepo/commitRaw/writeFile/stageFile/stubAppendManifest
  generate.go            # FR-T1 gate at ~line 394 (cfg.MultiTurnFallbackValue() after S1)
internal/config/
  load.go                # Load(ctx, LoadOpts) public API
  file_test.go           # writeTempTOML helper; S1's materialize/overlay unit test (do NOT duplicate)
cmd/stubagent/main.go    # the fake agent (STAGECOACH_STUB_* env knobs)
```

### Desired Codebase tree with files to be added

```bash
internal/e2e/config_precedence_test.go   # NEW, //go:build e2e — TestE2EConfigPrecedence (a/b/d + git-config-false)
internal/generate/multiturn_config_test.go # NEW — TestMultiTurnFallback_FalseFromFile (c): config.Load→CommitStaged
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (stub guard): e2e tests MUST pass "--provider stub" as an explicit CLI arg. load.go
//   (~line 196-209) rejects AMBIENT stub selection (STAGECOACH_PROVIDER env / stagecoach.provider git
//   key) to avoid a leaked env hijacking a real run. Explicit --provider stub is allowed.

// CRITICAL (exit code is the assertion, not the accessor): the e2e tests can't read the in-process
//   cfg; they observe behavior. (a) ⇒ exit 2 (exitcode.NothingToCommit) + commitCount UNCHANGED +
//   statusPorcelain still shows the un-staged file. (b) ⇒ exit 0 + commitCount+1 + clean tree.
//   Do NOT assert on stderr text for the auto_stage cases (stderr wording is cosmetic/Issue 7); assert
//   exit code + git state, which are contractual (§15.4).

// CRITICAL ((c) is in-process, NOT e2e): a one-shot-exhaust + large-diff + session_mode=append setup
//   yields exit 3 (Rescue) for BOTH multi_turn_fallback=true (if chunked calls also fail) AND =false.
//   Exit code cannot distinguish them. The deterministic distinguisher is the verbose trigger line
//   "multi-turn fallback" in the captured *bytes.Buffer — observable cleanly in-process (mirror
//   multiturn_test.go), fragile via subprocess stderr. So (c) lives in package generate.

// CRITICAL (don't duplicate S1): S1 adds TestMaterializeOverlay_AutoStageAll_MultiTurnFallback in
//   internal/config/file_test.go (white-box materialize/overlay). S2's (c) is a DIFFERENT layer:
//   package generate, black-box via config.Load + CommitStaged. Keep them separate; (c) MUST source
//   cfg from a TOML FILE (config.Load), not a direct struct assignment.

// CRITICAL (T2 not landed): there is NO STAGECOACH_AUTO_STAGE_ALL env case yet (sibling P1.M1.T2.S1).
//   For (d) use the git-config LAYER (git config stagecoach.autoStageAll true) to override the TOML
//   false — do NOT set/case on a STAGECOACH_AUTO_STAGE_ALL env var.

// CRITICAL (camelCase git key): the working git key is stagecoach.autoStageAll (camelCase; git forbids
//   underscores in the final segment — Issue 2). Use the camelCase key in runGit; never snake_case.

// CRITICAL (config.Load side effects): for the in-process (c) test, set DisableBootstrap:true (no
//   first-run auto-write), RepoDir:<repo>, and run with a clean CWD (no stray .stagecoach.toml). A
//   minimal TOML body `config_version = 3\n[generation]\nmulti_turn_fallback = false\n` loads cleanly.

// NOTE (build tag): e2e files need `//go:build e2e` as the FIRST line (blank line before package decl).
//   `make test` (go test -race ./...) EXCLUDES them; gate is `go test -tags e2e ./internal/e2e/...`.
```

## Implementation Blueprint

### Data models and structure
None. Pure test additions — no production types, no config changes. Tests consume S1's
`AutoStageAllValue()` / `MultiTurnFallbackValue()` accessors and the `*bool` fields.

### Implementation Tasks (ordered by dependencies)

> **Hard prerequisite**: P1.M1.T1.S1 must be merged (the `*bool` fields + accessors must exist). The
> accessors `AutoStageAllValue()`/`MultiTurnFallbackValue()` already exist in the tree (config.go:241/253);
> confirm `cfg.AutoStageAll`/`cfg.MultiTurnFallback` are `*bool` and the consumers read the accessors
> before writing these tests (otherwise (a) will wrongly exit 0 — the pre-fix bug).

```yaml
Task 1: CREATE internal/e2e/config_precedence_test.go  (//go:build e2e, package e2e)
  - FILE HEAD: `//go:build e2e` then blank line then `package e2e`.
  - IMPLEMENT a top-level test, e.g. `TestE2EConfigPrecedence_AutoStageAll`, with t.Run subtests for
    each case below. Reuse the package helpers (buildStagecoach, buildStub, newRepo, seedCommit,
    writeFile, writeStubConfig, runStagecoach, commitCount, statusPorcelain, runGit). No new helpers
    needed beyond a tiny local `defaultsExtras(setting string) string` that returns the TOML extras
    block, e.g. "\n[defaults]\nauto_stage_all = " + setting + "\n".
  - SHARED SETUP per subtest: bin := buildStagecoach(t); stub := buildStub(t).
  - DEPENDENCIES: S1 merged (accessors + *bool consumers).

Task 1a (case a — the headline reproduction, PRD h3.0):
  - repo := newRepo(t); seedCommit(t, repo, "readme.md", "init")
  - writeFile(t, repo, "b.txt", "b\n")           # UN-STAGED → dirty tree (do NOT stageFile)
  - cfg := writeStubConfig(t, stub, "\n[defaults]\nauto_stage_all = false\n")
  - env := stubEnv(map[string]string{"STAGECOACH_STUB_OUT": "feat: should not happen"})
  - before := commitCount(t, repo)
  - res := runStagecoach(t, bin, repo, cfg, env, "--provider", "stub")
  - ASSERT res.ExitCode == 2                       # NothingToCommit (§15.4)
  - ASSERT commitCount(t, repo) == before          # NO commit created
  - ASSERT strings.Contains(statusPorcelain(t, repo), "b.txt")  # tree STILL dirty (b.txt un-staged)
  - (Optional) ASSERT !strings.Contains(res.Stdout, "staging all") # the auto-stage notice must NOT print

Task 1b (case b — positive control):
  - repo := newRepo(t); seedCommit(t, repo, "readme.md", "init")
  - writeFile(t, repo, "b.txt", "b\n")            # un-staged
  - cfg := writeStubConfig(t, stub, "\n[defaults]\nauto_stage_all = true\n")  # or omit the block (default true)
  - env := stubEnv(map[string]string{"STAGECOACH_STUB_OUT": "feat: add b"})
  - before := commitCount(t, repo)
  - res := runStagecoach(t, bin, repo, cfg, env, "--provider", "stub")
  - ASSERT res.ExitCode == 0
  - ASSERT commitCount(t, repo) == before+1        # one new commit
  - ASSERT statusPorcelain(t, repo) == ""          # working tree CLEAN
  - (Optional sanity) diffTreeNames(t, repo, headSHA(t, repo)) contains "b.txt"
  - NOTE: also run an `auto_stage_all omitted` variant (extras="") to prove the DEFAULT-true path is
    unchanged — that is the strongest regression guard for "no behavior change for the default".

Task 1c (case d — layer precedence: global TOML false overridden by repo git-config true):
  - repo := newRepo(t); seedCommit(t, repo, "readme.md", "init")
  - writeFile(t, repo, "b.txt", "b\n")
  - cfg := writeStubConfig(t, stub, "\n[defaults]\nauto_stage_all = false\n")  # Layer 2 = false
  - runGit(t, repo, "config", "stagecoach.autoStageAll", "true")              # Layer 4 = true (camelCase!)
  - env := stubEnv(map[string]string{"STAGECOACH_STUB_OUT": "feat: add b"})
  - before := commitCount(t, repo)
  - res := runStagecoach(t, bin, repo, cfg, env, "--provider", "stub")
  - ASSERT res.ExitCode == 0                       # Layer 4 (true) wins over Layer 2 (false)
  - ASSERT commitCount(t, repo) == before+1        # auto-staged + committed

Task 1d (case d-bonus — git-config false STANDALONE ⇒ exit 2; proves the git layer propagates false):
  - repo := newRepo(t); seedCommit(t, repo, "readme.md", "init")
  - writeFile(t, repo, "b.txt", "b\n")
  - cfg := writeStubConfig(t, stub, "")            # no TOML setting (default true)
  - runGit(t, repo, "config", "stagecoach.autoStageAll", "false")             # Layer 4 = false
  - env := stubEnv(map[string]string{"STAGECOACH_STUB_OUT": "feat: x"})
  - before := commitCount(t, repo)
  - res := runStagecoach(t, bin, repo, cfg, env, "--provider", "stub")
  - ASSERT res.ExitCode == 2                       # git-config false propagates end-to-end
  - ASSERT commitCount(t, repo) == before
  - This pairs with git_test.go:177-185 (loadGitConfig sets *false) — the e2e proof of the same.

Task 2: CREATE internal/generate/multiturn_config_test.go  (package generate)  — case (c)
  - IMPLEMENT `TestMultiTurnFallback_FalseFromFile` that SOURCES cfg from a TOML file (end-to-end
    file→consumer), distinct from TestMultiTurnTriggerGate_TruthTable which sets the field directly.
  - STEPS:
      1. bin := stubtest.Build(t)
      2. repo := t.TempDir(); initRepo(t, repo); commitRaw(t, repo, "initial")
         writeFile(t, repo, "new.txt", strings.Repeat("change line\n", 8))  # ~24 tokens > chunk 4 ⇒ cond b true
         stageFile(t, repo, "new.txt")
      3. Write a minimal TOML: `config_version = 3\n[generation]\nmulti_turn_fallback = false\n`
         (use a t.TempDir file or inline os.WriteFile; internal/config.writeTempTOML is package config,
          so write the file inline here). Path := filepath.Join(t.TempDir(), "config.toml").
      4. ctx := context.Background()
         cfgPtr, err := config.Load(ctx, config.LoadOpts{
             ConfigPathOverride: tomlPath, RepoDir: repo, DisableBootstrap: true,
         })
         if err != nil { t.Fatalf(...) }
         cfg := *cfgPtr
      5. ASSERT cfg.MultiTurnFallbackValue() == false                 # TOML false reached the resolved Config
      6. ASSERT cfg.MultiTurnChunkTokens == 32000 (the default) is FINE — but to make cond (b) true we
         need the diff > chunk. With default 32000 the small diff does NOT exceed it. So OVERRIDE the
         loaded cfg's chunk size for the test (mirrors multiturn_test.go which sets it directly):
         cfg.MultiTurnChunkTokens = 4   # now ~24-token diff > 4 ⇒ cond (b) true; ONLY (c) is false
         cfg.MaxDuplicateRetries = 0    # exactly one one-shot attempt ⇒ exhaust ⇒ reach the gate
      7. m := stubAppendManifest(t, bin, []string{""}, false)         # SessionMode="append" (cond d), unparseable one-shot (cond a true)
      8. var buf bytes.Buffer
         _, err = CommitStaged(ctx, Deps{
             Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&buf, true),
         }, cfg)
      9. ASSERT !strings.Contains(buf.String(), "multi-turn fallback")  # gate short-circuited (false respected)
     10. ASSERT err is *RescueError{Kind: ErrRescue}  (errors.As)       # fell through to rescue, NOT multi-turn
  - ADD a paired CONTROL subtest (same setup but TOML `multi_turn_fallback = true`, OR omit the line so
    default true) asserting the trigger IS present (strings.Contains(buf, "multi-turn fallback")) — this
    proves the test setup is capable of firing multi-turn, so the false-case absence is meaningful (not a
    setup defect). This control is what makes (c) a trustworthy regression guard.
  - DEPENDENCIES: S1 merged (MultiTurnFallback *bool + MultiTurnFallbackValue()); the generate-package
    helpers initRepo/commitRaw/writeFile/stageFile/stubAppendManifest already exist (multiturn_test.go).
  - NOTE on config.Load env sensitivity: loadEnv reads STAGECOACH_*; the test sets STAGECOACH_STUB_* only
    via the Manifest Env (not os env), and no STAGECOACH_* config var is set, so Load is unaffected. If
    the test host leaks STAGECOACH_VERBOSE etc., that does not touch MultiTurnFallback. Keep it simple.

Task 3: VERIFY build + vet + the two test invocations
  - go build ./...                                   # everything compiles (test files included)
  - go vet ./internal/e2e/... ./internal/generate/...
  - go test -tags e2e ./internal/e2e/... -run 'ConfigPrecedence' -v
  - go test ./internal/generate/... -run 'MultiTurnFallback_FalseFromFile' -v
  - make test                                        # default tag set still green (e2e excluded by tag)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the (a) reproduction — minimal, mirrors S2_OneFile_NoPlannerCall inverted (scenarios_test.go)
func TestE2EConfigPrecedence_AutoStageAll(t *testing.T) {
	bin := buildStagecoach(t)
	stub := buildStub(t)

	t.Run("a_toml_false_exits2_no_commit", func(t *testing.T) {
		repo := newRepo(t)
		seedCommit(t, repo, "readme.md", "init")
		writeFile(t, repo, "b.txt", "b\n") // UN-staged → dirty tree
		cfg := writeStubConfig(t, stub, "\n[defaults]\nauto_stage_all = false\n")
		env := stubEnv(map[string]string{"STAGECOACH_STUB_OUT": "feat: should not happen"})
		before := commitCount(t, repo)
		res := runStagecoach(t, bin, repo, cfg, env, "--provider", "stub")
		if res.ExitCode != 2 {
			t.Fatalf("exit code = %d, want 2 (NothingToCommit); stderr:\n%s", res.ExitCode, res.Stderr)
		}
		if got := commitCount(t, repo); got != before {
			t.Errorf("commit count = %d, want %d (no commit must be created)", got, before)
		}
		if !strings.Contains(statusPorcelain(t, repo), "b.txt") {
			t.Errorf("working tree must still be dirty (b.txt un-staged); status:\n%s", statusPorcelain(t, repo))
		}
	})
	// ... t.Run("b_positive_control_true", ...), t.Run("d_toml_false_gitconfig_true_wins", ...),
	//     t.Run("d_gitconfig_false_standalone_exits2", ...)
}

// PATTERN: the (c) in-process test — TOML file → config.Load → CommitStaged (mirrors skip_cond_c_multiturn_off
// but sources cfg from a file). The CONTROL row (true) proves the setup CAN fire multi-turn.
func TestMultiTurnFallback_FalseFromFile(t *testing.T) {
	bin := stubtest.Build(t)
	cases := []struct{ name, tomlBody string; wantTrigger bool }{
		{"false_no_trigger", "config_version = 3\n[generation]\nmulti_turn_fallback = false\n", false},
		{"control_true_fires", "config_version = 3\n[generation]\nmulti_turn_fallback = true\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir(); initRepo(t, repo); commitRaw(t, repo, "initial")
			writeFile(t, repo, "new.txt", strings.Repeat("change line\n", 8)); stageFile(t, repo, "new.txt")
			tomlPath := filepath.Join(t.TempDir(), "config.toml")
			os.WriteFile(tomlPath, []byte(tc.tomlBody), 0o644)
			cfgPtr, err := config.Load(context.Background(), config.LoadOpts{
				ConfigPathOverride: tomlPath, RepoDir: repo, DisableBootstrap: true,
			})
			if err != nil { t.Fatalf("config.Load: %v", err) }
			cfg := *cfgPtr
			cfg.MultiTurnChunkTokens = 4   // cond (b) true for the ~24-token diff; ONLY (c) varies
			cfg.MaxDuplicateRetries = 0    // one one-shot attempt → exhaust → reach the FR-T1 gate
			m := stubAppendManifest(t, bin, []string{""}, false) // append + unparseable one-shot
			var buf bytes.Buffer
			_, err = CommitStaged(context.Background(), Deps{
				Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&buf, true),
			}, cfg)
			gotTrigger := strings.Contains(buf.String(), "multi-turn fallback")
			if gotTrigger != tc.wantTrigger {
				t.Errorf("trigger = %v, want %v; buf tail: %q", gotTrigger, tc.wantTrigger, tail(buf.String(), 200))
			}
		})
	}
}
```

### Integration Points

```yaml
NO production/config/doc changes. Tests only.

NEW TEST FILES:
  - internal/e2e/config_precedence_test.go   (//go:build e2e, package e2e)
  - internal/generate/multiturn_config_test.go (package generate)

CONSUMED CONTRACT (from S1, must be merged first):
  - config.Config.AutoStageAll   (*bool)        + AutoStageAllValue() bool
  - config.Config.MultiTurnFallback (*bool)     + MultiTurnFallbackValue() bool
  - consumers read the accessors: internal/cmd/default_action.go (~:121,:382),
    internal/generate/generate.go (~:394), internal/hook/exec.go (~:226), pkg/stagecoach/stagecoach.go (~:623)

TEST RUNNERS:
  - e2e:  go test -tags e2e ./internal/e2e/...     (excluded from `make test` by the build tag)
  - in-proc: go test ./internal/generate/...       (included in `make test`)
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build everything (test files compile too)
go build ./...
# Vet the two new test packages
go vet ./internal/e2e/... ./internal/generate/...
# Format check
gofmt -l internal/e2e/config_precedence_test.go internal/generate/multiturn_config_test.go
# Expected: no files listed. If listed, gofmt -w them.
```

### Level 2: Unit/Integration Tests (Component Validation)

```bash
# (c) the multi-turn-false-from-file test
go test ./internal/generate/... -run 'MultiTurnFallback_FalseFromFile' -v

# (a)/(b)/(d) the e2e config-precedence scenarios
go test -tags e2e ./internal/e2e/... -run 'ConfigPrecedence' -v

# Full affected packages
go test ./internal/generate/... -v
go test -tags e2e ./internal/e2e/... -v   # the whole e2e suite still green (no harness regressions)

# Race detector on the in-process test (e2e race run is optional but encouraged)
go test -race ./internal/generate/... -run 'MultiTurnFallback_FalseFromFile'
# Expected: ALL pass. The control row (true) MUST show the trigger; the false row MUST NOT.
```

### Level 3: Integration Testing (System Validation)

```bash
# Whole default-tag suite stays green (e2e excluded by build tag)
make test

# Manual end-to-end smoke of Issue 1's fix (mirrors PRD h3.0 reproduction) — independent of the test code:
make build
d=$(mktemp -d) && cd "$d" && git init -q && git config user.name T && git config user.email t@e && \
  printf 'init\n' > readme.md && git add readme.md && git commit -q -m seed && printf 'b\n' > b.txt && \
  printf 'config_version = 3\n[defaults]\nauto_stage_all = false\n[provider.stub]\ncommand = "/bin/true"\nprompt_delivery = "stdin"\noutput = "raw"\n' > cfg.toml && \
  /home/dustin/projects/stagecoach/bin/stagecoach --config cfg.toml --provider stub; echo "exit=$?"
# Expected: exit=2 "Nothing to commit."; b.txt still un-staged (git status shows ?? b.txt); no new commit.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Guard: prove the new tests actually EXERCISE the false path (not a no-op).
# Run the e2e (a) case against the PRE-S1 binary if available — it MUST fail (exit 0, commit created),
# confirming the test is a real regression guard for Issue 1. (If you can't build pre-S1, the control
# row b + the in-process control row together establish the same trust.)

# Confirm scope boundaries respected (should be empty):
grep -rn 'STAGECOACH_AUTO_STAGE_ALL\|STAGECOACH_MULTI_TURN_FALLBACK' internal/e2e internal/generate
# Expected: empty (env vars are sibling T2's job — not referenced by these tests).
grep -rn 'docs/configuration.md' internal/e2e internal/generate
# Expected: empty (docs are T3's job).

# Confirm no production file was touched by this subtask:
git diff --stat -- internal/config internal/cmd internal/generate/generate.go internal/hook pkg/stagecoach docs/
# Expected: empty (tests only).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/e2e/... ./internal/generate/...` clean
- [ ] `gofmt -l` lists nothing for the two new files
- [ ] `go test -tags e2e ./internal/e2e/... -run 'ConfigPrecedence' -v` — all subtests pass
- [ ] `go test ./internal/generate/... -run 'MultiTurnFallback_FalseFromFile' -v` — both rows pass
- [ ] `make test` (race, default tag set) green

### Feature Validation
- [ ] (a) TOML `auto_stage_all=false` ⇒ exit 2, no commit, dirty tree
- [ ] (b) `auto_stage_all=true`/omitted ⇒ exit 0, +1 commit, clean tree
- [ ] (c) TOML `multi_turn_fallback=false` ⇒ `MultiTurnFallbackValue()==false` + trigger absent + rescue; control (true) fires trigger
- [ ] (d) global TOML false + repo git-config true ⇒ exit 0 + commit (higher layer wins)
- [ ] (d-bonus) repo git-config false alone ⇒ exit 2 (git layer propagates false)
- [ ] Every e2e case passes `--provider stub` explicitly (stub-guard safe)
- [ ] The (c) control row proves the setup can fire multi-turn (so the false-case absence is meaningful)

### Scope-Boundary Validation
- [ ] NO `STAGECOACH_AUTO_STAGE_ALL`/`STAGECOACH_MULTI_TURN_FALLBACK` env cases (that's T2)
- [ ] NO docs/configuration.md edits (that's T3)
- [ ] NO config/production changes — tests only (consume S1's accessors)
- [ ] (c) does NOT duplicate S1's `TestMaterializeOverlay_AutoStageAll_MultiTurnFallback` (different package/layer)

### Code Quality & Docs
- [ ] New e2e file carries `//go:build e2e` as the first line
- [ ] Tests use existing helpers (no reinvented harness)
- [ ] Assertions are on contractual observables (exit code §15.4, commit count, tree state, verbose trigger) — not cosmetic stderr text
- [ ] Failure messages include enough context (exit code + stderr tail / buf tail) for fast triage

---

## Anti-Patterns to Avoid

- ❌ Don't select the stub via `$STAGECOACH_PROVIDER` env or git key — the self-hosting guard rejects it. Pass `--provider stub` on the CLI (every existing e2e test does).
- ❌ Don't make (c) an e2e/subprocess test — exit 3 (Rescue) is ambiguous between false and a failing multi-turn; the verbose trigger is only cleanly observable in-process. Keep (c) in package generate.
- ❌ Don't duplicate S1's materialize/overlay unit test — (c) must SOURCE cfg from a TOML file via `config.Load` (end-to-end), not assign the field directly.
- ❌ Don't add the env-var override case (`STAGECOACH_AUTO_STAGE_ALL`) — sibling T2 (P1.M1.T2.S1) owns it. Use the git-config Layer for (d).
- ❌ Don't use the snake_case git key `stagecoach.auto_stage_all` — git rejects underscores in the final segment (Issue 2). Use camelCase `stagecoach.autoStageAll`.
- ❌ Don't assert on cosmetic stderr wording for the auto_stage cases — assert exit code + git state (contractual per §15.4); stderr text is cosmetic (Issue 7).
- ❌ Don't stage the dirty file in case (a) — the whole point is an UN-staged dirty tree; `stageFile` would defeat the test (the staged path bypasses the auto-stage decision).
- ❌ Don't forget the control row in (c) — without it, a false-case "no trigger" result could hide a setup defect (e.g. cond b false). The true control proves the setup can fire.
- ❌ Don't run the e2e cases under `make test` and expect them to execute — the `//go:build e2e` tag excludes them; the gate is `go test -tags e2e`.

---

## Confidence Score: 9/10

One-pass success is very high: every helper, exit-code branch, and the exact (c) trigger-gate
observation pattern are enumerated with current line numbers and a verbatim mirror test
(`TestMultiTurnTriggerGate_TruthTable`). The (a) case is the mechanical inverse of the existing
`S2_OneFile_NoPlannerCall` positive control. The -1 is for the `config.Load` in-process path in (c): it
exercises real side effects (env, repo-local discovery), so the implementer must set
`DisableBootstrap:true` + `RepoDir` + a clean CWD; a careless Load call could pick up a stray
`.stagecoach.toml` or bootstrap-write — the gotcha box flags this, but it's the one non-mechanical step.
The control row (true) in (c) is the safety net that catches a misconfigured setup.
