# P1.M3.T1.S4 research findings — config observability: every precedence layer observably binds per role

Scope: READ-ONLY research at the tree state where P1.M1 (all), P1.M2 (all), P1.M3.T1.S1/S2 are LANDED and
P1.M3.T1.S3 is in flight (its TestHardCapHeartbeat_ExitCodeParity verified present in idle_parity_test.go:228).
Every fact below was verified at source by two scout agents + direct greps (see PRP for citations).

---

## 1. What the item asks (tasks.json contract, verbatim decode)

TESTS-ONLY. End-to-end BEHAVIORAL assertions that each precedence layer observably changes deadline behavior
for a role. Unlike S2/S3 (which set `cfg` fields directly), S4 drives **config.Load** through the REAL layer
plumbing (flags → env → git → repo file → defaults) and then observes kill/survive behavior through the real
`generate.CommitStaged` pipeline. Named cases: `--idle-timeout 1s` kills a hung stub; `[role.message].idle_timeout`
file key ditto; `STAGECOACH_MESSAGE_IDLE_TIMEOUT` env overrides the file; `--message-idle-timeout` flag overrides
all; unset ⇒ default 60s does NOT kill a short healthy run; hard-cap equivalents for `--timeout` (600s default
asserted via config, small override asserted via behavior). Git-key observability via the existing git-config
test-harness pattern (setGitConfig). Mock: stub agents only. Closes P1 (FR-R7 proof).

**No "recipe (d)" exists** in call_sites_and_error_contract.md (§9.6 has only personas (a)/(b)/(c)); S4 designs
its own ladder table. S2's PRP says explicitly: "the full behavioral verification is P1.M3.T1.S4."

## 2. The Load surface (verified)

- `internal/config/load.go:100` — `func Load(ctx context.Context, opts LoadOpts) (*Config, error)`;
  `LoadOpts{ConfigPathOverride string; RepoDir string; Flags *pflag.FlagSet; DisableBootstrap bool}`
  (load.go:23-27). `DisableBootstrap` is the documented TEST-ONLY seam (FR-B3) — S4 sets it true on every Load
  so no bootstrap file is written and no notice prints.
- **Repo-local `.stagecoach.toml` is resolved from process CWD, NOT RepoDir** (file.go:142-144 returns the bare
  relative path `".stagecoach.toml"`; load.go:155-159 Layer 3). `RepoDir` feeds ONLY the git layer
  (`loadGitConfig(opts.RepoDir)`). ⇒ every row that writes a repo file must chdir into the repo first.
- go.mod says **go 1.22** (module `github.com/dabstractor/stagecoach`) ⇒ `t.Chdir` (needs ≥1.24) is NOT
  available. Use the load_test.go:33-45 idiom: `os.Chdir(dir)` + `t.Cleanup(restore)`.
- Flags layer: skipped when `opts.Flags == nil` (load.go:183-185). Names read via `fs.Changed`:
  `"timeout"`, `"idle-timeout"`, `role+"-timeout"`, `role+"-idle-timeout"` (load.go:438/476-478/~504/526-528).
  Malformed flag values are SILENTLY ignored (no error return). Flags are STRINGs at zero default.
- Env layer: `STAGECOACH_TIMEOUT` (load.go:284), `STAGECOACH_IDLE_TIMEOUT` (load.go:294),
  `prefix+"_TIMEOUT"` / `prefix+"_IDLE_TIMEOUT"` where `prefix = "STAGECOACH_"+strings.ToUpper(role)`
  (load.go:~335/341/351). Presence semantics: present+non-empty overrides; **unset/empty is a no-op** — so
  `t.Setenv(k, "")` neutralizes ambient vars. Malformed env = HARD error.
- Git layer keys (git.go): `stagecoach.timeout` (:148), **`stagecoach.idleTimeout`** (:161, CAMELCASE — git
  rejects underscores in the final segment, config_ladder.md FINDING A), `stagecoach.role.<role>.timeout` (:180),
  **`stagecoach.role.<role>.idleTimeout`** (:192). Read via `git config --get` with `-C repoDir`; missing key =
  no-op; malformed = HARD error wrapped `git config <key>: %w`.
- File layer: `[defaults].idle_timeout` (fileDefaults.IdleTimeout string, file.go:58) parsed with **BARE
  `time.ParseDuration`** (file.go:211-215) — bare ints like `"1"` are REJECTED, use `"1s"`;
  `[role.<r>].idle_timeout` (fileRoleConfig.IdleTimeout, file.go:29) parsed via parseTimeout (bare int OK).
  Test TOMLs should carry `config_version = 3` (= `CurrentConfigVersion`, config.go:20) so the FR-B4/B9
  staleness advisory stays silent (advisory is noise-only, never an error).
- Global discovery (file.go:116-137): `XDG_CONFIG_HOME` (if absolute) wins over `$HOME/.config` — isolate with
  `t.Setenv("XDG_CONFIG_HOME", t.TempDir())`.
- Defaults (config.go:229-230): `Timeout: 600 * time.Second`, `IdleTimeout: 60 * time.Second`. ✔ 600s/60s.

## 3. The resolution + engine seam (verified)

- `internal/config/roles.go:78-87` — `func ResolveRoleDeadlines(role string, cfg Config) (idle, hard
  time.Duration)`: per-role non-zero override → globals, INDEPENDENT per field, no built-in rung; unknown role
  inherits globals. roleNames = planner/stager/message/arbiter (load.go:17).
- `internal/generate/generate.go:304` — `msgIdle, msgHard := config.ResolveRoleDeadlines("message", cfg)` →
  `generate.go:375` `provider.Execute(ctx, *spec, provider.Deadlines{Idle: msgIdle, Hard: msgHard}, deps.Verbose)`.
  `CommitStaged(ctx, deps Deps, cfg config.Config)`; `Deps{Git git.Git, Manifest provider.Manifest,
  Verbose *ui.Verbose, ...}` (generate.go:48-58).
- Executor D4 arming (executor.go:144): `if dl.Idle > 0 && (runtime.GOOS == "linux" || spec.ProgressiveStdout)` —
  stub print-mode ⇒ ProgressiveStdout false ⇒ **idle armed on LINUX only**; hard cap (`WithTimeout` on dl.Hard)
  is platform-uniform. Cause lines (verbose): `"idle deadline (%v without observable activity)"` (:152),
  `"hard cap (%v)"` (:157), printed by ui.VerboseCause as `DEBUG: cause: …`.
- Landed S2 platform idiom (idle_parity_test.go:73-133): SMALL idle (150ms-1s) + BOUNDED hard cap (3-4s) +
  stub `SleepMS` past both (5000) ⇒ Linux kills via idle, darwin/windows kill via the cap; assert
  `errors.Is(err, generate.ErrTimeout)` unconditionally, then **GOOS-split the cause assertion**
  (linux → "idle deadline"; else → "hard cap"). S4 adopts this verbatim; rows whose EXPECTED winner is the hard
  cap (idle deliberately large) assert "hard cap" unconditionally.

## 4. The harness to reuse (verified, landed)

- External test package `package generate_test` files: **hooks_freeze_test.go** (`initTempRepo(t) string` :45 —
  git init + identity + seed commit "seed: initial" + fileA.txt), **idle_parity_test.go** (`stageChange(t, repo)`
  :47 — modifies+stages fileA.txt; `headSHA(t, repo)` :58; TestIdleKill_ExitCodeParity :73;
  TestHardCapHeartbeat_ExitCodeParity :228 — S3's, present), **repro_freeze_test.go**. Shared namespace —
  REUSE these helpers; do not redefine. No `chdir` helper exists in the external package (load_test.go's is in
  package config, unreachable).
- `internal/stubtest/stubtest.go`: `Build(t testing.TB) string` (:61, sync.Once), `Manifest(bin string, o
  Options) provider.Manifest` (:172). Options: `Out` (single response), `SleepMS` (silent hang), `EmitMS/
  EmitLine/EmitTotalMS` (stdout ticks), `HeartbeatMS/HeartbeatTotalMS` (stderr ticks, never stdout).
- `internal/config/load_test.go`: `loadEnvSetup` (:87-97 — temp HOME/XDG, NO STAGECOACH_* scrub), `newFlagSet`
  (:54-84 — registers EVERY config-backed flag: String provider/model/timeout/idle-timeout, Bool verbose/
  no-color, per-role String `<role>-provider/-model/-timeout/-idle-timeout`, Int commits/max-commits, Bool
  single/no-decompose, StringArray exclude, String format/locale/template — COPY this registration verbatim
  into the ladder test so every `fs.Changed` probe is registered), `chdir` (:33-45, os.Chdir+t.Cleanup),
  `TestLoad_PerRoleTimeout_FlagBeatsEnv` (:1319-1333 — the Load-with-flags template: loadEnvSetup → chdir →
  t.Setenv env → fs.Set flag → Load(RepoDir, Flags) → assert).
- `internal/config/git_test.go:45-51` — `setGitConfig(t, dir, key, value)` via `git -C dir config key value`
  (the "existing git-config test harness" the contract names; copy into the external package).
- exitcode: external package may import internal/exitcode (`exitcode.For(err)`, `exitcode.Timeout == 124`) —
  idle_parity_test.go already does.
- Gates: `make test` = `go test -race ./...` (Makefile:70-71); `make lint` = `golangci-lint run` (:103-104).
  pflag v1.0.10 in go.mod.

## 5. Existing coverage S4 must NOT duplicate (all value-level, in internal/config)

M1T1S2/S3 landed: TestLoadEnv_IdleTimeout(:412), TestLoadEnv_PerRoleIdleTimeout(:461),
TestLoadFlags_IdleTimeoutString(:679), TestLoadFlags_PerRoleIdleTimeout(:718),
TestLoadGitConfig_IdleTimeout(:352), TestLoadGitConfig_PerRoleIdleTimeout(:393),
TestLoadTOMLIdleTimeout(:596)/_BareIntRejected(:634), TestResolveRoleDeadlines(:324),
TestLoad_GitOverridesRepoFile(:912), TestLoad_EnvOverridesGit(:933), TestLoad_CLIOverridesEnv(:948),
TestGenerateBootstrapConfig_DeadlineDefaults(:507). S2/S3 landed the cfg-direct BEHAVIORAL parity/discrimination
tests. **S4's novelty = Load-driven layer plumbing → ResolveRoleDeadlines → observable pipeline behavior.**
The runner asserts BOTH (a) resolved values at the ResolveRoleDeadlines seam (platform-uniform) and
(b) behavior (kill cause / success) — the cfg-level assert is the strict ladder proof; behavior corroborates.

## 6. Timing + design math

- Idle watcher polls ~500ms ⇒ a 1s idle kills at ~1-1.5s; a 1.5s hard cap kills at ~1.5s (platform-uniform).
- Standard row fixture: layer-under-test idle 1s; hard bound `--timeout 4s` (harness scaffolding via the flag
  layer's HARD field — orthogonal to idle-field precedence); losers 10/20/30s (all ≫ 4s so a precedence bug
  shows as a WRONG CAUSE (hard-cap kill at 4s) instead of a hang); hung stub `SleepMS: 5000` (self-bounds every
  row on every platform); Linux kill ~1-1.5s via idle; darwin kill 4s via cap. Elapsed hang-guards ≤ 6s kill /
  ≤ 10s success. -short skip on every row (~30s total added suite time).
- Precedence-broken failure modes stay fast+red: wrong winner < 4s ⇒ idle kill with wrong timing/cause on
  Linux (cause-line assert fails); wrong winner > 4s ⇒ hard-cap kill on Linux (idle-cause assert fails);
  cfg-level ResolveRoleDeadlines assert fails on ALL platforms regardless.
- Per-role-beats-global discriminator (inverted): `[defaults].idle_timeout="1s"` + `[role.message]
  idle_timeout="30s"` + `--timeout 4s` ⇒ expected HARD-CAP kill (30s idle > 4s cap). If global wrongly won,
  Linux idle-kills at 1s ⇒ red. Role isolation: `[role.planner].idle_timeout="1s"` ⇒ message must inherit 60s
  (cfg-level assert catches leaks on every platform; behavior corroborates on Linux).

## 7. Scope fences

- ZERO production change; ONE new test file. If any row fails, the defect is in the LANDED config surface
  (P1.M1), engine (P1.M2), or harness (S1/S2/S3) — surface it, never patch from here.
- Untracked `.stagecoach.toml` in the worktree never enters the payload: stageChange pre-stages fileA.txt so
  the staged diff is non-empty and FR16 auto-stage never fires (single-commit path; no decomposition).
- `t.Setenv` ⇒ rows must never call `t.Parallel()` (Go panics; chdir is process-global anyway).
- The §19 repo-file notice / any advisory prints to stderr — harmless test noise, never an error.
