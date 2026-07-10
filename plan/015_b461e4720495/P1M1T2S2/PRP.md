name: "P1.M1.T2.S2 — Add 4 --<role>-timeout CLI flags + flag-loop parsing (FR-R7 flag layer)"
description: >
  Add 4 per-role generation-timeout CLI flags to internal/cmd/root.go (4 package-level `string` flag
  vars `flag{Planner,Stager,Message,Arbiter}Timeout` + 4 `pf.StringVar(...)` registrations after the
  per-role reasoning block) and a `-timeout` branch to the per-role flag loop in internal/config/load.go
  `loadFlags` (after the `-reasoning` branch) that mirrors the global `--timeout` flag handling EXACTLY:
  `fs.Changed(role+"-timeout") → fs.GetString → parseTimeout → cfg.setRoleTimeout(role, d)`. Because
  `loadFlags` has NO error return, a malformed value is SILENTLY ignored (same as the global `--timeout`
  flag) — NOT the error-returning env pattern from S1. Plus: extend the `newFlagSet` test helper to
  register per-role `-timeout` flags + 3 unit tests. Plus docs/cli.md: 4 rows in the main flags table
  + 4 rows in the env/git mapping table. CONSUMES `setRoleTimeout` from P1.M1.T2.S1 (already landed at
  load.go:66-78). Does NOT touch setRoleTimeout itself, the env branch (S1), git-config reading (S3),
  ResolveRoleTimeout/defaultRoleTimeouts (P1.M2.T1), the 480s→120s default change (P1.M2.T2), the 13
  Execute call sites (P1.M3), or broader docs (P1.M4.T2.S1).

---

## Goal

**Feature Goal**: Wire the CLI-FLAG layer (PRD §16.1 layer 7, highest precedence) of per-role generation
timeouts (PRD §9.15 FR-R7, §15.2) so `stagecoach --planner-timeout 600s` (and stager/message/arbiter)
is registered as a pflag, parsed via `parseTimeout` ("600s" OR bare "600"), and stored into
`cfg.Roles[role].Timeout` via `setRoleTimeout` — using the EXACT `Changed→GetString→parseTimeout→set`
discipline the global `--timeout` flag already uses, and the EXACT loop-placement the existing
`-provider`/`-model`/`-reasoning` per-role flag branches use.

**Deliverable**:
1. **internal/cmd/root.go** — (a) 4 new package-level `string` vars `flagPlannerTimeout`,
   `flagStagerTimeout`, `flagMessageTimeout`, `flagArbiterTimeout` in the per-role `var (...)` block
   (after `flagArbiterReasoning`); (b) 4 `pf.StringVar(&flag<Role>Timeout, "<role>-timeout", "", "...")`
   registrations in `init()` after the per-role reasoning block (after `flagArbiterReasoning`, before
   the `// --version is auto-added` comment).
2. **internal/config/load.go** — a new `if fs.Changed(role + "-timeout") { ... }` branch in the
   `loadFlags` per-role loop (after the `-reasoning` branch) that `parseTimeout`s the value and calls
   `cfg.setRoleTimeout(role, d)`.
3. **internal/config/load_test.go** — (a) extend `newFlagSet` to register per-role `-timeout` flags;
   (b) 3 tests: per-role flag parsing (duration + bare-int + field-merge), a flag-beats-env precedence
   test, and a malformed-value silent-ignore test.
4. **docs/cli.md** — (a) 4 `--<role>-timeout` rows in the main global-flags table; (b) 4 rows in the
   flag→env→git mapping table.

**Success Definition**:
- `stagecoach --planner-timeout 600s` (via a Load with `Flags` set) → `cfg.Roles["planner"].Timeout == 600*time.Second`.
- `--stager-timeout 300` (bare int) → `cfg.Roles["stager"].Timeout == 300*time.Second` (proves
  `parseTimeout`, not `time.ParseDuration`).
- `--planner-timeout 600s` does NOT clobber `--planner-provider` on the same role (FR-R3 field-merge —
  both survive in `cfg.Roles["planner"]`).
- `--planner-timeout` BEATS `STAGECOACH_PLANNER_TIMEOUT` env (flag layer 7 > env layer 5).
- `--planner-timeout abc` (malformed) is silently ignored (loadFlags has no error return; mirrors the
  global `--timeout` flag); `cfg.Roles["planner"].Timeout` stays at whatever the lower layers set (or 0).
- A role with NO `--<role>-timeout` flag is untouched (`fs.Changed` is false → skipped).
- `go build ./...`, `go vet ./internal/cmd/... ./internal/config/...`, `gofmt -l`, `make lint`, `make test`
  all pass.

## User Persona (if applicable)

**Target User**: A developer/CI author who wants to give a specific role a longer/shorter generation
budget for a single invocation via a CLI flag — e.g. `stagecoach --planner-timeout 600s` for a
large-diff planning run, without exporting an env var or editing config.

**Use Case**: Multi-commit decomposition: the planner reasons over the whole diff and may need more
time than the default; `--planner-timeout 600s` (while leaving the global `--timeout`) gives the planner
its own budget for this run only. The flag is the per-invocation, highest-precedence source.

**User Journey**: `stagecoach --planner-timeout 600s` → `loadFlags` reads it via `fs.Changed` →
`parseTimeout` → `cfg.setRoleTimeout("planner", 600s)` → (after P1.M2.T1/P1.M3 land) the planner's
`provider.Execute` call uses 600s while other roles use the global. (This subtask delivers flag
parse+store; consumption is downstream.)

**Pain Points Addressed**: Today the only per-role timeout source is env (S1, just landed). There is no
`--<role>-timeout` flag (grep-confirmed — root.go registers `-provider`/`-model`/`-reasoning` per role,
but no `-timeout`). A per-invocation flag is the documented highest-precedence override (§16.1 layer 7)
and the most discoverable (shows in `--help`). This task adds it.

## Why

- **FR-R7 / §9.15 / §16.1 layer 7 / §15.2**: Per-role timeouts resolve across the 7-layer precedence;
  the CLI-flag layer (7) is the HIGHEST, beating env (5), file (3), and git-config (4). S1 made
  `setRoleTimeout` + the env branch; this task adds the flag source (the top of the stack).
- **Why a tiny, mechanical subtask**: the `loadFlags` per-role loop (load.go:452-470) already reads
  three per-role flags via three branches; adding a fourth (`-timeout` + `setRoleTimeout`) is a 1:1
  extension of the proven pattern. The global `--timeout` flag (load.go:433-438) is the EXACT
  `Changed→GetString→parseTimeout→set` template. `parseTimeout` + `setRoleTimeout` already exist.
- **Complementary, non-overlapping**: S1 owns the env layer (setRoleTimeout + env `_TIMEOUT` branch —
  LANDED); S3 owns git-config reading (planned). THIS task owns the flag layer. Resolution
  (P1.M2.T1), the default change (P1.M2.T2), the 13 call sites (P1.M3), and broader docs (P1.M4.T2.S1)
  are all fenced out.

## What

**User-visible behavior**: `--planner-timeout <dur>` (and stager/message/arbiter) appears in
`stagecoach --help` and, when passed, overrides only that role's generation timeout for this
invocation. Combined with the rest of FR-R7, it budgets that role independently. This subtask's
observable effect is at the unit-test level (loadFlags direct call → assert `cfg.Roles[role].Timeout`)
plus the `--help` rows; actual generation consumption lands with P1.M2.T1/P1.M3.

**Technical change (four small additions + tests + docs):**
1. 4 flag vars + 4 registrations in root.go (string flag, zero default — `Changed` reflects "user passed it").
2. The `loadFlags` per-role `-timeout` branch — `fs.Changed` gating, `parseTimeout` (accepts `"600s"`
   and bare `"600"`), SILENT ignore on bad value (loadFlags has no error return), DIRECT-set via
   `cfg.setRoleTimeout(role, d)`.
3. `newFlagSet` extended + 3 tests.
4. docs/cli.md — 4+4 rows.

### Success Criteria
- [ ] 4 `flag<Role>Timeout string` vars exist in root.go's per-role `var (...)` block.
- [ ] 4 `pf.StringVar(&flag<Role>Timeout, "<role>-timeout", "", ...)` registrations exist in `init()`.
- [ ] The `loadFlags` per-role loop has a `-timeout` branch after the `-reasoning` branch.
- [ ] `--planner-timeout 600s` → `cfg.Roles["planner"].Timeout == 600*time.Second`.
- [ ] `--stager-timeout 300` (bare int) → `300*time.Second` (proves parseTimeout used).
- [ ] `--planner-timeout` beats `STAGECOACH_PLANNER_TIMEOUT` env (flag > env precedence).
- [ ] `--planner-timeout abc` is silently ignored (no error; mirrors global `--timeout`).
- [ ] `--planner-timeout` + `--planner-provider` on the same role both survive (FR-R3 field-merge).
- [ ] `newFlagSet` registers per-role `-timeout` flags (so tests can `fs.Set` them).
- [ ] docs/cli.md main table + mapping table each have 4 new `--<role>-timeout` rows.
- [ ] `go build ./...`, `go vet ./internal/cmd/... ./internal/config/...`, `gofmt -l`, `make lint`, `make test` pass.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim flag-loop branch to add (with the silent-ignore rationale), the verbatim root.go
vars + registrations (with exact help text from the contract), the `newFlagSet` edit, the 3 tests to
clone with current line numbers, the docs rows with exact column values, the prerequisite
(`setRoleTimeout` already landed), and the scope fences against 6 sibling subtasks are all enumerated below.

### Documentation & References

```yaml
- file: internal/cmd/root.go
  why: "THE flag-registration change site. Per-role flag VARS block @66-81 (ends with flagArbiterReasoning
        string — INSERT the 4 flag<Role>Timeout vars after it, before the closing paren). Flag
        REGISTRATIONS in init(): the per-role reasoning block ENDS at flagArbiterReasoning @251-253 —
        INSERT the 4 pf.StringVar(...) calls after it, before the '// --version is auto-added' comment.
        Global --timeout registration @165 (the string-flag + zero-default discipline to mirror)."
  pattern: >
    // the var block (INSERT after flagArbiterReasoning string):
    	flagPlannerTimeout   string
    	flagStagerTimeout    string
    	flagMessageTimeout   string
    	flagArbiterTimeout   string
    // the registration (INSERT after the flagArbiterReasoning StringVar):
    	pf.StringVar(&flagPlannerTimeout, "planner-timeout", "",
    		"Per-role generation timeout for the decomposition planner (env STAGECOACH_PLANNER_TIMEOUT; git stagecoach.role.planner.timeout)")
    	// ... stager, message, arbiter analogs
  critical: "flag vars are STRING (zero default \"\"), same as flagPlannerProvider etc. — NOT time.Duration.
    The &flagVar address is the var's use (satisfies the `unused` linter; loadFlags reads via fs.Changed,
    never the var directly — exactly as flagProvider/flagModel do). The help text is GIVEN VERBATIM in
    the contract; copy it (it mentions env + the forward-looking git key, mirroring the existing
    --planner-provider help which mentions 'git stagecoach.role.planner')."

- file: internal/config/load.go
  why: "THE flag-loop change site + the parse helper + the dependency. loadFlags per-role loop @452-470
        (INSERT the -timeout branch after the -reasoning branch @463-467, before the loop's closing brace).
        The global --timeout flag handling @433-438 is the EXACT Changed→GetString→parseTimeout→set template
        to mirror. setRoleTimeout @66-78 (the dependency from S1 — ALREADY LANDED; consume, do NOT re-add).
        parseTimeout @640 (reuse — accepts '600s' AND bare '600'). roleNames @19 (the loop source)."
  pattern: >
    // the global --timeout template to mirror (@433-438):
    	if fs.Changed("timeout") {
    		if v, err := fs.GetString("timeout"); err == nil {
    			if d, perr := parseTimeout(v); perr == nil {
    				cfg.Timeout = d
    			}
    		}
    	}
    // the per-role -timeout branch (INSERT inside the per-role loop after the -reasoning branch):
    	if fs.Changed(role + "-timeout") {
    		if v, err := fs.GetString(role + "-timeout"); err == nil {
    			if d, perr := parseTimeout(v); perr == nil {
    				cfg.setRoleTimeout(role, d)
    			}
    		}
    	}
  critical: "loadFlags has NO error return (func loadFlags(cfg *Config, fs *pflag.FlagSet)). So a malformed
    --planner-timeout abc is SILENTLY IGNORED (the perr==nil guard skips the set) — IDENTICAL to the
    global --timeout flag. Do NOT copy the env branch's `return fmt.Errorf(...)` — it will not compile.
    The ONLY differences from the global --timeout template: name is role+\"-timeout\", set is
    cfg.setRoleTimeout(role, d) instead of cfg.Timeout = d. parseTimeout, pflag (fs) — both already imported."

- file: internal/config/load_test.go
  why: "newFlagSet @53-77 (the test flagset — MUST be extended to register per-role -timeout flags, else
        fs.Set('planner-timeout',...) errors and fs.Changed returns false). TestLoadFlags_PerRole @500
        (the per-role flag test to clone). TestLoadFlags_TimeoutString @486 (the global --timeout flag
        test to clone — proves the duration form). TestLoad_PerRoleFlagBeatsEnv @1061 (the flag>env
        precedence test to clone — full Load path with loadEnvSetup + chdir)."
  pattern: >
    // newFlagSet per-role loop (ADD one line):
    	for _, role := range roleNames {
    		fs.String(role+"-provider", "", "")
    		fs.String(role+"-model", "", "")
    		fs.String(role+"-timeout", "", "")   // ← ADD
    	}
    // test skeleton (clone of TestLoadFlags_PerRole):
    	cfg := Defaults(); fs := newFlagSet(t)
    	if err := fs.Set("planner-timeout", "600s"); err != nil { t.Fatal(err) }
    	loadFlags(&cfg, fs)
    	if rc := cfg.Roles["planner"]; rc.Timeout != 600*time.Second { t.Errorf(...) }
  critical: "Without the newFlagSet edit, fs.Set('planner-timeout', ...) returns 'unknown flag' error.
    Adding the registration is SAFE for all existing tests (un-set flags: Changed==false, no-op). Do NOT
    add -reasoning to newFlagSet (pre-existing gap, out of scope). Tests read cfg.Roles[role].Timeout
    (the PER-ROLE field), NOT cfg.Timeout (the global). Use fs.Set (sets Changed=true). For the
    flag-beats-env test use the full Load() path (loadEnvSetup + t.Setenv + fs), mirroring TestLoad_PerRoleFlagBeatsEnv."

- file: docs/cli.md
  why: "TWO tables to update. Main global-flags table: per-role rows @47-59 — INSERT 4 --<role>-timeout
        rows after --arbiter-reasoning @59, before --version @60. Columns: | flag | type | default | env |
        git-config | description |. Env/git mapping table: per-role rows ~@433-443 — INSERT 4 rows after
        --arbiter-reasoning @443. Columns: | flag | env | git-config |."
  pattern: >
    // MAIN table row (INSERT after --arbiter-reasoning row, before --version):
    | `--planner-timeout <dur>` | string | "" | `STAGECOACH_PLANNER_TIMEOUT` | — | Per-role generation timeout for the planner (e.g. `"600s"` or `600`) |
    // (stager/message/arbiter analogs)
    // MAPPING table row (INSERT after --arbiter-reasoning row):
    | `--planner-timeout` | `STAGECOACH_PLANNER_TIMEOUT` | — |
    // (stager/message/arbiter analogs)
  critical: "Use '—' (em-dash) in the git-config column — git.go does NOT read stagecoach.role.<role>.timeout
    today (grep-confirmed; per-role git reading is P1.M1.T2.S3). This matches EVERY existing per-role row
    (provider/model/reasoning all use —). The env column IS populated (STAGECOACH_<ROLE>_TIMEOUT shipped
    in S1). DOCS SCOPE = docs/cli.md ONLY (README/how-it-works/configuration are P1.M4.T2.S1)."

- docfile: plan/015_b461e4720495/architecture/research_role_config.md
  why: "§4 (CLI flag registration — internal/cmd/root.go) specifies this task: 'There are NO flagXxxTimeout
        vars — FR-R7 adds 4 (flagPlannerTimeout, etc.)' and 'FR-R7's --<role>-timeout flags should follow
        the SAME string-flag + parseTimeout pattern (consistent error messages).'"
  section: "4. CLI flag registration — internal/cmd/root.go"

- docfile: plan/015_b461e4720495/P1M1T2S1/PRP.md
  why: "S1 is the CONTRACT for the dependency: it produces setRoleTimeout(role string, d time.Duration)
        at load.go:66-78 (ALREADY LANDED in the working tree — confirmed by reading the file). This task
        CONSUMES setRoleTimeout in the loadFlags branch. Read it to confirm the helper signature + the
        env-branch precedent (which differs from this task ONLY in error handling: env returns an error,
        flag silently ignores)."

- docfile: plan/015_b461e4720495/P1M1T1S1/PRP.md
  why: "S1 (grandparent) produced RoleConfig.Timeout time.Duration (config.go:42) — the field setRoleTimeout
        writes and this task's flag branch ultimately populates. ALREADY LANDED. Consumed, not modified."
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/
  root.go          # per-role flag VARS @66-81 ← ADD 4 flag<Role>Timeout vars;
                   # init() per-role reasoning block ENDS @251-253 ← ADD 4 pf.StringVar registrations;
                   # global --timeout registration @165 (the string-flag discipline to mirror)
internal/config/
  load.go          # setRoleTimeout @66-78 (S1 — LANDED; consume); loadFlags per-role loop @452-470
                   # ← ADD -timeout branch after -reasoning @463-467; global --timeout @433-438 (mirror); parseTimeout @640
  load_test.go     # newFlagSet @53-77 ← ADD role+"-timeout" registration; TestLoadFlags_PerRole @500;
                   # TestLoadFlags_TimeoutString @486; TestLoad_PerRoleFlagBeatsEnv @1061 ← ADD 3 tests
  config.go        # RoleConfig.Timeout time.Duration (S1 grandparent — LANDED; consumed, not modified)
docs/
  cli.md           # main flags table per-role rows @47-59 ← ADD 4 rows; env/git mapping table @433-443 ← ADD 4 rows
```

### Desired Codebase tree with files to be added/modified

```bash
internal/cmd/root.go           # MODIFY: +4 flag vars +4 StringVar registrations
internal/config/load.go        # MODIFY: +1 flag-loop branch (the -timeout branch in loadFlags)
internal/config/load_test.go   # MODIFY: +1 line in newFlagSet +3 tests
docs/cli.md                    # MODIFY: +4 rows main table +4 rows mapping table
# (no new files; no struct changes; no other package touched)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (loadFlags has NO error return — SILENT ignore, NOT error): func loadFlags(cfg *Config, fs *pflag.FlagSet)
//   has no error return. So the per-role -timeout branch MUST mirror the global --timeout flag
//   (@433-438): Changed→GetString→parseTimeout→set, with the parseTimeout error GUARDED by `if perr == nil`
//   (a malformed value is silently skipped). Do NOT copy S1's env branch, which does
//   `return fmt.Errorf("%s_TIMEOUT: %w", prefix, err)` — that pattern WILL NOT COMPILE in loadFlags.
//   This env-vs-flag asymmetry is INTENTIONAL and matches the global --timeout / --provider / --model flags.

// CRITICAL (use parseTimeout, NOT time.ParseDuration): parseTimeout (load.go:640) accepts "600s"/"2m"
//   (time.ParseDuration) AND bare "600" (strconv.Atoi seconds). time.ParseDuration rejects bare ints.
//   The global --timeout flag, STAGECOACH_TIMEOUT env, and stagecoach.timeout git ALL use parseTimeout —
//   the per-role flag must too. A test with the bare-int form (--stager-timeout 300 → 300s) is what
//   PROVES parseTimeout was used.

// CRITICAL (DIRECT-set via setRoleTimeout, not overlay): the flag layer (7) writes via
//   cfg.setRoleTimeout(role, d) (DIRECT), bypassing overlay(). This is correct — flag is the HIGHEST
//   layer, so a DIRECT set is the escape hatch (same reason the global --timeout DIRECT-sets cfg.Timeout
//   at load.go:436). Do NOT route through overlay (overlay is for file/git layer merges; env/flag DIRECT-set).

// CRITICAL (per-role field, NOT global): the branch sets cfg.Roles[role].Timeout (via setRoleTimeout),
//   NOT cfg.Timeout. They are DIFFERENT fields. The role→global fallback (Roles[role].Timeout==0 ⇒ use
//   cfg.Timeout) is P1.M2.T1's ResolveRoleTimeout — NOT this task. Tests must assert cfg.Roles[role].Timeout.

// CRITICAL (newFlagSet MUST register -timeout): the test helper newFlagSet (load_test.go:53) registers
//   per-role -provider/-model but NOT -timeout. Without adding `fs.String(role+"-timeout", "", "")` to
//   its loop, fs.Set("planner-timeout", ...) returns an 'unknown flag' error and fs.Changed is false.
//   ADD the one line. Safe for all existing tests (un-set flags are no-ops). Do NOT add -reasoning
//   (pre-existing gap, out of scope).

// CRITICAL (flag vars are STRING, zero default): flagPlannerTimeout etc. are `string` (NOT time.Duration),
//   default "". The &flagVar address is the var's ONLY use (loadFlags reads via fs.Changed/fs.GetString,
//   never the var directly — exactly as flagProvider/flagModel). This satisfies the `unused` linter.

// CRITICAL (depends on S1 — ALREADY MET): setRoleTimeout exists at load.go:66-78 (S1 landed in the
//   working tree — confirmed by reading the file). cfg.setRoleTimeout(role, d) compiles. CONFIRM with
//   grep before editing: `grep -n 'func (c \*Config) setRoleTimeout' internal/config/load.go`.

// COORDINATION (no conflict with S1): S1 edits loadEnv (setRoleTimeout helper + env _TIMEOUT branch).
//   THIS task edits loadFlags (the flag-loop -timeout branch). DIFFERENT functions, non-overlapping
//   line regions → clean merge. Anchor the load.go edit on the -reasoning branch TEXT inside loadFlags
//   (robust to S1's line drift). S1 does NOT touch root.go or docs/cli.md → no conflict there either.

// CRITICAL (docs git-config column = —): git.go does NOT read stagecoach.role.<role>.timeout today
//   (per-role git reading is P1.M1.T2.S3). Use — in the docs git-config column, matching EVERY existing
//   per-role row. The env column IS populated (STAGECOACH_<ROLE>_TIMEOUT shipped in S1). The flag help
//   text DOES mention the git key (per the contract) — this mirrors the existing --planner-provider help
//   which mentions 'git stagecoach.role.planner' (a forward-looking advisory; the docs table is the
//   conservative, accurate live reference).

// SCOPE: do NOT modify setRoleTimeout / the env branch (S1), git.go per-role reading (S3),
//   ResolveRoleTimeout/defaultRoleTimeouts (P1.M2.T1), the 480s→120s default change (P1.M2.T2), any of
//   the 13 Execute call sites (P1.M3), or README/how-it-works/configuration docs (P1.M4.T2.S1).
```

## Implementation Blueprint

### Data models and structure
None. No new types, no struct changes. Four new package-level `string` flag vars, four `pf.StringVar`
registrations, one new branch in the `loadFlags` per-role loop (reuses `setRoleTimeout` from S1 +
`parseTimeout`), one line in `newFlagSet`, three tests, eight docs rows. The `0`-duration "inherit"
sentinel is S1's concern; this task stores whatever `parseTimeout` returns (always a positive duration
for a valid flag value; a malformed value is silently skipped).

### Implementation Tasks (ordered by dependencies)

> **Prerequisite**: S1 (P1.M1.T2.S1) merged — `setRoleTimeout(role string, d time.Duration)` must exist.
> CONFIRM (it does): `grep -n 'func (c \*Config) setRoleTimeout' internal/config/load.go` → load.go:66-78.
> Then proceed.

```yaml
Task 1: MODIFY internal/cmd/root.go — add 4 per-role timeout flag VARS
  - LOCATE the per-role var block (search "flagPlannerProvider  string" — currently @69; the block
    closes with "flagArbiterReasoning string" @80 then ")".
  - INSERT immediately AFTER "flagArbiterReasoning string", BEFORE the block's closing ")":
    	flagPlannerTimeout   string
    	flagStagerTimeout    string
    	flagMessageTimeout   string
    	flagArbiterTimeout   string
  - VERIFY byte-identical alignment to the sibling vars (gofmt will align the column — run gofmt -w).
  - NAMING: flag<Role>Timeout (camelCase, matches flagPlannerProvider/flagPlannerReasoning).
  - DEPENDENCIES: none (the &flagVar address is used in Task 2).

Task 2: MODIFY internal/cmd/root.go — register 4 flags in init() after the reasoning block
  - LOCATE the per-role reasoning registration block: it ENDS with:
        pf.StringVar(&flagArbiterReasoning, "arbiter-reasoning", "",
            "Per-role reasoning override for the leftover arbiter (env STAGECOACH_ARBITER_REASONING; git stagecoach.role.arbiter)")
    (currently @251-253), immediately FOLLOWED by the comment "// --version is auto-added by cobra ...".
  - INSERT immediately AFTER the flagArbiterReasoning StringVar, BEFORE the "// --version" comment:
    	// §9.15 FR-R7 — per-role generation timeout flags (string, zero default; loadFlags reads via fs.Changed).
    	pf.StringVar(&flagPlannerTimeout, "planner-timeout", "",
    		"Per-role generation timeout for the decomposition planner, e.g. \"600s\" or 600 (env STAGECOACH_PLANNER_TIMEOUT; git stagecoach.role.planner.timeout)")
    	pf.StringVar(&flagStagerTimeout, "stager-timeout", "",
    		"Per-role generation timeout for the (tooled) staging agent (env STAGECOACH_STAGER_TIMEOUT; git stagecoach.role.stager.timeout)")
    	pf.StringVar(&flagMessageTimeout, "message-timeout", "",
    		"Per-role generation timeout for the message composer (env STAGECOACH_MESSAGE_TIMEOUT; git stagecoach.role.message.timeout)")
    	pf.StringVar(&flagArbiterTimeout, "arbiter-timeout", "",
    		"Per-role generation timeout for the leftover arbiter (env STAGECOACH_ARBITER_TIMEOUT; git stagecoach.role.arbiter.timeout)")
  - VERIFY: each flag name is "<role>-timeout", default "", help text mentions env + git key (mirrors
    the existing --planner-provider help convention). The contract gives the help text verbatim.
  - DEPENDENCIES: Task 1 (the flag vars must exist to take their address).

Task 3: MODIFY internal/config/load.go — add the -timeout branch to the loadFlags per-role loop
  - LOCATE loadFlags' per-role loop (search "for _, role := range roleNames" — the SECOND hit, inside
    loadFlags, currently @452). The loop body has -provider/-model/-reasoning branches.
  - FIND the -reasoning branch inside it:
        if fs.Changed(role + "-reasoning") {
            if v, err := fs.GetString(role + "-reasoning"); err == nil {
                cfg.setRoleReasoning(role, v)
            }
        }
  - INSERT immediately AFTER that block, BEFORE the loop's closing "}":
    	// §9.15 FR-R7 / §15.2 — per-role generation timeout via CLI flag (layer 7, DIRECT-set via
    	// setRoleTimeout). Mirrors the global --timeout flag handling above EXACTLY: Changed→GetString→
    	// parseTimeout→set. loadFlags has NO error return, so a malformed value is silently ignored
    	// (same as the global --timeout); parseTimeout accepts "600s" and bare "600".
    	if fs.Changed(role + "-timeout") {
    		if v, err := fs.GetString(role + "-timeout"); err == nil {
    			if d, perr := parseTimeout(v); perr == nil {
    				cfg.setRoleTimeout(role, d)
    			}
    		}
    	}
  - VERIFY it sets cfg.Roles[role].Timeout (via setRoleTimeout), NOT cfg.Timeout.
  - VERIFY the bad-value path is SILENT (no return fmt.Errorf — loadFlags can't return an error).
  - NO new imports: parseTimeout, pflag (fs) — both already imported in load.go.
  - DEPENDENCIES: S1 (setRoleTimeout must exist — it does @66-78).

Task 4: MODIFY internal/config/load_test.go — extend newFlagSet + add 3 tests
  - 4a. EXTEND newFlagSet: in its per-role loop (search 'for _, role := range roleNames' inside
        newFlagSet, currently @60-63), add after the role+"-model" line:
        	fs.String(role+"-timeout", "", "")
  - 4b. TEST A — TestLoadFlags_PerRoleTimeout (clone TestLoadFlags_PerRole @500):
        	cfg := Defaults()
        	fs := newFlagSet(t)
        	if err := fs.Set("planner-timeout", "600s"); err != nil { t.Fatal(err) }   // duration form
        	if err := fs.Set("stager-timeout", "300"); err != nil { t.Fatal(err) }     // bare-int form (proves parseTimeout)
        	if err := fs.Set("planner-provider", "agy"); err != nil { t.Fatal(err) }   // field-merge: same role, different field
        	loadFlags(&cfg, fs)
        	if rc := cfg.Roles["planner"]; rc.Timeout != 600*time.Second || rc.Provider != "agy" {
        		t.Errorf("Roles[planner]=%+v want Timeout=600s Provider=agy (field-merge)", rc)
        	}
        	if rc := cfg.Roles["stager"]; rc.Timeout != 300*time.Second {
        		t.Errorf("Roles[stager].Timeout=%v want 300s (bare int via parseTimeout)", rc.Timeout)
        	}
        	// unset role: message has no -timeout → absent or Timeout==0
        	if rc, ok := cfg.Roles["message"]; ok && rc.Timeout != 0 { t.Errorf("message timeout should be 0/absent, got %v", rc.Timeout) }
  - 4c. TEST B — TestLoadFlags_PerRoleTimeout_MalformedIgnored (proves silent-ignore, NOT error):
        	cfg := Defaults()
        	fs := newFlagSet(t)
        	if err := fs.Set("planner-timeout", "not-a-dur"); err != nil { t.Fatal(err) }
        	loadFlags(&cfg, fs) // MUST NOT panic / MUST NOT error (loadFlags returns nothing)
        	// malformed value silently ignored → planner.Timeout stays 0 (no lower layer set it)
        	if rc, ok := cfg.Roles["planner"]; ok && rc.Timeout != 0 {
        		t.Errorf("malformed --planner-timeout should be ignored, got Roles[planner].Timeout=%v", rc.Timeout)
        	}
  - 4d. TEST C — TestLoad_PerRoleTimeout_FlagBeatsEnv (clone TestLoad_PerRoleFlagBeatsEnv @1061 —
        full Load() path proving flag layer 7 > env layer 5):
        	_, repo, _ := loadEnvSetup(t)
        	chdir(t, repo)
        	t.Setenv("STAGECOACH_PLANNER_TIMEOUT", "480s")   // env layer 5
        	fs := newFlagSet(t)
        	if err := fs.Set("planner-timeout", "600s"); err != nil { t.Fatal(err) }  // flag layer 7 (wins)
        	cfg, err := Load(context.Background(), LoadOpts{RepoDir: repo, Flags: fs})
        	if err != nil { t.Fatalf("Load err=%v", err) }
        	if rc := cfg.Roles["planner"]; rc.Timeout != 600*time.Second {
        		t.Errorf("Roles[planner].Timeout=%v want 600s (flag > env)", rc.Timeout)
        	}
  - NAMING: Test{LoadFlags_PerRoleTimeout, LoadFlags_PerRoleTimeout_MalformedIgnored,
    Load_PerRoleTimeout_FlagBeatsEnv} — matches the file's Test<Area>_<Detail> convention. PLACE next to
    the mirrored tests (TestLoadFlags_PerRole / TestLoad_PerRoleFlagBeatsEnv).
  - USE fs.Set (sets Changed=true), time.Duration literals (600*time.Second). For TEST C use the full
    Load() path (loadEnvSetup + chdir + t.Setenv + fs), mirroring TestLoad_PerRoleFlagBeatsEnv.
  - DEPENDENCIES: Tasks 1-3 + 4a (newFlagSet must register -timeout for fs.Set to work).

Task 5: MODIFY docs/cli.md — add 4 rows to the main table + 4 rows to the mapping table
  - 5a. MAIN global-flags table: INSERT after the --arbiter-reasoning row (currently @59), before the
       --version row (currently @60):
        | `--planner-timeout <dur>` | string | "" | `STAGECOACH_PLANNER_TIMEOUT` | — | Per-role generation timeout for the planner (e.g. `"600s"` or `600`) |
        | `--stager-timeout <dur>` | string | "" | `STAGECOACH_STAGER_TIMEOUT` | — | Per-role generation timeout for the (tooled) staging agent (e.g. `"300s"` or `300`) |
        | `--message-timeout <dur>` | string | "" | `STAGECOACH_MESSAGE_TIMEOUT` | — | Per-role generation timeout for the message composer (e.g. `"120s"` or `120`) |
        | `--arbiter-timeout <dur>` | string | "" | `STAGECOACH_ARBITER_TIMEOUT` | — | Per-role generation timeout for the leftover arbiter (e.g. `"120s"` or `120`) |
  - 5b. ENV/GIT MAPPING table: INSERT after the --arbiter-reasoning row (currently @443):
        | `--planner-timeout` | `STAGECOACH_PLANNER_TIMEOUT` | — |
        | `--stager-timeout` | `STAGECOACH_STAGER_TIMEOUT` | — |
        | `--message-timeout` | `STAGECOACH_MESSAGE_TIMEOUT` | — |
        | `--arbiter-timeout` | `STAGECOACH_ARBITER_TIMEOUT` | — |
  - VERIFY: git-config column is "—" (em-dash) — git.go does NOT read stagecoach.role.<role>.timeout
    today (that's P1.M1.T2.S3). This matches every existing per-role row. env column IS populated.
  - DEPENDENCIES: none (docs ride with the work).

Task 6: VERIFY — build, vet, format, targeted tests, full suite, grep guards
  - go build ./...
  - go vet ./internal/cmd/... ./internal/config/...
  - gofmt -l internal/cmd/root.go internal/config/load.go internal/config/load_test.go
  - go test ./internal/config/... -run 'PerRoleTimeout|Timeout' -v
  - make test && make lint
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the loadFlags per-role -timeout branch (1:1 mirror of the global --timeout flag @433-438)
// Inside loadFlags' `for _, role := range roleNames { ... }`, after the -reasoning branch:
if fs.Changed(role + "-timeout") {                       // gate on "user passed it"
	if v, err := fs.GetString(role + "-timeout"); err == nil {
		if d, perr := parseTimeout(v); perr == nil {      // "600s" OR bare "600"; SILENT ignore on bad value
			cfg.setRoleTimeout(role, d)                   // DIRECT-set (flag layer 7, bypasses overlay)
		}
	}
}

// CONTRAST: the env branch (S1, loadEnv) RETURNS an error on bad value — do NOT copy that here:
//   (env)  if v, ok := os.LookupEnv(prefix+"_TIMEOUT"); ok && v != "" {
//             d, err := parseTimeout(v); if err != nil { return fmt.Errorf("%s_TIMEOUT: %w", prefix, err) }
//             cfg.setRoleTimeout(role, d) }
//   loadFlags has NO error return → the flag layer SILENTLY ignores a malformed value, exactly like
//   the global --timeout flag. This env-vs-flag asymmetry is intentional and consistent.

// PATTERN: the root.go flag registration (mirrors --planner-provider @184)
pf.StringVar(&flagPlannerTimeout, "planner-timeout", "",
	"Per-role generation timeout for the decomposition planner, e.g. \"600s\" or 600 (env STAGECOACH_PLANNER_TIMEOUT; git stagecoach.role.planner.timeout)")

// PATTERN: the flag parsing test (clone of TestLoadFlags_PerRole @500)
func TestLoadFlags_PerRoleTimeout(t *testing.T) {
	cfg := Defaults()
	fs := newFlagSet(t)
	if err := fs.Set("planner-timeout", "600s"); err != nil { t.Fatal(err) }
	if err := fs.Set("stager-timeout", "300"); err != nil { t.Fatal(err) } // bare int → parseTimeout
	if err := fs.Set("planner-provider", "agy"); err != nil { t.Fatal(err) } // field-merge
	loadFlags(&cfg, fs)
	if rc := cfg.Roles["planner"]; rc.Timeout != 600*time.Second || rc.Provider != "agy" {
		t.Errorf("Roles[planner]=%+v want Timeout=600s Provider=agy", rc)
	}
	if rc := cfg.Roles["stager"]; rc.Timeout != 300*time.Second {
		t.Errorf("Roles[stager].Timeout=%v want 300s", rc.Timeout)
	}
}
```

### Integration Points

```yaml
NO database / routes / public-API / struct changes. Four flag vars + four registrations + one flag-loop
branch + one newFlagSet line + three tests + eight docs rows.

CLI (internal/cmd/root.go):
  - +4 vars: flag{Planner,Stager,Message,Arbiter}Timeout string
  - +4 registrations: pf.StringVar(&flag<Role>Timeout, "<role>-timeout", "", "...")

LOAD PATH (internal/config/load.go):
  - +1 branch in loadFlags' per-role loop: fs.Changed(role+"-timeout") → parseTimeout → cfg.setRoleTimeout(role, d)

CONSUMED (from S1, already landed):
  - setRoleTimeout(role string, d time.Duration) (load.go:66-78) — the flag branch calls it.
  - RoleConfig.Timeout time.Duration (config.go:42, from S1's grandparent) — the field it writes.

DOCS (docs/cli.md):
  - +4 rows main flags table (git-config column = —; env column populated)
  - +4 rows env/git mapping table (git-config column = —)

DOWNSTREAM (this subtask ENABLES but does NOT build — sibling subtasks):
  - P1.M1.T2.S3: stagecoach.role.<role>.timeout git-config reading (NEW loop in loadGitConfig). When it
    lands, it can flip the docs/cli.md git-config column from — to stagecoach.role.<role>.timeout.
  - P1.M2.T1.S1: ResolveRoleTimeout(role, cfg) + defaultRoleTimeouts{planner:480s} (reads Roles[role].Timeout).
  - P1.M2.T2.S1: global default 480s→120s + pinning-test fixes.
  - P1.M3: 13 provider.Execute call sites pass the resolved per-role timeout instead of cfg.Timeout.

PRECEDENCE (this task = layer 7, the flag source — HIGHEST):
  CLI flag --<role>-timeout (THIS) > env STAGECOACH_<ROLE>_TIMEOUT (S1) > [role.<role>].timeout TOML
    (S1+S2 file layer) > stagecoach.role.<role>.timeout git (S3) > global timeout > built-in role default.

UNCHANGED (do NOT touch): config.go structs (S1 grandparent); load.go setRoleTimeout/env-branch (S1);
  file.go materialize/overlay (S1/S2); git.go (S3); Defaults().Timeout (stays 480s — P1.M2.T2); the 13
  Execute call sites (P1.M3); README/how-it-works/configuration docs (P1.M4.T2.S1).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build everything (the flag vars + registrations + the loadFlags branch must compile across packages)
go build ./...
# Vet the changed packages (cmd registers the flags; config reads them)
go vet ./internal/cmd/... ./internal/config/...
# Format check
gofmt -l internal/cmd/root.go internal/config/load.go internal/config/load_test.go
# Expected: nothing listed. If listed: gofmt -w the file(s).
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new flag-loop tests (targeted)
go test ./internal/config/... -run 'PerRoleTimeout|Timeout' -v
# Expected: all pass — per-role duration form + bare-int form + field-merge; malformed silent-ignore;
#           flag-beats-env precedence.

# Full config package (regression — existing loadFlags / per-role / global-timeout tests stay green)
go test ./internal/config/... -v

# Whole suite (race) — loadFlags is on the load path of every config.Load with Flags
make test
# Expected: ALL pass. Global default still 480s (unchanged here) → 480s-pinning tests untouched.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary
make build

# Smoke: --planner-timeout loads cleanly (parse happens; no consumer uses it yet, but the flag must
# register + parse without error). After this task the value is STORED in cfg.Roles["planner"].Timeout;
# behavior observation (the planner actually using 600s) lands with P1.M2.T1/P1.M3. This smoke proves
# the flag register + parse path and the --help surface:
BIN=/home/dustin/projects/stagecoach/bin/stagecoach
mkdir -p /tmp/sc_role_flag && cd /tmp/sc_role_flag && git init -q
# (a) the flag is registered + appears in help:
$BIN --help 2>&1 | grep -E 'planner-timeout|stager-timeout|message-timeout|arbiter-timeout'
# Expected: 4 lines (one per role).
# (b) a valid value loads without a parse error (it may exit for other reasons — e.g. nothing staged —
#     but NOT a timeout parse error):
$BIN --planner-timeout 600s --dry-run --no-color 2>&1 | head -5
# Expected: loads WITHOUT error about the timeout flag.
cd / && rm -rf /tmp/sc_role_flag
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: 4 flag vars exist (one each)
grep -n 'flagPlannerTimeout\|flagStagerTimeout\|flagMessageTimeout\|flagArbiterTimeout' internal/cmd/root.go
# Expected: each var appears (declaration + &-address in the StringVar = 2 hits each).

# Grep guard: 4 flag registrations exist with the right names
grep -n '"planner-timeout"\|"stager-timeout"\|"message-timeout"\|"arbiter-timeout"' internal/cmd/root.go
# Expected: 4 hits (one per pf.StringVar).

# Grep guard: the loadFlags -timeout branch exists (once) and uses setRoleTimeout (NOT cfg.Timeout=)
grep -n 'role + "-timeout"' internal/config/load.go
# Expected: one hit inside loadFlags (the Changed line).
grep -nA4 'role + "-timeout")' internal/config/load.go
# Expected: the branch calls cfg.setRoleTimeout(role, d) — NOT cfg.Timeout = d.

# Grep guard: the flag branch SILENTLY ignores bad values (no return fmt.Errorf in loadFlags for timeout)
grep -n '_TIMEOUT: %w\|return fmt.Errorf.*timeout' internal/config/load.go
# Expected: the env branch's "%s_TIMEOUT: %w" (loadEnv, S1) is the ONLY hit — loadFlags must NOT have one.

# Grep guard: parseTimeout (not time.ParseDuration) is used in the flag branch
grep -n 'parseTimeout\|ParseDuration' internal/config/load.go
# Expected: the global --timeout, the env _TIMEOUT branch, AND the new flag branch all use parseTimeout;
#           time.ParseDuration appears only inside parseTimeout itself (@641).

# Grep guard: newFlagSet registers -timeout
grep -n 'role+"-timeout"' internal/config/load_test.go
# Expected: one hit (the new fs.String line in newFlagSet).

# Grep guard: docs/cli.md has the 4 new flag rows + 4 mapping rows
grep -n 'planner-timeout\|stager-timeout\|message-timeout\|arbiter-timeout' docs/cli.md
# Expected: 8 hits total (4 in the main table, 4 in the mapping table).

# Scope-boundary guard: this subtask added NO git-config reading / resolution / default changes
grep -rn 'stagecoach.role.*timeout\|ResolveRoleTimeout\|defaultRoleTimeouts' internal/config/load.go internal/cmd/root.go
# Expected: empty (those are P1.M1.T2.S3, P1.M2.T1 — NOT this subtask). NOTE: the flag HELP TEXT in
#           root.go mentions 'git stagecoach.role.planner.timeout' (forward-looking advisory, per the
#           contract — this is expected and mirrors the existing --planner-provider help).
grep -n '120 \* time.Second' internal/config/config.go
# Expected: empty (global default 480s→120s is P1.M2.T2; Defaults().Timeout must still be 480*time.Second).

# Scope-boundary guard: only root.go + load.go + load_test.go + docs/cli.md changed
git diff --stat -- internal/cmd/ internal/config/ docs/
# Expected: internal/cmd/root.go + internal/config/load.go + internal/config/load_test.go + docs/cli.md.
#           NO config.go struct / file.go / git.go churn.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/cmd/... ./internal/config/...` clean
- [ ] `gofmt -l` on the 3 changed .go files empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) all pass, incl. the 3 new tests

### Feature Validation
- [ ] 4 `flag<Role>Timeout string` vars exist in root.go's per-role var block
- [ ] 4 `pf.StringVar(&flag<Role>Timeout, "<role>-timeout", "", ...)` registrations exist in init()
- [ ] `--planner-timeout 600s` → `cfg.Roles["planner"].Timeout == 600*time.Second`
- [ ] `--stager-timeout 300` (bare int) → `300*time.Second` (proves parseTimeout used)
- [ ] `--planner-timeout abc` silently ignored (no error; mirrors global `--timeout`)
- [ ] `--planner-timeout` + `--planner-provider` on the same role both survive (FR-R3)
- [ ] `--planner-timeout` beats `STAGECOACH_PLANNER_TIMEOUT` env (flag layer 7 > env layer 5)
- [ ] A role with no `--<role>-timeout` is untouched (Changed==false → skipped)
- [ ] `newFlagSet` registers per-role `-timeout` flags
- [ ] docs/cli.md: 4 rows in main table + 4 rows in mapping table (git-config column = —)

### Scope-Boundary Validation
- [ ] NO `setRoleTimeout` modification / env-branch change (S1 — consumed only)
- [ ] NO `stagecoach.role.<role>.timeout` git-config reading added (P1.M1.T2.S3)
- [ ] NO `ResolveRoleTimeout` / `defaultRoleTimeouts` added (P1.M2.T1)
- [ ] `Defaults().Timeout` STILL 480s; 480s-pinning tests UNCHANGED (P1.M2.T2)
- [ ] NO config.go struct / file.go / git.go / Execute call-site changes
- [ ] NO README / how-it-works / configuration docs changes (P1.M4.T2.S1)
- [ ] Only `internal/cmd/root.go` + `internal/config/load.go` + `internal/config/load_test.go` + `docs/cli.md` changed

### Code Quality & Docs
- [ ] Flag vars are `string` (zero default), aligned by gofmt; `&flagVar` address is their use
- [ ] Flag help text matches the contract verbatim (env + forward-looking git key, mirrors --planner-provider)
- [ ] loadFlags branch comment cites FR-R7 / §15.2 + the silent-ignore rationale + the global --timeout mirror
- [ ] Tests use `fs.Set` (sets Changed=true), read `cfg.Roles[role].Timeout` (per-role field, not global), test ≥2 roles + bare-int form
- [ ] docs rows match the existing per-row column convention (type/default/env/git-config/description)

---

## Anti-Patterns to Avoid

- ❌ Don't copy the env `_TIMEOUT` branch's `return fmt.Errorf("%s_TIMEOUT: %w", prefix, err)` into loadFlags — `loadFlags` has NO error return, so it WILL NOT COMPILE. The flag layer SILENTLY ignores a malformed value (the `if perr == nil` guard skips the set), exactly like the global `--timeout` flag at load.go:433-438. The env-vs-flag asymmetry is intentional.
- ❌ Don't use `time.ParseDuration` in the flag branch — it rejects bare `"600"`. Use `parseTimeout` (load.go:640), consistent with the global `--timeout` flag / `STAGECOACH_TIMEOUT` env / `stagecoach.timeout` git. The bare-int test case (`--stager-timeout 300` → 300s) is what PROVES parseTimeout was chosen.
- ❌ Don't set `cfg.Timeout` (the global) in the per-role branch — set `cfg.Roles[role].Timeout` via `cfg.setRoleTimeout(role, d)`. They are DIFFERENT fields; the role→global fallback is P1.M2.T1's `ResolveRoleTimeout`, not this task.
- ❌ Don't route the flag value through `overlay()` — the flag layer (7) is the HIGHEST, so DIRECT-set via setRoleTimeout is the escape hatch (same reason the global `--timeout` DIRECT-sets `cfg.Timeout` at load.go:436).
- ❌ Don't forget to extend `newFlagSet` — without `fs.String(role+"-timeout", "", "")` in its per-role loop, `fs.Set("planner-timeout", ...)` returns 'unknown flag' and `fs.Changed` is false. The 3 tests all depend on this one-line edit.
- ❌ Don't make the flag vars `time.Duration` — they are `string` (zero default `""`), same as `flagPlannerProvider`/`flagTimeout`. `loadFlags` reads via `fs.Changed`/`fs.GetString`, never the var directly (the `&flagVar` address is its only use, satisfying the `unused` linter — exactly as `flagProvider`/`flagModel`).
- ❌ Don't put `stagecoach.role.<role>.timeout` in the docs/cli.md git-config column — git.go does NOT read per-role keys today (that's P1.M1.T2.S3). Use `—`, matching EVERY existing per-role row. The env column IS populated (`STAGECOACH_<ROLE>_TIMEOUT` shipped in S1). (The flag help text mentioning the git key is a forward-looking advisory per the contract, mirroring the existing `--planner-provider` help — not a docs-table value.)
- ❌ Don't touch `setRoleTimeout` / the env branch (S1), `git.go` per-role reading (S3), `ResolveRoleTimeout`/`defaultRoleTimeouts` (P1.M2.T1), the 480s→120s default change (P1.M2.T2), the 13 Execute call sites (P1.M3), or README/how-it-works/configuration docs (P1.M4.T2.S1).
- ❌ Don't read `cfg.Timeout` in the tests to verify the per-role flag — read `cfg.Roles[role].Timeout`. (A test asserting on `cfg.Timeout` would pass even if the branch were missing, masking the bug.)
- ❌ Don't test only the planner role — test ≥2 (planner + stager) plus the bare-int form to prove the loop is general and parseTimeout (not ParseDuration) is used.

---

## Confidence Score: 10/10

One-pass success is essentially certain: the `loadFlags` branch is a 1:1 clone of the existing global
`--timeout` flag handling (load.go:433-438) — only the flag name (`role+"-timeout"`) and the setter
(`cfg.setRoleTimeout(role, d)` vs `cfg.Timeout = d`) differ. The 4 flag vars + registrations are a 1:1
clone of the existing `--planner-provider`/`--planner-reasoning` pattern (root.go:184, 241) with help
text given verbatim by the contract. The prerequisite (`setRoleTimeout`) is ALREADY LANDED (verified by
reading load.go:66-78). The `newFlagSet` edit is one line. The 3 tests are clones of three existing
tests (`TestLoadFlags_PerRole`, `TestLoadFlags_TimeoutString`, `TestLoad_PerRoleFlagBeatsEnv`). The docs
rows follow the exact column convention of the existing per-role rows. There is NO file-level conflict
with the parallel siblings (S1 edits `loadEnv` in load.go; this task edits `loadFlags` in load.go —
different functions; S1 doesn't touch root.go/docs/cli.md). The only vigilance points — silent-ignore
(not error-return) in loadFlags, parseTimeout (not ParseDuration), per-role field (not global),
DIRECT-set (not overlay), newFlagSet extension, docs `—` git column — are all enumerated as CRITICAL
gotchas with grep guards.
