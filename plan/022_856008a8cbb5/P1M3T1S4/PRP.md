name: "P1.M3.T1.S4 — config observability: every precedence layer observably binds per role (FR-R7 end-to-end proof, tests-only)"
description: >
  TESTS-ONLY task (no production change, no docs — the item contract: "DOCS: none — tests only; Mock: stub agents
  only"). Prove FR-R7's per-role two-deadline ladder binds END-TO-END through the REAL config.Load layer plumbing —
  unlike S2/S3 which set cfg fields directly, every test here drives config.Load(ctx, LoadOpts{RepoDir, Flags,
  DisableBootstrap:true}) with REAL precedence sources (pflag flag set, t.Setenv env vars, `git -C repo config` keys,
  a `.stagecoach.toml` written at the repo root, and nothing at all) and then observes KILL/SURVIVE behavior through
  the real generate.CommitStaged pipeline with stub agents. ONE NEW FILE internal/generate/idle_ladder_test.go,
  package generate_test (the external package S2 established — reuses initTempRepo/stageChange/headSHA and may import
  internal/exitcode). Table-driven TestIdleLadder_EveryLayerBinds with 16 rows; each row asserts BOTH (a) the resolved
  pair at the seam the pipeline actually reads — config.ResolveRoleDeadlines("message", *cfg) — and (b) observable
  behavior (kill via *generate.RescueError/ErrTimeout/exit 124 with the right verbose cause line, or success with HEAD
  advanced). Rows, mapped to the contract's named cases: DEFAULT (nothing set ⇒ cfg.IdleTimeout==60s && cfg.Timeout==600s
  asserted from the LOADED cfg, and a short healthy run is NOT killed); FLAG-GLOBAL (--idle-timeout 1s kills a hung
  stub); FLAG-PERROLE (--message-idle-timeout 1s beats env 10s + git 20s + file 30s — the full per-role ladder in one
  row); ENV-PERROLE-beats-FILE (STAGECOACH_MESSAGE_IDLE_TIMEOUT=1s overrides [role.message].idle_timeout="30s");
  ENV-GLOBAL (STAGECOACH_IDLE_TIMEOUT=1s); GIT-GLOBAL (camelCase `git config stagecoach.idleTimeout 1s` — FINDING A);
  GIT-PERROLE (stagecoach.role.message.idleTimeout 1s); GIT-beats-FILE (git 1s beats [defaults].idle_timeout="30s" —
  FR34's unusual git>file ordering); FILE-GLOBAL ([defaults].idle_timeout="1s"); FILE-PERROLE ([role.message].
  idle_timeout="1s"); FILE-PERROLE-beats-GLOBAL ([defaults]=1s + [role.message]=30s ⇒ the HARD CAP kills at the 4s
  bound — the inverted discriminator proving per-role won); HARD-CAP rows for --timeout 1500ms flag, [role.message].
  timeout="1500ms" file, STAGECOACH_TIMEOUT=1500ms env, stagecoach.timeout=1500ms git (600s default already asserted
  in the DEFAULT row); PLANNER-ISOLATION ([role.planner].idle_timeout="1s" must NOT leak into message — message
  inherits 60s, only the 4s bound kills). Every idle row carries a `--timeout 4s` flag as harness scaffolding so
  non-Linux platforms (where the D4 rule DISARMS the idle watcher for print-mode stubs) terminate bounded via the cap —
  the landed S2 idiom (idle_parity_test.go:122): assert errors.Is(err, generate.ErrTimeout) unconditionally, then
  GOOS-split the cause-line assertion (linux ⇒ "DEBUG: cause: idle deadline"; darwin/windows ⇒ "DEBUG: cause: hard
  cap"); rows whose expected winner IS the hard cap assert "hard cap" unconditionally. Hung stub = stubtest.Options{
  Out:"feat: never", SleepMS:5000} (sleeps past every deadline); healthy stub = Options{Out:"feat: quick"}. Loser
  values (10/20/30s) all exceed the 4s bound so a precedence bug fails FAST with a wrong cause, never a hang; the
  SleepMS-5000 stub self-bounds every row on every platform. Kill rows also assert Execute-count==1 (no retry),
  HEAD unchanged, exitcode.For==124, elapsed ≤6s; the success row asserts HEAD advanced + subject equality + no
  cause line. Helpers in-file: newLadderFlagSet (COPIES load_test.go newFlagSet's full flag registration so every
  fs.Changed probe is registered), ladderScrubEnv (t.Setenv every ambient STAGECOACH_* var to "" — empty is the
  documented no-op), ladderIsolateHome (temp HOME/XDG_CONFIG_HOME), ladderChdir (os.Chdir+t.Cleanup — go 1.22 has NO
  t.Chdir, and the repo-local .stagecoach.toml is resolved from process CWD, NOT RepoDir), ladderGitConfig (the
  setGitConfig pattern), ladderWriteRepoFile (writes .stagecoach.toml with config_version = 3). LoadOpts.DisableBootstrap:
  true on every Load (the documented TEST-ONLY seam) so no bootstrap file is written. GOTCHAS: git keys are camelCase
  (git rejects stagecoach.idle_timeout); [defaults].idle_timeout parses with BARE time.ParseDuration (bare ints
  rejected — write "1s" strings everywhere); t.Setenv forbids t.Parallel; every test TOML carries config_version = 3
  (== CurrentConfigVersion) to keep the FR-B4/B9 advisory silent; reuse — never redefine — initTempRepo/stageChange/
  headSHA from the shared generate_test package. If ANY row fails, the defect is in the LANDED config surface (P1.M1),
  engine (P1.M2), or harness (S1/S2/S3) — surface it, never patch from here. Closes P1: the final proof that FR-R7's
  ladder binds observably at every layer.

---

## Goal

**Feature Goal**: Prove FR-R7 end-to-end: every one of the five precedence layers (flag > env > git > file > default),
for both deadlines (idle + hard cap), for the per-role AND global axes, observably binds the message role's deadline
behavior through the REAL pipeline — `config.Load` (real flag set / env / git keys / TOML file / defaults) →
`config.ResolveRoleDeadlines("message", cfg)` (the exact seam generate.go:304 reads) → `provider.Execute` →
kill-or-survive behavior with stub agents. This is the config-observability counterpart to S2 (kill parity) and S3
(false-kill/backstop discrimination), and it **closes P1**.

**Deliverable** (1 new test file, ZERO production change):
- **internal/generate/idle_ladder_test.go** — `package generate_test`, with `TestIdleLadder_EveryLayerBinds`:
  a 16-row table-driven test + 6 small helpers. No other file is created or modified.

**Success Definition**:
- Nothing set ⇒ loaded cfg carries `IdleTimeout == 60s` && `Timeout == 600s` AND a short healthy run SUCCEEDS
  (commit created) — the default layer binds without false kills.
- Each of flag/env/git/file layers, set alone (global or per-role message), produces an IDLE KILL of a hung stub
  on Linux (`DEBUG: cause: idle deadline`) and a bounded kill everywhere (cause `hard cap` on D4-disarmed platforms).
- Precedence holds behaviorally: `--message-idle-timeout` beats env+git+file; env beats file; git beats file;
  `[role.message].idle_timeout` beats `[defaults].idle_timeout` (inverted discriminator: only the 4s bound kills).
- Hard-cap equivalents bind behaviorally: `--timeout 1500ms`, `[role.message].timeout="1500ms"`,
  `STAGECOACH_TIMEOUT=1500ms`, `git config stagecoach.timeout 1500ms` each kill a hung stub with cause
  `hard cap` on ALL platforms.
- `[role.planner].idle_timeout="1s"` does NOT leak into the message role (message inherits 60s; cfg-level assert
  is platform-uniform, behavior corroborates on Linux).
- Every kill row: `*generate.RescueError` + `errors.Is(err, generate.ErrTimeout)` + `exitcode.For(err)==124` +
  Execute-count == 1 + HEAD unchanged + elapsed ≤ 6s. All existing tests stay green; `make test` + `make lint`
  green; `gofmt -l` empty; `git status` shows ONLY the one new test file.

## User Persona (if applicable)

**Target User**: The maintainer closing P1 — verifying that the five-layer config ladder shipped across P1.M1
(fields, env/git/file wiring, flags) and P1.M2 (executor + 13 threaded call sites) actually changes RUNTIME
behavior per role, not just config structs. Also any future contributor touching config loading, precedence, or
deadlines: these rows are the fence — a regression in any layer's plumbing (a renamed env var, a snake_case git
key, a broken overlay guard) turns a kill row red or a survive row red.

**Use Case**: A user sets `git config stagecoach.idleTimeout 5s` in one repo, `STAGECOACH_MESSAGE_IDLE_TIMEOUT=10s`
in their shell, and `--idle-timeout 1s` once on the CLI. FR34 promises flag > env > git > file > default. These
tests are the only place that promise is verified by OBSERVING which deadline actually kills a hung agent.

**User Journey**: Layer sources are staged (flag set on a pflag set; env via t.Setenv; git keys written into the
temp repo; TOML written at the repo root; nothing for the default row) → config.Load merges them (real overlay
code) → CommitStaged resolves the message role's deadlines via ResolveRoleDeadlines → Execute arms the watcher →
the hung stub is killed by the layer that won (cause line names it) or the healthy run commits.

**Pain Points Addressed**: Every prior test (M1T1S2/S3 value-level; S2/S3 behavioral with cfg set directly) proves
the ladder PIECES. Nothing proves the composition — that a value typed into a real layer source surfaces as a
real kill (or a real survival) through the pipeline. A silent seam break (env var renamed, git key snake_cased,
overlay guard inverted) would ship undetected today.

## Why

- **FR-R7's ladder is only as real as its observable binding** (spec §9.15/§9.8 FR34): "Each role resolves TWO
  deadlines independently, on the same precedence ladder as provider/model/reasoning" — the spec's own success
  criterion is behavioral ("kill the role's agent"), and this task is that criterion's proof.
- **The seams are many and individually fragile** (config_ladder.md §8's ten seam groups): camelCase git keys
  (git rejects `stagecoach.idle_timeout`), bare-ParseDuration on `[defaults].idle_timeout` (bare ints rejected),
  CWD-relative repo-file discovery, Changed-only flag semantics, empty-env no-op semantics, per-role field-merge
  overlay guards. Each is unit-tested in isolation (internal/config); only an end-to-end row catches a
  composition break ACROSS packages (config → generate → provider).
- **Closes P1**: P1.M3.T1.S2 proved kill parity, S3 proves discrimination; S4 proves the CONFIG LADDER feeds
  those behaviors. Together: FR-A1/A2/A5 + FR-R7 all pinned end-to-end.
- **Cheap and hermetic**: stub agents only; ~30s of suite time, all `-short`-skipped; no new deps.

## What

**User-visible behavior**: None (tests only).

**Technical change**: One new external-package test file with a table-driven test. See the Blueprint.

### Success Criteria
- [ ] `internal/generate/idle_ladder_test.go` created, `package generate_test`, containing
      `TestIdleLadder_EveryLayerBinds` with exactly the 16 rows of the Blueprint table (names as specified),
      plus the 6 helpers; every row has a `-short` skip guard.
- [ ] Every row drives `config.Load(context.Background(), config.LoadOpts{RepoDir: repo, Flags: fs,
      DisableBootstrap: true})` after `ladderScrubEnv` + `ladderIsolateHome` + `ladderChdir(t, repo)` +
      `stageChange`, asserts `config.ResolveRoleDeadlines("message", *cfg)` equals the row's expected pair
      (and, for the planner-isolation row, `ResolveRoleDeadlines("planner", *cfg)`), then runs
      `generate.CommitStaged` with `stubtest.Manifest(stubtest.Build(t), c.stub)` and `ui.NewVerbose(&vbuf, true)`.
- [ ] Kill rows assert: `errors.As → *generate.RescueError`, `errors.Is(err, generate.ErrTimeout)`,
      `exitcode.For(err) == exitcode.Timeout` (124), Execute-count == 1 (`strings.Count(vbuf.String(),
      "DEBUG: command: ") == 1`), HEAD unchanged, elapsed ≤ row bound; idle-expecting rows GOOS-split the cause
      assertion (linux ⇒ contains `"DEBUG: cause: idle deadline"`, else ⇒ `"DEBUG: cause: hard cap"`); hard-cap
      rows assert `"DEBUG: cause: hard cap"` unconditionally.
- [ ] The DEFAULT row asserts `cfg.IdleTimeout == 60*time.Second && cfg.Timeout == 600*time.Second` from the
      loaded cfg AND run success (HEAD advanced to `res.CommitSHA`, `res.Subject == "feat: quick"`, no
      `"DEBUG: cause:"` line).
- [ ] Git rows use the CAMELCASE keys `stagecoach.idleTimeout` / `stagecoach.role.message.idleTimeout` /
      `stagecoach.timeout` via a `ladderGitConfig` helper (the `git -C repo config key value` pattern).
- [ ] File rows write `.stagecoach.toml` at the repo root (after chdir) with `config_version = 3` and
      duration-STRING values (`"1s"`, `"30s"`, `"1500ms"` — never bare ints).
- [ ] ZERO production change; `git status` shows ONLY the new test file; `make test` + `make lint` green;
      `gofmt -l` empty; all existing tests green.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact Load surface (LoadOpts fields incl. the DisableBootstrap test seam; CWD-relative repo-file
discovery forcing chdir; go 1.22 ⇒ os.Chdir helper, NOT t.Chdir), every layer's exact spellings (flag names
`idle-timeout`/`message-idle-timeout`/`timeout`; env vars `STAGECOACH_IDLE_TIMEOUT`/`STAGECOACH_MESSAGE_IDLE_TIMEOUT`/
`STAGECOACH_TIMEOUT`; camelCase git keys; TOML keys `[defaults].idle_timeout`/`[role.message].idle_timeout` with the
bare-int-rejected quirk), the resolution seam (`ResolveRoleDeadlines` at roles.go:78-87 → generate.go:304 →
provider.Deadlines), the landed D4 platform idiom (bounded `--timeout 4s` scaffolding + GOOS-split cause assertion,
from idle_parity_test.go:122), the shared external-package helpers to reuse (initTempRepo/stageChange/headSHA), the
newFlagSet registration to copy, the scrub/idle semantics (empty env = no-op), the advisory-silencing
`config_version = 3`, the timing math (500ms watcher poll ⇒ 1s idle kills at ~1-1.5s; losers 10-30s ≫ 4s bound ⇒
precedence bugs fail fast with a wrong cause), the no-retry/HEAD/exit-124 assertion set from S2, and the fence that
a failing row indicates a LANDED defect.

### Documentation & References

```yaml
# MUST READ — this task's own verified findings (every fact below was checked at source)
- docfile: plan/022_856008a8cbb5/P1M3T1S4/research/findings.md
  why: "§2 the Load surface (LoadOpts fields, CWD-relative repo file, flag/env/git/file spellings + quirks);
        §3 the resolution seam + D4 arming + the landed GOOS-split cause idiom; §4 the harness to reuse
        (external-package helpers with file:line, newFlagSet registration to copy, setGitConfig pattern, gates);
        §5 the existing value-level tests NOT to duplicate; §6 the timing/design math (why losers exceed the 4s
        bound; the inverted per-role discriminator; planner isolation); §7 scope fences."
  critical: "§2's three traps an implementer WILL hit without them: (1) the repo-local .stagecoach.toml resolves
             from process CWD not RepoDir — chdir required, and go 1.22 has no t.Chdir; (2) DisableBootstrap:true
             is required or Load bootstraps a global file; (3) [defaults].idle_timeout rejects bare ints — write
             duration strings. Plus §3: idle rows are Linux-discriminating; the 4s bound keeps darwin/windows
             green via the cap with the swapped cause line."

# MUST READ — the ladder map (the item's RESEARCH NOTE; ten seam groups + five-layer ladder + FINDING A)
- docfile: plan/022_856008a8cbb5/architecture/config_ladder.md
  section: "§1-§8 (config.go/file.go/load.go/git.go/roles.go/root.go/bootstrap.go seams; §7 test conventions incl.
            the sentinel-discipline and the git_test.go found/missing/bad triple; §8 the checklist)."
  critical: "FINDING A: git config rejects underscores in the final key segment — the keys are CAMELCASE
             (stagecoach.idleTimeout). The ladder quote: per-role keys '(CLI flag > env > file > git, all already
             merged into cfg.Roles by the loaders)' then per-role > global — loaders merge BEFORE resolution."

# MUST READ — the S2 contract (this task's package, helpers, and assertion ladder come from it)
- docfile: plan/022_856008a8cbb5/P1M3T1S2/PRP.md
  why: "Defines the external package generate_test + idle_parity_test.go (whose helpers initTempRepo/stageChange/
        headSHA this task REUSES), the LARGE/SMALL discriminator idiom, the no-retry counting technique, the
        exit-124 assertion set this task mirrors per row, and the import-cycle rationale. Its DOWNSTREAM note:
        'P1.M3.T1.S4 reuses this harness; idle_parity_test.go is its natural extension home.'"
  critical: "S2's tests set cfg directly; S4's novelty is driving config.Load with REAL layer sources — keep that
             distinction crisp in the file's header comment."

# READ — the in-flight sibling contract (assume it lands exactly as specified; do not duplicate)
- docfile: plan/022_856008a8cbb5/P1M3T1S3/PRP.md
  why: "S3 appends TestHardCapHeartbeat_ExitCodeParity to idle_parity_test.go (verified already present at :228) —
        same package, so helper-name collisions are possible; S3 adds none (it reuses). Its cause-string facts
        (hard-cap naming lives ONLY in the verbose line) apply to S4's hard-cap rows."
  critical: "Do not re-assert S3's personas (progressive/heartbeat) — S4's stubs are the simple (a) hung and
             healthy forms only."

# MUST EDIT — the ONE new file
- file: internal/generate/idle_ladder_test.go   # NEW; package generate_test (external)
  why: "The deliverable. External package is REQUIRED to import internal/exitcode (exitcode imports generate ⇒
        cycle for in-package tests) and to share initTempRepo/stageChange/headSHA with hooks_freeze_test.go /
        idle_parity_test.go."
  pattern: "File header comment citing FR-R7/FR34/§9.8 + the S2-vs-S4 distinction (cfg-direct → Load-driven).
            Mirror idle_parity_test.go's import block + assertion style."
  gotcha: "REUSE the shared helpers — do NOT redefine initTempRepo/stageChange/headSHA (compile error if you do).
           Grep the external package for any helper name before defining a new one."

# CONTEXT — the Load + layer code under observation (READ-ONLY)
- file: internal/config/load.go        # Load (:100), LoadOpts (:23-27), layer comments (:155-185), loadFlags
        #   (:438/:476/:~504/:526), loadEnv (:284/:294/:~335-351)
- file: internal/config/file.go        # repoLocalConfigPath ".stagecoach.toml" (:142-144 — CWD-relative!),
        #   globalConfigPath (:116-124 — XDG wins), [defaults].idle_timeout bare ParseDuration (:211-215)
- file: internal/config/git.go         # stagecoach.timeout (:148), stagecoach.idleTimeout (:161),
        #   stagecoach.role.<r>.timeout (:180), stagecoach.role.<r>.idleTimeout (:192) — camelCase
- file: internal/config/roles.go       # ResolveRoleDeadlines (:78-87) — THE seam generate reads
- file: internal/config/config.go      # IdleTimeout/Timeout fields (:86-87), Defaults 60s/600s (:229-230),
        #   CurrentConfigVersion=3 (:20)
  critical: "READ-ONLY. If any row fails, the defect is HERE (P1.M1's surface) or in the engine/harness — surface
             it in the report; do NOT modify config, generate, or provider code from this tests-only task."

# CONTEXT — the pipeline + engine under observation (READ-ONLY)
- file: internal/generate/generate.go  # CommitStaged (:188), Deps (:48-58), ResolveRoleDeadlines call (:304),
        #   Execute with provider.Deadlines (:375)
- file: internal/provider/executor.go  # D4 arming (:144: Idle>0 && (linux || ProgressiveStdout)); cause strings
        #   (:152 "idle deadline (%v without observable activity)", :157 "hard cap (%v)")
- file: internal/generate/idle_parity_test.go  # LANDED model: TestIdleKill_ExitCodeParity (:73-133) — the
        #   bounded-cap + GOOS-split idiom this task copies; helpers stageChange (:47), headSHA (:58)
- file: internal/generate/hooks_freeze_test.go  # initTempRepo (:45) — seeds repo + "seed: initial" commit
- file: internal/stubtest/stubtest.go  # Build (:61), Manifest (:172), Options{Out, SleepMS, ...}
- file: internal/exitcode/exitcode.go  # For() + Timeout == 124
- file: internal/config/load_test.go   # newFlagSet (:54-84 — COPY the registration), chdir (:33-45 — the
        #   os.Chdir+t.Cleanup idiom), TestLoad_PerRoleTimeout_FlagBeatsEnv (:1319-1333 — the Load-with-flags
        #   template), loadEnvSetup (:87-97 — the HOME/XDG isolation; NOTE it scrubs nothing — S4 adds the scrub)
- file: internal/config/git_test.go    # setGitConfig (:45-51) — the "existing git-config test harness" pattern
- file: go.mod / Makefile              # go 1.22 (NO t.Chdir); make test = go test -race ./...; make lint =
        #   golangci-lint run
```

### Current Codebase tree (relevant slice)

```bash
internal/generate/                  # 16 test files; EXTERNAL (package generate_test): hooks_freeze_test.go,
                                    #   idle_parity_test.go, repro_freeze_test.go (shared namespace)
  idle_parity_test.go               # S2's file (+S3's append) — helpers to REUSE; the idiom to mirror
  hooks_freeze_test.go              # initTempRepo (repo + seed commit)
internal/config/
  load.go / file.go / git.go / roles.go / config.go   # the Load + layer code under observation (READ-ONLY)
  load_test.go / git_test.go        # newFlagSet + chdir + setGitConfig patterns to COPY (package config —
                                    #   its helpers are unimportable; copy, don't import)
internal/provider/executor.go       # D4 arming + cause strings (READ-ONLY)
internal/stubtest/stubtest.go       # Build/Manifest/Options (READ-ONLY; landed S1 knobs)
internal/exitcode/exitcode.go       # For/Timeout=124 (READ-ONLY)
internal/ui/                        # NewVerbose(&buf, true) (READ-ONLY)
# go.mod (go 1.22) / Makefile — READ-ONLY (no new deps; stdlib + existing internal imports + spf13/pflag)
```

### Desired Codebase tree with files to be added

```bash
internal/generate/idle_ladder_test.go   # NEW — package generate_test: TestIdleLadder_EveryLayerBinds (16 rows)
                                        #   + helpers: newLadderFlagSet, ladderScrubEnv, ladderIsolateHome,
                                        #   ladderChdir, ladderGitConfig, ladderWriteRepoFile
# (NO production file. NO engine/config/stub change. NO docs. NO modification of any existing file.)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (go 1.22 has NO testing.T.Chdir — and the repo file is CWD-relative): the repo-local
// .stagecoach.toml resolves from the process CWD (file.go:142-144 returns the bare relative path), NOT from
// LoadOpts.RepoDir (RepoDir feeds ONLY the git layer). go.mod says go 1.22, so t.Chdir (≥1.24) does not exist.
// Every row must call a ladderChdir helper (os.Chdir(repo) + t.Cleanup(restore), copying load_test.go:33-45)
// BEFORE Load. Because chdir is process-global, rows must NEVER call t.Parallel() (also forbidden with t.Setenv).

// CRITICAL (DisableBootstrap — or Load writes a file and prints): with no global config (isolated XDG home)
// Load's FR-B3 fallback BOOTSTRAP-WRITES a config and prints a notice, unless LoadOpts{DisableBootstrap: true}.
// That flag is the documented TEST-ONLY seam — set it on EVERY Load. (The written bootstrap's timeout keys are
// commented/inert anyway, but the write+notice is noise and an extra FS effect you don't want.)

// CRITICAL (ambient STAGECOACH_* env leaks into Load): loadEnvSetup scrubs nothing. If the developer's shell
// exports any STAGECOACH_* var (e.g. STAGECOACH_IDLE_TIMEOUT=45s), every row would silently mutate. Scrub FIRST:
// iterate os.Environ(), and for every key with the "STAGECOACH_" prefix call t.Setenv(key, "") — EMPTY is the
// documented no-op (presence semantics: present+non-empty overrides; unset/empty ignored; STAGECOACH_CONFIG
// likewise ignores ""). THEN let the row's setup t.Setenv its own vars. Order matters: scrub before setup.

// CRITICAL (camelCase git keys — FINDING A): `git config stagecoach.idle_timeout 1s` is REJECTED by git
// ("invalid key") and would fail the row setup; the keys are stagecoach.idleTimeout /
// stagecoach.role.message.idleTimeout (camelCase final segment), even though TOML/env spell idle_timeout /
// STAGECOACH_..._IDLE_TIMEOUT. This asymmetry is exactly what the git rows exist to pin.

// CRITICAL ([defaults].idle_timeout rejects bare ints): the GLOBAL file key parses with BARE time.ParseDuration
// (file.go:211-215) — "1" is an error; per-role file keys (and env/git/flags) use parseTimeout where "1" means
// 1s. Write duration STRINGS everywhere ("1s", "30s", "1500ms") so every layer parses identically.

// CRITICAL (test TOMLs carry config_version = 3): CurrentConfigVersion == 3 (config.go:20). A file with active
// settings but no version triggers the FR-B4/B9 advisory notice (stderr noise; never an error). Prefix every
// written .stagecoach.toml with "config_version = 3\n\n".

// CRITICAL (the D4 platform split — idle rows discriminate on Linux only): the idle watcher arms iff
// Idle>0 AND (GOOS==linux || spec.ProgressiveStdout) (executor.go:144); print-mode stubs have ProgressiveStdout
// false until P2. ⇒ On darwin/windows an idle-configured row would run to the stub's 5s answer and SUCCEED
// (red) unless bounded. EVERY idle row carries `--timeout 4s` (the flag layer's HARD field — orthogonal to the
// idle-field precedence under test) so non-Linux kills via the cap in 4s. Assert errors.Is(err,
// generate.ErrTimeout) unconditionally, then GOOS-split the cause line EXACTLY as landed S2 does
// (idle_parity_test.go:122): linux ⇒ "DEBUG: cause: idle deadline"; else ⇒ "DEBUG: cause: hard cap". Never
// fabricate arming; never skip platforms. Rows whose EXPECTED winner is the hard cap (FilePerRoleIdle_BeatsGlobal,
// the four hard-cap rows, PlannerIsolation) assert "hard cap" UNCONDITIONALLY (idle is deliberately > the bound).

// GOTCHA (reuse the shared external-package helpers): package generate_test already defines initTempRepo
// (hooks_freeze_test.go:45 — seeds the repo AND an initial "seed: initial" commit), stageChange (idle_parity_
// test.go:47 — modifies+stages fileA.txt), headSHA (:58). REUSE them; redefining any is a compile error. Before
// defining a new helper name, grep the three external files for collisions. (load_test.go's newFlagSet/chdir and
// git_test.go's setGitConfig live in package config's test binary — unreachable; COPY them under the ladder*
// names.)

// GOTCHA (newFlagSet must register EVERY config-backed flag): loadFlags probes fs.Changed for provider/model/
// timeout/idle-timeout/verbose/no-color/<role>-{provider,model,timeout,idle-timeout}/commits/max-commits/single/
// no-decompose/exclude/format/locale/template. Copy load_test.go:54-84's FULL registration into newLadderFlagSet
// (zero defaults, String/Bool/Int/StringArray as there) rather than a partial list — then fs.Set("idle-timeout",
// "1s") marks Changed exactly as production cobra does.

// GOTCHA (flag malformed values are silently ignored; env malformed are Load errors): never feed a row a
// malformed value "expecting" the silent-ignore behavior here — that asymmetry is M1T1S3's unit-tested
// territory. All S4 values are well-formed duration strings.

// GOTCHA (timing margins vs the ~500ms watcher poll): a 1s idle kills at the first detection tick after expiry
// (~1-1.5s). Hard caps fire at their wall-clock value (WithTimeout — platform-uniform). Margins: losers 10/20/30s
// ≫ the 4s bound (a precedence bug surfaces as a WRONG CAUSE at 4s, not a hang); stub SleepMS 5000 > every
// deadline (self-bounding); elapsed hang-guards ≤6s kill rows / ≤10s success row. Re-run with -count=2 before
// trusting (deadline tests are the flake-prone kind).

// GOTCHA (the untracked .stagecoach.toml never pollutes the commit): stageChange pre-stages fileA.txt, so the
// staged diff is non-empty, FR16 auto-stage never fires, and the single-commit path (no decomposition) snapshots
// only the staged content. The repo file sits untracked and invisible to the payload.

// GOTCHA (advisory/§19 notices print to stderr — harmless): the repo-file §19 notice and any config advisory
// print via the config package's notice writer; they are test-output noise, never errors. Do not "fix" them.
```

## Implementation Blueprint

### Data models and structure

None (tests only). The file consumes LANDED types: `config.LoadOpts{RepoDir, Flags, DisableBootstrap}`,
`config.Load` → `*config.Config`, `config.ResolveRoleDeadlines(role, cfg) (idle, hard time.Duration)`,
`generate.CommitStaged(ctx, generate.Deps{Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&vbuf, true)}, cfg)`,
`*generate.RescueError`, `generate.ErrTimeout`, `exitcode.For`/`exitcode.Timeout`, `stubtest.Build`/`Manifest`/
`Options{Out, SleepMS}`, `pflag.FlagSet` (github.com/spf13/pflag v1.0.10).

The row type:

```go
// ladderCase is one precedence-layer→behavior row. setup stages the REAL layer sources (t.Setenv env vars,
// ladderGitConfig git keys, ladderWriteRepoFile TOML, fs.Set flags) AFTER the runner's scrub+isolate+chdir.
type ladderCase struct {
    name      string
    setup     func(t *testing.T, repo string, fs *pflag.FlagSet)
    stub      stubtest.Options // hung: {Out: "feat: never", SleepMS: 5000}; healthy: {Out: "feat: quick"}
    wantIdle  time.Duration    // expected ResolveRoleDeadlines("message", *cfg) idle
    wantHard  time.Duration    // … hard
    wantRoles map[string][2]time.Duration // extra roles to assert (e.g. planner isolation)
    want      string           // "idle": linux idle-kill / darwin cap-kill; "hard": cap-kill everywhere;
                               // "success": err==nil, HEAD advanced
    maxElapsed time.Duration   // hang-guard
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/generate/idle_ladder_test.go — helpers
  - FILE HEADER comment: cite FR-R7/§9.15, FR34/§9.8, config_ladder.md's five-layer ladder; state the S2/S4
    distinction (S2/S3 set cfg directly and proved kill parity/discrimination; THIS file drives config.Load
    through real layer sources and proves the ladder OBSERVABLY binds per role); note package generate_test is
    required for internal/exitcode (import cycle) and shares helpers with hooks_freeze_test.go/idle_parity_test.go.
  - IMPORTS (mirror idle_parity_test.go + pflag + config): bytes, context, errors, fmt (optional), os, os/exec,
    path/filepath, runtime, strings, testing, time, github.com/spf13/pflag, internal/config, internal/exitcode,
    internal/generate, internal/git, internal/stubtest, internal/ui.
  - HELPERS (each with a one-line doc comment; grep the external package for name collisions FIRST):
    func newLadderFlagSet(t *testing.T) *pflag.FlagSet — COPY load_test.go:54-84 newFlagSet's registration
        VERBATIM (String provider/model/timeout/"idle-timeout"; Bool verbose/no-color; per-role loop String
        role+"-provider"/-model/-timeout/-role+"-idle-timeout"; Int commits/max-commits; Bool single/
        no-decompose; StringArray exclude; String format/locale/template) — zero defaults, so fs.Set marks
        Changed exactly as production cobra does.
    func ladderScrubEnv(t *testing.T) — for _, kv := range os.Environ() { if name, _, ok := strings.Cut(kv, "=");
        ok && strings.HasPrefix(name, "STAGECOACH_") { t.Setenv(name, "") } } — EMPTY is the documented no-op.
    func ladderIsolateHome(t *testing.T) — home := t.TempDir(); t.Setenv("HOME", home); t.Setenv(
        "XDG_CONFIG_HOME", home) — global discovery finds nothing (XDG wins; DisableBootstrap skips the write).
    func ladderChdir(t *testing.T, dir string) — copy load_test.go:33-45's chdir idiom: remember os.Getwd(),
        os.Chdir(dir), t.Cleanup(func() { os.Chdir(back) }). (go 1.22: NO t.Chdir.)
    func ladderGitConfig(t *testing.T, repo, key, value string) — copy git_test.go:45-51 setGitConfig:
        exec.Command("git", "-C", repo, "config", key, value), CombinedOutput, t.Fatalf on error.
    func ladderWriteRepoFile(t *testing.T, repo, toml string) — os.WriteFile(filepath.Join(repo,
        ".stagecoach.toml"), []byte(toml), 0o644) + t.Fatalf on error. (CWD-relative discovery ⇒ chdir first.)

Task 2: APPEND the runner + 16-row table to idle_ladder_test.go — TestIdleLadder_EveryLayerBinds
  - RUNNER shape (single test func; the table below is the spec — adapt field names to your ladderCase):
        func TestIdleLadder_EveryLayerBinds(t *testing.T) {
            if testing.Short() { t.Skip("skipping slow config-ladder behavioral tests in -short mode") }
            bin := stubtest.Build(t)
            for _, c := range ladderCases {
                t.Run(c.name, func(t *testing.T) {
                    if testing.Short() { t.Skip("skipping slow config-ladder row in -short mode") }
                    repo := initTempRepo(t)            // shared helper (hooks_freeze_test.go:45)
                    ladderScrubEnv(t); ladderIsolateHome(t); ladderChdir(t, repo)
                    fs := newLadderFlagSet(t)
                    if c.setup != nil { c.setup(t, repo, fs) }
                    cfg, err := config.Load(context.Background(), config.LoadOpts{
                        RepoDir: repo, Flags: fs, DisableBootstrap: true})
                    if err != nil { t.Fatalf("config.Load: %v", err) }
                    // (a) THE SEAM ASSERT — platform-uniform, the strict ladder proof: the exact resolver
                    // generate.go:304 reads. A wrong layer value fails HERE on every platform.
                    idle, hard := config.ResolveRoleDeadlines("message", *cfg)
                    if idle != c.wantIdle || hard != c.wantHard {
                        t.Errorf("message deadlines = (%v, %v), want (%v, %v)", idle, hard, c.wantIdle, c.wantHard)
                    }
                    for role, want := range c.wantRoles {
                        i2, h2 := config.ResolveRoleDeadlines(role, *cfg)
                        if i2 != want[0] || h2 != want[1] { t.Errorf("%s deadlines = (%v, %v), want (%v, %v)", role, i2, h2, want[0], want[1]) }
                    }
                    // (b) THE BEHAVIOR ASSERT — kill/survive through the real pipeline.
                    stageChange(t, repo); before := headSHA(t, repo)   // shared helpers
                    var vbuf bytes.Buffer
                    start := time.Now()
                    res, err := generate.CommitStaged(context.Background(), generate.Deps{
                        Git: git.New(repo), Manifest: stubtest.Manifest(bin, c.stub),
                        Verbose: ui.NewVerbose(&vbuf, true)}, *cfg)
                    elapsed := time.Since(start)
                    if elapsed > c.maxElapsed { t.Errorf("elapsed %v exceeds hang-guard %v; buf:\n%s", elapsed, c.maxElapsed, vbuf.String()) }
                    switch c.want {
                    case "success":
                        if err != nil { t.Fatalf("healthy run failed: %v (FR-R7 default must not kill); buf:\n%s", err, vbuf.String()) }
                        if res.Subject != c.stub.Out { t.Errorf("Subject = %q, want %q", res.Subject, c.stub.Out) }
                        if got := headSHA(t, repo); got == before || got != res.CommitSHA {
                            t.Errorf("HEAD = %s, want advanced to %s (before %s)", got, res.CommitSHA, before)
                        }
                        if strings.Contains(vbuf.String(), "DEBUG: cause:") {
                            t.Errorf("a deadline fired on a healthy default run; buf:\n%s", vbuf.String())
                        }
                        if n := strings.Count(vbuf.String(), "DEBUG: command: "); n != 1 { t.Errorf("Execute count = %d, want 1", n) }
                    default: // "idle" | "hard" — a bounded kill
                        if err == nil { t.Fatalf("expected a deadline rescue, got nil (the layer did not bind); buf:\n%s", vbuf.String()) }
                        var re *generate.RescueError
                        if !errors.As(err, &re) { t.Fatalf("err = %T %v, want *generate.RescueError", err, err) }
                        if !errors.Is(err, generate.ErrTimeout) { t.Errorf("errors.Is(ErrTimeout)=false: %v", err) }
                        if code := exitcode.For(err); code != exitcode.Timeout {
                            t.Errorf("exitcode.For = %d, want %d (124)", code, exitcode.Timeout)
                        }
                        if n := strings.Count(vbuf.String(), "DEBUG: command: "); n != 1 {
                            t.Errorf("Execute count = %d, want 1 (immediate rescue, NO retry)", n)
                        }
                        if got := headSHA(t, repo); got != before { t.Errorf("HEAD moved on a killed run: %s → %s", before, got) }
                        if c.want == "hard" {
                            if !strings.Contains(vbuf.String(), "DEBUG: cause: hard cap") {
                                t.Errorf("missing hard-cap cause line; buf:\n%s", vbuf.String())
                            }
                        } else if runtime.GOOS == "linux" {   // the landed S2 GOOS-split idiom (idle_parity_test.go:122)
                            if !strings.Contains(vbuf.String(), "DEBUG: cause: idle deadline") {
                                t.Errorf("linux: missing idle cause line (the layer bound the wrong deadline or lost precedence); buf:\n%s", vbuf.String())
                            }
                        } else if !strings.Contains(vbuf.String(), "DEBUG: cause: hard cap") {
                            t.Errorf("non-linux: missing hard-cap cause line (the 4s bound must terminate a D4-disarmed platform); buf:\n%s", vbuf.String())
                        }
                    }
                })
            }
        }
  - THE 16-ROW TABLE (exact values; every idle row carries fs.Set("timeout","4s") as the hard bound — the
    flag layer's HARD field is orthogonal to the idle-field precedence under test; losers 10/20/30s all ≫ 4s
    so a precedence bug fails with a WRONG CAUSE at 4s instead of hanging; SleepMS 5000 > every deadline):

    | # | name | setup (after scrub+isolate+chdir) | stub | wantIdle/Hard (message) | want | maxElapsed |
    |---|------|-----------------------------------|------|--------------------------|------|------------|
    | 1 | Defaults_BindAndSpare | none | {Out:"feat: quick"} | 60s / 600s | success | 10s |
    | 2 | FlagGlobal_IdleKills | fs.Set("idle-timeout","1s"); fs.Set("timeout","4s") | hung | 1s / 4s | idle | 6s |
    | 3 | FlagPerRoleIdle_BeatsEnvGitFile | fs.Set("message-idle-timeout","1s"); fs.Set("timeout","4s"); t.Setenv("STAGECOACH_MESSAGE_IDLE_TIMEOUT","10s"); ladderGitConfig(repo,"stagecoach.role.message.idleTimeout","20s"); ladderWriteRepoFile(repo, cfgV3+"\n[role.message]\nidle_timeout = \"30s\"\n") | hung | 1s / 4s | idle | 6s |
    | 4 | EnvPerRoleIdle_BeatsFile | t.Setenv("STAGECOACH_MESSAGE_IDLE_TIMEOUT","1s"); fs.Set("timeout","4s"); ladderWriteRepoFile(repo, cfgV3+"\n[role.message]\nidle_timeout = \"30s\"\n") | hung | 1s / 4s | idle | 6s |
    | 5 | EnvGlobal_IdleKills | t.Setenv("STAGECOACH_IDLE_TIMEOUT","1s"); fs.Set("timeout","4s") | hung | 1s / 4s | idle | 6s |
    | 6 | GitGlobal_IdleKills | ladderGitConfig(repo,"stagecoach.idleTimeout","1s")  [CAMELCASE]; fs.Set("timeout","4s") | hung | 1s / 4s | idle | 6s |
    | 7 | GitPerRole_IdleKills | ladderGitConfig(repo,"stagecoach.role.message.idleTimeout","1s"); fs.Set("timeout","4s") | hung | 1s / 4s | idle | 6s |
    | 8 | GitIdle_BeatsFile | ladderGitConfig(repo,"stagecoach.idleTimeout","1s"); ladderWriteRepoFile(repo, cfgV3+"\n[defaults]\nidle_timeout = \"30s\"\n"); fs.Set("timeout","4s") | hung | 1s / 4s | idle | 6s |
    | 9 | FileGlobal_IdleKills | ladderWriteRepoFile(repo, cfgV3+"\n[defaults]\nidle_timeout = \"1s\"\n"); fs.Set("timeout","4s") | hung | 1s / 4s | idle | 6s |
    | 10 | FilePerRole_IdleKills | ladderWriteRepoFile(repo, cfgV3+"\n[role.message]\nidle_timeout = \"1s\"\n"); fs.Set("timeout","4s") | hung | 1s / 4s | idle | 6s |
    | 11 | FilePerRoleIdle_BeatsGlobal | ladderWriteRepoFile(repo, cfgV3+"\n[defaults]\nidle_timeout = \"1s\"\n\n[role.message]\nidle_timeout = \"30s\"\n"); fs.Set("timeout","4s") | hung | 30s / 4s | hard | 8s |
    | 12 | FlagGlobalHardCap_Kills | fs.Set("timeout","1500ms") | hung | 60s / 1500ms | hard | 6s |
    | 13 | FilePerRoleHardCap_Kills | ladderWriteRepoFile(repo, cfgV3+"\n[role.message]\ntimeout = \"1500ms\"\n") | hung | 60s / 1500ms | hard | 6s |
    | 14 | EnvGlobalHardCap_Kills | t.Setenv("STAGECOACH_TIMEOUT","1500ms") | hung | 60s / 1500ms | hard | 6s |
    | 15 | GitGlobalHardCap_Kills | ladderGitConfig(repo,"stagecoach.timeout","1500ms"); | hung | 60s / 1500ms | hard | 6s |
    | 16 | PlannerIsolation_NoLeakToMessage | ladderWriteRepoFile(repo, cfgV3+"\n[role.planner]\nidle_timeout = \"1s\"\n"); fs.Set("timeout","4s") | hung | 60s / 4s (+wantRoles planner = 1s/4s) | hard | 8s |

    const cfgV3 = "config_version = 3\n"  // silences the FR-B4/B9 advisory; CurrentConfigVersion == 3
    hung = stubtest.Options{Out: "feat: never", SleepMS: 5000}  // sleeps silently past every deadline
    Row notes:
    - Row 1 is the contract's "unset ⇒ default 60s does NOT kill a short healthy run" + "600s default asserted
      via config": wantIdle/wantHard come from the LOADED cfg (Layer-1 defaults through Load, not Defaults()).
    - Rows 2/4 are the contract's literally-named cases ("--idle-timeout 1s kills a hung stub";
      "STAGECOACH_MESSAGE_IDLE_TIMEOUT env overrides the file"); row 3 is "--message-idle-timeout flag overrides
      all" (env+git+file all set against it); row 10 is "[role.message].idle_timeout file key ditto".
    - Row 8 pins FR34's unusual git>file ordering behaviorally. Row 11 is the INVERTED discriminator: per-role
      30s beat global 1s ⇒ the 4s bound is what kills (cause "hard cap" everywhere; if global wrongly won, Linux
      idle-kills at ~1s and the hard-cap assertion fails). Row 16 proves per-ROLE independence: planner's 1s must
      NOT leak into message (cfg-level assert is the platform-uniform proof; the cap-kill corroborates on Linux).
    - Rows 12-15 are the "hard-cap equivalents for --timeout": flag/file-per-role/env/git, each killing at ~1.5s
      with the "hard cap" cause on ALL platforms (idle default 60s ≫ 1500ms can never fire first).
  - Every t.Run body starts with its own `if testing.Short() { t.Skip(...) }` (subtests don't inherit skips).

Task 3: VERIFY — focused runs, determinism, full gates, scope
  - go build ./... ; go vet ./internal/generate/... ; gofmt -l internal/generate/idle_ladder_test.go (fix if listed)
  - go test ./internal/generate/ -run 'TestIdleLadder_EveryLayerBinds' -v        # all 16 rows PASS
  - go test ./internal/generate/ -count=2 -run 'TestIdleLadder'                  # timing confidence re-run
  - go test ./internal/generate/ -count=1                                       # regression net (S2/S3 rows green)
  - make test ; make lint                                                       # full race suite + golangci-lint
  - grep guards (see Validation Loop Level 4); git status --short (ONLY the new file)
```

### Implementation Patterns & Key Details

```go
// PATTERN (the row fixture — every idle row): the layer under test sets idle SMALL; the --timeout 4s flag is
// HARNESS SCAFFOLDING via the hard field (field-merge independence means it cannot disturb idle precedence);
// losers exceed the bound so precedence bugs fail fast with a wrong cause:
fs.Set("idle-timeout", "1s")   // or t.Setenv / ladderGitConfig / ladderWriteRepoFile per row
fs.Set("timeout", "4s")        // the bound: non-Linux terminates via cap; Linux precedence-bugs surface here

// PATTERN (the two-witness assert — the heart of S4): (a) the seam generate actually reads, platform-uniform;
// (b) the observable behavior. A seam break fails (a) everywhere; a wiring break fails (b) on Linux.
idle, hard := config.ResolveRoleDeadlines("message", *cfg)   // generate.go:304's exact call
if idle != c.wantIdle || hard != c.wantHard { t.Errorf(...) }

// PATTERN (the GOOS-split cause assertion — verbatim from landed idle_parity_test.go:118-130):
if runtime.GOOS == "linux" {
    // idle armed (D4): the layer that won names itself
    if !strings.Contains(vbuf.String(), "DEBUG: cause: idle deadline") { t.Errorf(...) }
} else if !strings.Contains(vbuf.String(), "DEBUG: cause: hard cap") {
    t.Errorf("... (D4-disarmed platform — the 4s bound must fire) ...")   // idle disarmed on darwin/windows
}

// PATTERN (the scrub-then-set order): neutralize ambient STAGECOACH_* BEFORE the row sets its own vars:
ladderScrubEnv(t)      // t.Setenv(k, "") for every ambient STAGECOACH_* — empty is the documented no-op
// …then the row's setup t.Setenv("STAGECOACH_MESSAGE_IDLE_TIMEOUT", "1s") etc.

// PATTERN (the Load call — the D4 of config tests): real layers, no bootstrap side effects:
cfg, err := config.Load(context.Background(), config.LoadOpts{
    RepoDir: repo,        // feeds the git layer (git -C repo config --get stagecoach.…)
    Flags:  fs,           // the flag layer (fs.Changed only)
    DisableBootstrap: true, // the documented TEST-ONLY seam: no FR-B3 write, no notice
})
```

### Integration Points

```yaml
GENERATE PACKAGE (internal/generate):
  - idle_ladder_test.go: NEW external-package file; 1 test (16 rows) + 6 helpers. Shares the package namespace
    with hooks_freeze_test.go / idle_parity_test.go / repro_freeze_test.go — reuses initTempRepo/stageChange/
    headSHA; adds only ladder*-prefixed + newLadderFlagSet symbols.

NO production change (internal/config, internal/generate, internal/provider, internal/stubtest, internal/exitcode,
  internal/ui, cmd/, pkg/ — ALL READ-ONLY; the ladder under observation is LANDED P1.M1/P1.M2). NO stub change
  (personas (a) hung + healthy are Options{Out,SleepMS} — landed since before S1's knobs). NO docs ("tests only").
  NO new deps (stdlib + existing internal imports + github.com/spf13/pflag v1.0.10).

CONSUMERS: none downstream — this CLOSES P1 (the last P1 work item). P2 (event-stream) and P3 (docs) are
  independent. P3.M1.T1.S2's "cross-file retirement grep" may count these tests' lines — nothing to coordinate.

FENCE: if ANY row fails, the defect is in the LANDED config surface (P1.M1 — e.g. a layer key/plumbing regression),
  the engine (P1.M2 — e.g. D4 arming), or the S1/S2 harness — surface it in the implementation report; do NOT
  modify production code or weaken the assertion from this tests-only task.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...                      # the new test file compiles alongside the tree
go vet ./internal/generate/...
gofmt -l internal/generate/idle_ladder_test.go
# Expected: clean / empty output. If gofmt lists the file: gofmt -w it.

# Scope guard: ONLY the one new test file.
git status --short
# Expected: ?? internal/generate/idle_ladder_test.go   (plus plan/ artifacts the workflow tracks)
#           and NOTHING else — no production file, no existing test file modified.
```

### Level 2: Unit Tests (Component Validation) — THE PRIMARY GATE

```bash
# The ladder, focused (all 16 rows; ~30s):
go test ./internal/generate/ -run 'TestIdleLadder_EveryLayerBinds' -v -count=1
# Expected: PASS — every row. Watch for the per-row ordering in -v output: seam assert passes, then behavior.

# Timing confidence (deadline tests are the flake-prone kind — two clean runs before trusting):
go test ./internal/generate/ -count=2 -run 'TestIdleLadder'

# -short sanity (rows skip; nothing hangs):
go test -short ./internal/generate/ -run 'TestIdleLadder' -v
# Expected: SKIP.

# The regression net (S2/S3 rows and every existing test untouched and green):
go test ./internal/generate/ ./internal/config/ -count=1
# Expected: green — esp. TestIdleKill_ExitCodeParity, TestHardCapHeartbeat_ExitCodeParity, TestInvariants,
#           TestLoad_* precedence tests (S4 observes the same code they pin).

# Full race suite + lint.
make test   # go test -race ./...
make lint   # golangci-lint run
# Expected: green / zero errors (no unused imports; helpers all used).
```

### Level 3: Integration Testing (System Validation)

```bash
# There is no further integration surface — the rows ARE the integration (real config.Load layer plumbing →
# real CommitStaged → real executor → real git on temp repos → exitcode.For). The e2e subprocess harness
# (internal/e2e) is NOT extended (library-level is the established S2/S3 surface for this proof).

# Optional manual confidence — watch one layer bind by hand through the real CLI (Linux):
d=$(mktemp -d) && cd "$d" && git init -q && git config user.email t@e.com && git config user.name T
echo hi > f.txt && git add f.txt
SC=$(go run .)   # or: go build -o /tmp/stagecoach . && SC=/tmp/stagecoach
git config stagecoach.idleTimeout 1s          # the camelCase git layer, by hand
$SC --provider <any-installed> --verbose 2>&1 | tail -3   # expect a rescue + "DEBUG: cause: idle deadline (1s …)"
git config --unset stagecoach.idleTimeout
cd - && rm -rf "$d"
# (Best-effort: requires an installed agent CLI; the table rows are the authoritative proof.)

# Platform note: the full idle discrimination runs on Linux (CI's Linux job carries it; on darwin the idle rows
# pass via the documented D4 degradation — bounded cap-kill with the swapped cause assertion):
go env GOOS
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: the test + its 16 rows exist, once each.
grep -c 'func TestIdleLadder_EveryLayerBinds' internal/generate/idle_ladder_test.go                       # 1
for r in Defaults_BindAndSpare FlagGlobal_IdleKills FlagPerRoleIdle_BeatsEnvGitFile EnvPerRoleIdle_BeatsFile \
         EnvGlobal_IdleKills GitGlobal_IdleKills GitPerRole_IdleKills GitIdle_BeatsFile FileGlobal_IdleKills \
         FilePerRole_IdleKills FilePerRoleIdle_BeatsGlobal FlagGlobalHardCap_Kills FilePerRoleHardCap_Kills \
         EnvGlobalHardCap_Kills GitGlobalHardCap_Kills PlannerIsolation_NoLeakToMessage; do
  grep -q "\"$r\"" internal/generate/idle_ladder_test.go || echo "MISSING ROW: $r"; done
# Expected: no MISSING ROW lines.

# Guard 2: the contract's literally-named spellings are exercised (camelCase git keys; exact env vars; flags).
grep -c 'stagecoach.idleTimeout' internal/generate/idle_ladder_test.go                                     # ≥1 (camelCase!)
grep -c 'stagecoach.role.message.idleTimeout' internal/generate/idle_ladder_test.go                       # ≥1
grep -c 'STAGECOACH_MESSAGE_IDLE_TIMEOUT' internal/generate/idle_ladder_test.go                           # ≥1
grep -c '"message-idle-timeout"' internal/generate/idle_ladder_test.go                                    # ≥1
grep -c '"idle-timeout"' internal/generate/idle_ladder_test.go                                            # ≥1
grep -c 'STAGECOACH_IDLE_TIMEOUT\|STAGECOACH_TIMEOUT' internal/generate/idle_ladder_test.go               # ≥1 each
! grep -q 'stagecoach.idle_timeout' internal/generate/idle_ladder_test.go && echo "OK: no snake_case git key"

# Guard 3: the D4 platform split + the bound exist (S2's landed idiom, not skips).
grep -c 'runtime.GOOS == "linux"' internal/generate/idle_ladder_test.go                                   # ≥1
grep -c '"timeout", "4s"' internal/generate/idle_ladder_test.go                                           # ≥10 (every idle row's bound)
grep -c 'DisableBootstrap: true' internal/generate/idle_ladder_test.go                                    # 1

# Guard 4: hermeticity mechanics present.
grep -c 'XDG_CONFIG_HOME' internal/generate/idle_ladder_test.go                                           # ≥1 (ladderIsolateHome)
grep -c 'STAGECOACH_' internal/generate/idle_ladder_test.go | head -1                                     # scrub present (ladderScrubEnv)
grep -c 'config_version = 3' internal/generate/idle_ladder_test.go                                        # ≥1 (advisory silenced)
grep -c 't.Parallel' internal/generate/idle_ladder_test.go                                                # 0 (chdir + t.Setenv forbid it)

# Guard 5: the two-witness pattern (seam assert + behavior) and parity asserts.
grep -c 'config.ResolveRoleDeadlines' internal/generate/idle_ladder_test.go                               # ≥2
grep -c 'exitcode.For(err)' internal/generate/idle_ladder_test.go                                         # 1
grep -c 'DEBUG: cause: idle deadline\|DEBUG: cause: hard cap' internal/generate/idle_ladder_test.go       # ≥2 each

# Guard 6: no shared-helper redefinition (the external package owns initTempRepo/stageChange/headSHA).
for h in 'func initTempRepo' 'func stageChange' 'func headSHA'; do
  grep -q "$h(" internal/generate/idle_ladder_test.go && echo "REDEFINED: $h"; done
# Expected: no output.

# Guard 7: scope — no production file touched, no existing test modified.
git status --porcelain | grep -v '^?? plan/' | grep -v 'idle_ladder_test.go'
# Expected: EMPTY.
```

## Final Validation Checklist

### Technical Validation
- [ ] Level 1-4 all pass; `go vet ./internal/generate/...` clean; `gofmt -l` empty on the new file
- [ ] `go test ./internal/generate/ -run TestIdleLadder -count=1` green (16/16 rows) and `-count=2` clean
- [ ] `make test` (race) + `make lint` green; `-short` run skips all rows

### Feature Validation (FR-R7's ladder, observably)
- [ ] **Default layer**: loaded cfg = 60s idle / 600s hard; healthy run NOT killed; commit created (row 1)
- [ ] **Flag layer**: `--idle-timeout 1s` kills a hung stub (row 2); `--message-idle-timeout` beats env+git+file
      (row 3); `--timeout 1500ms` hard-cap kills on all platforms (row 12)
- [ ] **Env layer**: `STAGECOACH_MESSAGE_IDLE_TIMEOUT=1s` overrides the file key (row 4);
      `STAGECOACH_IDLE_TIMEOUT=1s` binds (row 5); `STAGECOACH_TIMEOUT=1500ms` hard-cap kills (row 14)
- [ ] **Git layer**: camelCase `stagecoach.idleTimeout` (row 6) + `stagecoach.role.message.idleTimeout` (row 7)
      bind via the git-config harness; git beats file (row 8); `stagecoach.timeout` hard-cap kills (row 15)
- [ ] **File layer**: `[defaults].idle_timeout="1s"` kills (row 9); `[role.message].idle_timeout="1s"` kills
      (row 10); per-role beats `[defaults]` — the inverted discriminator (row 11);
      `[role.message].timeout="1500ms"` hard-cap kills (row 13)
- [ ] **Per-role independence**: `[role.planner].idle_timeout` does not leak into message (row 16)
- [ ] Every kill row: `*RescueError` + `ErrTimeout` + exit 124 + Execute-count 1 + HEAD unchanged + bounded
      elapsed; the D4 GOOS-split cause assertion present and never a skip
- [ ] `git status` shows ONLY internal/generate/idle_ladder_test.go

### Code Quality & Scope Validation
- [ ] Reuses initTempRepo/stageChange/headSHA; only ladder*/newLadderFlagSet symbols added; no t.Parallel
- [ ] Every written TOML carries `config_version = 3`; every git key camelCase; every duration a string
- [ ] Comments cite FR-R7/FR34/§9.8/§9.15/D4 and name the row's layer (the package's comment style)
- [ ] Stdlib + existing internal imports + pflag only; no new deps; no new files beyond the one

---

## Anti-Patterns to Avoid

- ❌ Don't set `cfg.IdleTimeout`/`cfg.Timeout` directly. That's S2/S3's (already-landed) territory; S4's entire
  value is driving `config.Load` with REAL layer sources. If you find yourself mutating the loaded cfg, stop —
  the row is wrong.
- ❌ Don't use `stagecoach.idle_timeout` (snake_case) in git config — git rejects it ("invalid key") and the
  row setup fails. camelCase final segment only (`stagecoach.idleTimeout`), even though TOML/env spell it
  idle_timeout / _IDLE_TIMEOUT.
- ❌ Don't write bare ints into `[defaults].idle_timeout` ("1") — that key parses with bare time.ParseDuration
  and rejects them; use duration strings ("1s") everywhere so all layers parse identically.
- ❌ Don't omit the `--timeout 4s` bound from idle rows. On darwin/windows the D4 rule disarms the idle watcher
  (print-mode stub) and the row would run to the stub's 5s answer — a green-looking FAILURE. The bound makes
  every platform terminate killed; the GOOS-split cause assertion keeps the Linux discrimination honest.
- ❌ Don't skip rows on non-Linux (`t.Skip` on GOOS). The landed idiom (idle_parity_test.go:122) is
  platform-UNIFORM kills with a GOOS-split CAUSE assertion — bounded, green, and discriminating where it can be.
- ❌ Don't omit `DisableBootstrap: true`. Without it Load bootstraps a global config into your temp XDG home and
  prints a notice — an unneeded FS effect and noise on every row.
- ❌ Don't forget the env scrub, or order it after setup. Ambient `STAGECOACH_*` from a developer shell silently
  mutates every row; scrub FIRST (t.Setenv to "" — empty is the documented no-op), then set the row's own vars.
- ❌ Don't chdir via `t.Chdir` (go 1.22 — doesn't exist) and don't forget chdir entirely (the repo-local
  .stagecoach.toml resolves from process CWD, not RepoDir). Use the os.Chdir + t.Cleanup helper.
- ❌ Don't call `t.Parallel()` anywhere in the file — t.Setenv panics and chdir is process-global.
- ❌ Don't redefine `initTempRepo`/`stageChange`/`headSHA` — they're already in the shared generate_test package;
  redefinition is a compile error, and "fixing" it by renaming the shared helpers breaks S2/S3's files.
- ❌ Don't register a partial flag set. Copy newFlagSet's FULL registration; loadFlags probes many flags via
  fs.Changed and a missing registration silently changes layer behavior.
- ❌ Don't rely on `config.Defaults()` for the 600s/60s assertions — assert against the LOADED cfg (the default
  layer must bind through Load, which is the claim).
- ❌ Don't assert `re.Cause.Error()` contains "hard" for cap kills — the naming lives ONLY in the verbose
  `DEBUG: cause: hard cap (…)` line (S3's verified contract). Assert the verbose buffer.
- ❌ Don't drop the seam assert (ResolveRoleDeadlines) in favor of behavior alone — it's the platform-uniform
  witness that catches layer plumbing bugs even where behavior can't discriminate (e.g. row 16 on darwin).
- ❌ Don't touch production code if a row fails. The defect is in the LANDED config surface (P1.M1), engine
  (P1.M2), or harness (S1/S2/S3) — surface it in the report; patching around it from a tests-only task is how
  observability silently rots.
- ❌ Don't skip the `-short` guards or exceed the row bounds — ~30s of suite time is acceptable, unbounded
  hangs are not (every row is doubly bounded: the 4s/1.5s deadline AND the SleepMS-5000 stub self-termination).

---

## Confidence Score: 9/10

Every fact the PRP asserts was verified at source: the Load surface (LoadOpts + the DisableBootstrap test seam,
CWD-relative repo-file discovery, per-layer spellings and quirks), the seam the pipeline reads
(ResolveRoleDeadlines at roles.go:78-87 → generate.go:304), the D4 arming rule and the landed GOOS-split cause
idiom (idle_parity_test.go:118-130), the shared external-package helpers (initTempRepo/stageChange/headSHA at
their file:lines), the newFlagSet registration to copy, defaults 60s/600s and CurrentConfigVersion=3, go 1.22
(no t.Chdir), and the existing value-level tests this task must not duplicate. The design reuses the S2 harness
and its proven assertion ladder; every row is doubly bounded (deadline + self-terminating stub) so no failure
mode hangs. The -1: (a) timing rows on pathologically loaded CI runners can flake (1s idle vs the 500ms poll
leaves ~2× margin; `-count=2` is specified to catch it), and (b) S3 lands concurrently — verified present at
idle_parity_test.go:228 already, but if its final shape renames anything in the shared package the implementer
must re-grep helper names before defining new ones (Guard 6 covers this).
