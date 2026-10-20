name: "P1.M1.T1.S2 — precedence layers: STAGECOACH_IDLE_TIMEOUT env, stagecoach.idleTimeout git keys, [defaults]/[role] idle_timeout file keys (+ per-role variants)"
description: >
  Add the three middle precedence layers (env, git-config, config-file) for the idle deadline (FR-R7/
  §9.30), mirroring the existing timeout seams exactly: loadEnv gains STAGECOACH_IDLE_TIMEOUT + per-role
  STAGECOACH_<ROLE>_IDLE_TIMEOUT (parseTimeout, hard-error-on-malformed, setRoleIdleTimeout); git.go
  gains stagecoach.idleTimeout + stagecoach.role.<role>.idleTimeout (CAMELCASE — git rejects underscores)
  in the same per-role loop; file.go gains fileDefaults/fileRoleConfig.IdleTimeout strings, the loadTOML
  bare-time.ParseDuration global parse (mirroring the timeout seam), the materialize signature + per-role
  parseTimeout loop, and the two overlay guards. Plus bootstrap.go env/git doc lines (Mode A) and the
  full symmetry test twins (45s sentinels). loadFlags/root flags are S3; the executor is P1.M2.

---

## Goal

**Feature Goal**: Complete the idle-timeout precedence ladder's three middle layers (flag > **env** >
**git** > **file** > default) so `STAGECOACH_IDLE_TIMEOUT` / `stagecoach.idleTimeout` /
`[defaults].idle_timeout` — and their per-role variants — all resolve into `Config.IdleTimeout` /
`RoleConfig.IdleTimeout` (from S1) structurally identically to timeout's ladder, ready for S3's flag
layer and P1.M2's executor consumption.

**Deliverable**: (1) load.go — `setRoleIdleTimeout` helper + global `STAGECOACH_IDLE_TIMEOUT` + per-role
`prefix+"_IDLE_TIMEOUT"` env reads; (2) git.go — `stagecoach.idleTimeout` block + `stagecoach.role.<role>.idleTimeout`
in the existing per-role loop (both CAMELCASE); (3) file.go — `fileDefaults.IdleTimeout` /
`fileRoleConfig.IdleTimeout` strings, the loadTOML global parse (bare time.ParseDuration), materialize
signature + `c := &Config{...IdleTimeout: idleTimeout...}` + the per-role parseTimeout loop, the global
and per-role overlay guards, and the ~16 mechanical call-site updates; (4) bootstrap.go env/git doc lines
(Mode A); (5) the symmetry test twins (load_test env, git_test camelCase found/missing/bad, file_test
parse+overlay).

**Success Definition**:
- `STAGECOACH_IDLE_TIMEOUT=45s` → `cfg.IdleTimeout == 45s`; `STAGECOACH_PLANNER_IDLE_TIMEOUT=45` (bare int) → `cfg.Roles["planner"].IdleTimeout == 45s`; malformed either → a HARD error naming the var.
- `git config stagecoach.idleTimeout 45` → `cfg.IdleTimeout == 45s`; malformed → error containing `git config stagecoach.idleTimeout`.
- `[defaults] idle_timeout = "45s"` → resolved IdleTimeout 45s; `[role.planner] idle_timeout = "45"` (bare int OK, parseTimeout) → per-role 45s; malformed → error containing `[role.<role>].idle_timeout`.
- Overlay: a higher layer's non-zero idle overrides; a zero/omitted idle preserves the lower layer; the per-role idle merge preserves the sibling Timeout (field-merge).
- Every test twin uses DISTINCT sentinels (45s idle vs the timeout tests' 90s/120s/600s) so no assertion passes through the wrong field.
- `go build ./...` (after S1), `go test ./internal/config/...`, `make test`, `make lint` pass.

## User Persona (if applicable)

**Target User**: A stagecoach user whose agent occasionally hangs mid-generation — they set a short idle
deadline (env, git-config, or file) so a silent agent is killed at 45s while a productive-but-slow one
runs to the 600s hard cap.

**Use Case**: `STAGECOACH_IDLE_TIMEOUT=45s stagecoach` or `git config stagecoach.idleTimeout 45` per-repo
— the same three surfaces timeout already offers, now for the idle deadline.

**Pain Points Addressed**: Before the ladder, the idle deadline exists only as a config field (S1) with
no way to SET it from any precedence layer except programmatically.

## Why

- **FR-R7 / FR25 / §9.30 (P0, → G30)**: the idle deadline is the PRIMARY productivity signal (kill on
  silence) with the hard cap as backstop. S1 established the fields + resolution; S2 makes every
  non-flag precedence layer (env, git, file — global AND per-role) able to set it, mirroring timeout's
  proven seams so the two ladders stay structurally identical.
- **FR35/FR36 (§9.8)**: the env-var and git-key registries gain their idle entries. The git key is
  CAMELCASE (`idleTimeout`) because git rejects underscores in the final config-key segment — matching
  §16.3's `idleTimeout = 60` example and the existing `autoStageAll` convention (FR36's snake_case
  `idle_timeout` spelling is git-infeasible; the codebase's docs-side fix pattern applies).

## What

**User-visible behavior**: `STAGECOACH_IDLE_TIMEOUT` / `stagecoach.idleTimeout` / `[defaults].idle_timeout`
(and the `_<ROLE>_` / `role.<role>.` / `[role.<role>]` per-role variants) now set the idle deadline on
the standard precedence ladder. Malformed values are hard config-load errors naming the source.

**Technical change (load.go + git.go + file.go + bootstrap.go doc lines + tests):**
1. load.go: setRoleIdleTimeout (:66-75 mirror); STAGECOACH_IDLE_TIMEOUT (:272-278 mirror); per-role `_IDLE_TIMEOUT` (:316-325 mirror, same loop).
2. git.go: stagecoach.idleTimeout block (:147-156 mirror); stagecoach.role.<role>.idleTimeout in the same per-role loop (:158-177 extension).
3. file.go: two string fields; the loadTOML bare-time.ParseDuration global parse (:196-204 mirror); materialize signature + seed + the per-role parseTimeout loop (:342-358 mirror); overlay guards (:391-392 + :507-508 mirrors); ~16 call-site updates.
4. bootstrap.go: one env doc line (after :359) + one git doc line (after :375).

### Success Criteria
- [ ] env: global + per-role idle reads, parseTimeout, hard errors naming the var
- [ ] git: `stagecoach.idleTimeout` + `stagecoach.role.<role>.idleTimeout` (camelCase), same error style
- [ ] file: `[defaults].idle_timeout` (bare time.ParseDuration seam) + `[role.<role>].idle_timeout` (parseTimeout)
- [ ] materialize threads idleTimeout; both overlay guards present
- [ ] bootstrap.go env/git doc lines added (Mode A)
- [ ] Test twins with distinct sentinels green; the ~16 signature-rippled call sites compile+pass
- [ ] `go build ./...`, `go test ./internal/config/...`, `make test`, `make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?**
**Yes** — every mirrored seam is quoted with verified line numbers, the camelCase git-key constraint is explicit, the file-layer parser choice (bare time.ParseDuration, per the item contract) is pinned with its consequence, the materialize signature ripple is fully enumerated (~16 sites), and the test-twin symmetry rule is specified with distinct sentinels.

### Documentation & References

```yaml
- file: internal/config/load.go
  why: "THE env layer. setRoleTimeout helper (:66-75) → twin setRoleIdleTimeout. Global STAGECOACH_TIMEOUT
        (:272-278: LookupEnv → parseTimeout → error \"STAGECOACH_TIMEOUT: %w\" → cfg.Timeout = d) → twin
        STAGECOACH_IDLE_TIMEOUT. Per-role loop (:316-325, inside the roleNames loop after _REASONING:
        prefix+\"_TIMEOUT\" → parseTimeout → error \"%s_TIMEOUT: %w\" → setRoleTimeout) → twin
        prefix+\"_IDLE_TIMEOUT\" in the SAME loop right after."
  pattern: "Presence-semantic DIRECT-set (bypasses overlay) — the env layer's escape hatch for false.
            parseTimeout accepts \"45s\" AND bare \"45\"."
  gotcha: "Anchor on constructs (grep STAGECOACH_TIMEOUT / setRoleTimeout); S1 is editing the same file's
           neighbors — line numbers drift. HARD error on malformed (NOT silent-ignore — that asymmetry is
           loadFlags-only)."

- file: internal/config/git.go
  why: "THE git layer. Global block (:147-156: gitConfigGet \"stagecoach.timeout\" → parseTimeout → error
        \"git config stagecoach.timeout: %w\" → c.Timeout = d) → twin stagecoach.idleTimeout. Per-role loop
        (:158-177: key := \"stagecoach.role.\"+role+\".timeout\" → same shape → setRoleTimeout) → EXTEND
        the SAME loop with a second key read (stagecoach.role.<role>.idleTimeout → setRoleIdleTimeout).
        Update the loop's 'this loop is timeout-only' comment."
  pattern: "gitConfigGet maps a missing key to found=false (no-op) — a role with no key is untouched."
  critical: "CAMELCASE KEYS: `stagecoach.idleTimeout` / `stagecoach.role.<role>.idleTimeout`. git REJECTS
             underscores in the final key segment — a snake_case key is un-settable (invalid key error).
             This matches §16.3's `idleTimeout = 60` example and the existing autoStageAll convention."

- file: internal/config/file.go
  why: "THE file layer. fileRoleConfig.Timeout string (:28) → twin IdleTimeout after it. fileDefaults.Timeout
        string (:56) → twin IdleTimeout after it. loadTOML global parse (:196-204: BARE time.ParseDuration,
        error \"parse config %s: invalid timeout %q: %w\") → twin idle_timeout (same bare parser — the
        ITEM's contract: mirror the seam; consequence: file-global accepts ONLY \"45s\"-style Go durations,
        NOT bare ints — same as timeout today). materialize (:234-235) gains idleTimeout →
        materialize(fc, timeout, idleTimeout, hookTimeout) + c := &Config{Timeout:..., IdleTimeout:...,
        HookTimeout:...}. Per-role loop (:342-358: parseTimeout(frc.Timeout), error \"parse config:
        [role.%s].timeout: %w\", RoleConfig{...,Timeout: rt}) → twin (parseTimeout, [role.%s].idle_timeout,
        IdleTimeout: rit). Overlay global guard (:391-392) + per-role guard (:507-508) → twins
        (if src.IdleTimeout != 0 / if rc.IdleTimeout != 0)."
  pattern: "The duration-zero guard (`!= 0`) — NOT `!= nil` (IdleTimeout is a plain time.Duration)."
  gotcha: "The materialize signature change ripples to ~16 call sites: file.go:215 (production), 
           bootstrap_validate_test.go:68, file_test.go:948/1004/1009/1117/1183/1191/1308/1340/1359,
           multiturn_test.go:46/88/108/116. Mechanical: `0, 0` → `0, 0, 0`; bootstrap_validate
           `120*time.Second, 10*time.Minute` → `120*time.Second, 0, 10*time.Minute`."

- file: internal/config/bootstrap.go
  why: "Mode A docs ONLY: env block line :359 (`#   STAGECOACH_TIMEOUT    generation timeout...`) → ADD
        an idle line after it; git block line :375 (`#   git config stagecoach.timeout 120s`) → ADD
        `#   git config stagecoach.idleTimeout 45s` after it. NOT the template body (~L174) or the flag
        doc block — those are S3."
  gotcha: "The existing :359/:375 prose says 120s — S1 changes the default to 600s. S2's ADDED lines use
           an example value (e.g. 45s), avoiding the stale-default question; do NOT rewrite the existing
           timeout lines (S3 owns the 120s→600s prose sweep with the template + flag help)."

- docfile: plan/022_856008a8cbb5/architecture/config_ladder.md
  why: "THE verified line-for-line map of every timeout surface + the complete idle-seam checklist (§8
        items 2-4 are S2's scope) + the test-twin symmetry rule (§7) + the camelCase FINDING A."
- docfile: plan/022_856008a8cbb5/P1M1T1S1/PRP.md
  why: "S1 is the CONTRACT — it produces Config.IdleTimeout/RoleConfig.IdleTimeout (0 ⇒ inherit) and
        explicitly excludes load.go/file.go/git.go (S2 owns them). Confirms S2 compiles only after S1."
- docfile: plan/022_856008a8cbb5/P1M1T1S2/research/findings.md
  why: "The verified seams (quoted), the camelCase rationale, the parser-choice consequence, the ~16-site
        ripple list, the sentinel rule, scope fences."
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  load.go             # +setRoleIdleTimeout, +STAGECOACH_IDLE_TIMEOUT, +per-role _IDLE_TIMEOUT (env layer)
  git.go              # +stagecoach.idleTimeout block, +per-role idleTimeout in the loop (git layer)
  file.go             # +2 string fields, +loadTOML parse, +materialize sig/seed/loop, +2 overlay guards (file layer)
  bootstrap.go        # +2 doc lines (env block, git block) — Mode A
  load_test.go        # +idle env twins (global/per-role/malformed)
  git_test.go         # +idle git twins (found/missing/bad — camelCase key)
  file_test.go        # +idle parse/overlay twins; ~10 materialize call sites updated (4-arg)
  multiturn_test.go   # ~4 materialize call sites updated (4-arg)
  bootstrap_validate_test.go  # 1 materialize call site updated (4-arg)
```

### Desired Codebase tree with files to be added

```bash
(No new files — all edits to existing internal/config files.)
```

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (S1 dependency): Config.IdleTimeout / RoleConfig.IdleTimeout exist ONLY after S1 lands.
//   Confirm: grep -n "IdleTimeout" internal/config/config.go. S2's code references both fields — it will
//   not compile pre-S1. S1 also reseeds Timeout 600s (any comment you touch that names the default
//   should say 600s, not 120s).

// CRITICAL (camelCase git keys): git REJECTS underscores in the final config-key segment. The keys are
//   `stagecoach.idleTimeout` and `stagecoach.role.<role>.idleTimeout` — NEVER stagecoach.idle_timeout.
//   (§16.3's `idleTimeout = 60` example is authoritative; matches the existing autoStageAll convention.)

// CRITICAL (file-global parser — mirror the seam, per the item contract): the loadTOML global parse uses
//   BARE time.ParseDuration (file.go:196-204), NOT parseTimeout. The idle twin mirrors that seam exactly:
//   [defaults].idle_timeout accepts ONLY "45s"-style Go durations; a bare int "45" is a hard parse error
//   there (same as [defaults].timeout today). The per-role file key + env + git DO use parseTimeout (both
//   forms). This pre-existing asymmetry is preserved deliberately.

// CRITICAL (hard-error env discipline): a malformed STAGECOACH_IDLE_TIMEOUT / _<ROLE>_IDLE_TIMEOUT is a
//   HARD error naming the var (mirroring STAGECOACH_TIMEOUT at :275: `return fmt.Errorf("STAGECOACH_IDLE_TIMEOUT: %w", err)`).
//   The silent-ignore-on-malformed behavior exists ONLY in loadFlags (flag layer) — which is S3's, not S2's.

// GOTCHA (per-role reads extend the EXISTING loops): load.go's per-role env block and git.go's per-role
//   loop already iterate roleNames for _TIMEOUT / .timeout. Add the idle read INSIDE those same loops
//   (right after the timeout read), do NOT create parallel loops. Update git.go's "this loop is
//   timeout-only" comment.

// GOTCHA (overlay guards are duration-zero): `if src.IdleTimeout != 0` / `if rc.IdleTimeout != 0` — plain
//   time.Duration value type, NOT a pointer. 0 = inherit lower layer; non-zero = override (mirrors
//   Config.Timeout's existing guards exactly).

// GOTCHA (materialize signature ripple): adding the idleTimeout param breaks ~16 call sites (1 production
//   + 15 test). Mechanical fix-all: `materialize(fc, 0, 0)` → `materialize(fc, 0, 0, 0)`; the one valued
//   site bootstrap_validate_test.go:68 → `materialize(&fc, 120*time.Second, 0, 10*time.Minute)`.

// GOTCHA (test sentinels): use 45s for every idle test value (the timeout tests use 90s/60s/120s/600s)
//   so no idle assertion can accidentally pass through the Timeout field or vice versa.

// SCOPE: S2 is load.go + git.go + file.go + bootstrap.go(2 doc lines) + the test twins. Do NOT add
//   loadFlags idle reads or root.go flags (S3), the bootstrap template body (S3), docs/*.md (S3),
//   executor/activity infra (P1.M2), stubagent (P1.M3), or ResolveRoleDeadlines/callers (S1/P1.M2).
//   Do NOT bump CurrentConfigVersion (additive optional keys never bump — FR-B4).
```

## Implementation Blueprint

### Data models and structure
No new types. Two string decode fields (fileDefaults/fileRoleConfig), three layer reads, one helper, one signature extension, two guards — all mirroring existing timeout seams.

### Implementation Tasks (ordered by dependencies)

> **Prerequisite — S1 has LANDED** (confirmed during research: `grep -c IdleTimeout
> internal/config/config.go` == 4 — Config.IdleTimeout + RoleConfig.IdleTimeout exist; the defaultRoleTimeouts
> retirement may still be mid-flight in roles.go, but S2 does not depend on roles.go). S2's code references
> both fields and compiles against S1's landed config.go. Anchor on constructs (grep the timeout seam
> names — `STAGECOACH_TIMEOUT`, `setRoleTimeout`, `stagecoach.timeout`, `src.Timeout != 0`), not line
> numbers: S1 edits the same package and the live tree drifts.

```yaml
Task 1: MODIFY internal/config/load.go — setRoleIdleTimeout helper + the two env reads
  - HELPER (mirror :66-75 setRoleTimeout; place right after it):
        // setRoleIdleTimeout sets the IdleTimeout field for a role in cfg.Roles, lazily allocating the map.
        func (c *Config) setRoleIdleTimeout(role string, d time.Duration) {
            if c.Roles == nil {
                c.Roles = make(map[string]RoleConfig)
            }
            rc := c.Roles[role]
            rc.IdleTimeout = d
            c.Roles[role] = rc
        }
  - GLOBAL (mirror :272-278; place right after the STAGECOACH_TIMEOUT block):
        if v, ok := os.LookupEnv("STAGECOACH_IDLE_TIMEOUT"); ok && v != "" {
            d, err := parseTimeout(v)
            if err != nil {
                return fmt.Errorf("STAGECOACH_IDLE_TIMEOUT: %w", err)
            }
            cfg.IdleTimeout = d
        }
  - PER-ROLE (mirror :316-325; INSIDE the same roleNames loop, right after the _TIMEOUT read):
        if v, ok := os.LookupEnv(prefix + "_IDLE_TIMEOUT"); ok && v != "" {
            d, err := parseTimeout(v)
            if err != nil {
                return fmt.Errorf("%s_IDLE_TIMEOUT: %w", prefix, err)
            }
            cfg.setRoleIdleTimeout(role, d)
        }
  - DEPENDENCIES: S1 (RoleConfig.IdleTimeout; Config.IdleTimeout).

Task 2: MODIFY internal/config/git.go — the camelCase global key + the per-role loop extension
  - GLOBAL (mirror the :147-156 timeout block; place right after it):
        // --- idleTimeout: accepts both "45" (seconds) and "45s" (Go duration) forms. CAMELCASE — git
        // rejects underscores in the final key segment (§16.3 example: idleTimeout = 60). ---
        if v, found, err := gitConfigGet(repoDir, "stagecoach.idleTimeout"); err != nil {
            return nil, err
        } else if found {
            d, perr := parseTimeout(v)
            if perr != nil {
                return nil, fmt.Errorf("git config stagecoach.idleTimeout: %w", perr)
            }
            c.IdleTimeout = d
        }
  - PER-ROLE (EXTEND the existing :158-177 loop — add a second key read per role; update the loop's
    "this loop is timeout-only" comment to "timeout + idle_timeout"):
        key2 := "stagecoach.role." + role + ".idleTimeout"
        if v, found, err := gitConfigGet(repoDir, key2); err != nil {
            return nil, err
        } else if found {
            d, perr := parseTimeout(v)
            if perr != nil {
                return nil, fmt.Errorf("git config %s: %w", key2, perr)
            }
            c.setRoleIdleTimeout(role, d)
        }
  - DEPENDENCIES: Task 1 (setRoleIdleTimeout).

Task 3: MODIFY internal/config/file.go — the two decode fields, the loadTOML parse, materialize, the guards
  - FIELDS: after fileDefaults.Timeout (:56) add `IdleTimeout string \`toml:"idle_timeout"\`` (comment:
    duration string, parsed in loadTOML); after fileRoleConfig.Timeout (:28) add the same field
    (comment: parsed in materialize via parseTimeout).
  - loadTOML GLOBAL PARSE (mirror :196-204; place right after the timeout parse, BEFORE hook_timeout):
        var idleTimeout time.Duration
        if fc.Defaults.IdleTimeout != "" {
            idleTimeout, err = time.ParseDuration(fc.Defaults.IdleTimeout)
            if err != nil {
                return nil, fmt.Errorf("parse config %s: invalid idle_timeout %q: %w", path, fc.Defaults.IdleTimeout, err)
            }
        }
    (BARE time.ParseDuration — mirrors the timeout seam per the item contract; bare ints rejected here.)
  - materialize SIGNATURE (:234): `func materialize(fc *fileConfig, timeout, idleTimeout, hookTimeout time.Duration) (*Config, error)`;
    seed (:235): `c := &Config{Timeout: timeout, IdleTimeout: idleTimeout, HookTimeout: hookTimeout}`;
    production call (:215): `materialize(&fc, timeout, idleTimeout, hookTimeout)`.
  - PER-ROLE LOOP (:342-358; add the idle parse beside the timeout parse):
        var rit time.Duration
        if frc.IdleTimeout != "" {
            d, perr := parseTimeout(frc.IdleTimeout) // parseTimeout: "45s" OR bare "45"
            if perr != nil {
                return nil, fmt.Errorf("parse config: [role.%s].idle_timeout: %w", role, perr)
            }
            rit = d
        }
        // ... RoleConfig{Provider: ..., Model: ..., Reasoning: ..., Timeout: rt, IdleTimeout: rit}
  - OVERLAY GUARDS: after the global :391-392 guard add `if src.IdleTimeout != 0 { dst.IdleTimeout = src.IdleTimeout }`;
    in the per-role merge (:507-508) add `if rc.IdleTimeout != 0 { existing.IdleTimeout = rc.IdleTimeout }`.
  - CALL-SITE RIPPLE (mechanical, ~16 sites — grep `materialize(`): file_test.go:948/1004/1009/1117/1183/1191/1308/1340/1359
    and multiturn_test.go:46/88/108/116 → append `, 0` (`0, 0` → `0, 0, 0`); bootstrap_validate_test.go:68
    → `materialize(&fc, 120*time.Second, 0, 10*time.Minute)`.
  - DEPENDENCIES: S1 + Tasks 1-2.

Task 4: MODIFY internal/config/bootstrap.go — the two Mode A doc lines
  - ENV BLOCK: after line :359 (`#   STAGECOACH_TIMEOUT    generation timeout, e.g. "120s" or 120 (seconds)`)
    add: `#   STAGECOACH_IDLE_TIMEOUT   idle deadline (kill on silence), e.g. "45s" or 45 (seconds)`
  - GIT BLOCK: after line :375 (`#   git config stagecoach.timeout 120s`) add:
    `#   git config stagecoach.idleTimeout 45s`  (camelCase, matching the block's autoStageAll convention)
  - DO NOT touch the template body (~L174 `# timeout = "120s"`) or the flag doc block — S3 owns those.
  - DEPENDENCIES: none (comment-only).

Task 5: ADD the test twins (distinct 45s sentinels; mirror the existing timeout tests' harness)
  - load_test.go:
      * TestLoadEnv_IdleTimeout: t.Setenv STAGECOACH_IDLE_TIMEOUT=45s → cfg.IdleTimeout==45*time.Second;
        a bare-int row (45 → 45s); a malformed row ("abc") → err contains "STAGECOACH_IDLE_TIMEOUT".
      * TestLoadEnv_PerRoleIdleTimeout: STAGECOACH_PLANNER_IDLE_TIMEOUT=45s and a bare-int 45 row →
        cfg.Roles["planner"].IdleTimeout==45s (and Roles nil-alloc when absent); a same-role _PROVIDER
        proves field-merge (idle set, provider set, siblings untouched); malformed → err contains the var name.
        (Mirror TestLoadEnv_PerRoleTimeout :368 + the bad-value discipline :390-400.)
  - git_test.go:
      * Found: setGitConfig(t, repo, "stagecoach.idleTimeout", "45") → cfg.IdleTimeout == 45*time.Second
        (CAMELCASE key); a per-role row setGitConfig(..., "stagecoach.role.planner.idleTimeout", "45s")
        → Roles["planner"].IdleTimeout == 45s.
      * Missing: no key → cfg.IdleTimeout untouched (zero / default preserved).
      * TestLoadGitConfig_BadIdleTimeout: value "abc" → err contains "stagecoach.idleTimeout".
  - file_test.go:
      * Parse: writeTempTOML with `[defaults]\nidle_timeout = "45s"` → loadTOML → resolved IdleTimeout==45s;
        `[role.planner]\nidle_timeout = "45"` (bare int, parseTimeout) → per-role 45s; malformed
        `[defaults] idle_timeout = "abc"` → err contains "invalid idle_timeout"; malformed per-role →
        err contains "[role.planner].idle_timeout"; bare-int at the GLOBAL file key ("45") → REJECTED
        (bare time.ParseDuration — pins the seam).
      * Overlay: global — src.IdleTimeout=45s overrides dst's; zero src preserves dst's; per-role — an
        idle-only src merge preserves the sibling Timeout (field-merge), mirroring the Timeout overlay tests.
  - DEPENDENCIES: Tasks 1-3.

Task 6: VERIFY — full green + the grep gates
  - go build ./... (S1 landed; the ~16 materialize sites updated)
  - go test ./internal/config/... -v  (all twins + existing tests green)
  - grep gates (Level 4 below); make test && make lint
```

### Implementation Patterns & Key Details

```go
// PATTERN: the env read (hard error naming the var — mirrors STAGECOACH_TIMEOUT)
if v, ok := os.LookupEnv("STAGECOACH_IDLE_TIMEOUT"); ok && v != "" {
	d, err := parseTimeout(v)
	if err != nil {
		return fmt.Errorf("STAGECOACH_IDLE_TIMEOUT: %w", err)
	}
	cfg.IdleTimeout = d
}

// PATTERN: the git read (camelCase key; wrapped error; missing key = no-op via found=false)
if v, found, err := gitConfigGet(repoDir, "stagecoach.idleTimeout"); err != nil {
	return nil, err
} else if found {
	d, perr := parseTimeout(v)
	if perr != nil {
		return nil, fmt.Errorf("git config stagecoach.idleTimeout: %w", perr)
	}
	c.IdleTimeout = d
}

// PATTERN: the file-global parse (BARE time.ParseDuration — mirrors the timeout seam; NOT parseTimeout)
var idleTimeout time.Duration
if fc.Defaults.IdleTimeout != "" {
	idleTimeout, err = time.ParseDuration(fc.Defaults.IdleTimeout)
	if err != nil {
		return nil, fmt.Errorf("parse config %s: invalid idle_timeout %q: %w", path, fc.Defaults.IdleTimeout, err)
	}
}

// PATTERN: the per-role file parse (parseTimeout — BOTH forms; role-wrapped error)
if frc.IdleTimeout != "" {
	d, perr := parseTimeout(frc.IdleTimeout)
	if perr != nil {
		return nil, fmt.Errorf("parse config: [role.%s].idle_timeout: %w", role, perr)
	}
	rit = d
}

// PATTERN: the overlay guards (duration-zero; plain value type — NOT pointer)
if src.IdleTimeout != 0 { dst.IdleTimeout = src.IdleTimeout }          // global
if rc.IdleTimeout != 0  { existing.IdleTimeout = rc.IdleTimeout }      // per-role field-merge
```

### Integration Points

```yaml
NO struct/signature changes beyond materialize's +1 param. No public API, no executor, no flags.

CODE:
  - internal/config/load.go — +setRoleIdleTimeout, +STAGECOACH_IDLE_TIMEOUT, +per-role _IDLE_TIMEOUT
  - internal/config/git.go — +stagecoach.idleTimeout block, +per-role idleTimeout loop read
  - internal/config/file.go — +2 decode fields, +loadTOML parse, materialize sig+seed+loop, +2 guards, ~16 call sites
  - internal/config/bootstrap.go — +2 doc lines (env block, git block)
TESTS:
  - load_test.go / git_test.go / file_test.go — the idle twins (45s sentinels)

CONSUMED (from S1, must land first):
  - Config.IdleTimeout / RoleConfig.IdleTimeout (0 ⇒ inherit; toml "idle_timeout")

DOWNSTREAM (consumes S2's ladder — do NOT implement here):
  - P1.M1.T1.S3: the flag layer (loadFlags "idle-timeout" / role+"-idle-timeout" reads, root.go StringVars,
    the bootstrap template body, docs/cli.md + docs/configuration.md tables)
  - P1.M2.T1.S2/S3: the executor's Deadlines struct + the 13 call sites → ResolveRoleDeadlines
  - P1.M3.T1.S4: config observability (every precedence layer observably binds per role)

UNCHANGED: root.go (S3); the bootstrap template body (S3); the executor (P1.M2);
  ResolveRoleDeadlines/ResolveRoleTimeout + their callers (S1/P1.M2); CurrentConfigVersion (FR-B4).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...     # S1 landed; the ~16 materialize call sites updated; both fields resolve
go vet ./...
gofmt -l internal/config/
# Expected: empty.
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The config package — the new twins + the signature-rippled tests
go test ./internal/config/... -v
# Expected: ALL pass — the env/git/file idle twins (45s sentinels), the camelCase git key cases,
#           the overlay-guard twins, and every existing test (the materialize sites now 4-arg).

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# Manual five-layer sanity (env > git > file > default), using the real Load chain:
d=$(mktemp -d) && cd "$d" && git init -q && git config user.email t@e && git config user.name t
printf '[defaults]\nidle_timeout = "30s"\n' > .stagecoach.toml
# file layer alone (via a tiny go run or the test suite — the twin tests cover this; here just prove
# the seam composes): STAGECOACH_IDLE_TIMEOUT=45s overrides the file's 30s at Load time.
# (The in-repo twin tests are the within-scope proof; the full behavioral verification is P1.M3.T1.S4.)
rm -rf "$d"
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: the env var is read
grep -n '"STAGECOACH_IDLE_TIMEOUT"\|_IDLE_TIMEOUT' internal/config/load.go
# Expected: the global read + the per-role read (2+ hits).

# Grep guard: the CAMELCASE git keys are read; NO snake_case git key anywhere
grep -n 'stagecoach.idleTimeout\|stagecoach.role.*idleTimeout' internal/config/git.go
# Expected: the global block + the per-role loop (camelCase only).
grep -n 'stagecoach.idle_timeout' internal/config/
# Expected: EMPTY (snake_case git key is un-settable — never referenced).

# Grep guard: the file keys + guards
grep -n 'idle_timeout' internal/config/file.go
# Expected: the 2 toml tags + the loadTOML parse + the per-role parse + the 2 guard-adjacent comments.
grep -n 'src.IdleTimeout != 0\|rc.IdleTimeout != 0' internal/config/file.go
# Expected: both overlay guards.

# Grep guard: the materialize signature + all call sites are 4-arg
grep -rn 'materialize(' internal/config/*_test.go | grep -v ', 0, 0, 0\|time.Minute)' | grep 'materialize(.*0, 0)$'
# Expected: empty (no stale 3-arg call sites).

# Grep guard: the bootstrap doc lines
grep -n 'STAGECOACH_IDLE_TIMEOUT\|stagecoach.idleTimeout' internal/config/bootstrap.go
# Expected: the env line + the git line (2 hits).

# Scope-boundary guard: no flag-layer work (S3), no executor work (P1.M2)
grep -rn 'idle-timeout' internal/config/load.go internal/cmd/root.go
# Expected: EMPTY in S2's diff (loadFlags "idle-timeout" + root.go flags are S3's).
git diff --name-only
# Expected: only internal/config/{load.go,git.go,file.go,bootstrap.go,load_test.go,git_test.go,file_test.go,multiturn_test.go,bootstrap_validate_test.go}.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean (S1 landed; ~16 materialize sites updated)
- [ ] `go vet ./...` clean
- [ ] `gofmt -l internal/config/` empty
- [ ] `make lint` zero errors
- [ ] `go test ./internal/config/...` green; `make test` green

### Feature Validation
- [ ] STAGECOACH_IDLE_TIMEOUT / STAGECOACH_<ROLE>_IDLE_TIMEOUT read via parseTimeout; malformed = hard error naming the var
- [ ] stagecoach.idleTimeout / stagecoach.role.<role>.idleTimeout (camelCase) read; malformed = wrapped error; missing = no-op
- [ ] [defaults].idle_timeout (bare time.ParseDuration seam) + [role.<role>].idle_timeout (parseTimeout, both forms)
- [ ] materialize threads idleTimeout; both overlay guards (`!= 0`) present; per-role merge preserves siblings
- [ ] bootstrap.go env + git doc lines added
- [ ] All test twins use distinct 45s sentinels; the global-file bare-int rejection pinned

### Scope-Boundary Validation
- [ ] NO loadFlags / root.go flag work (S3)
- [ ] NO bootstrap template-body change (S3)
- [ ] NO docs/*.md change (S3)
- [ ] NO executor / activity / stubagent change (P1.M2/P1.M3)
- [ ] NO ResolveRoleDeadlines/caller change (S1/P1.M2)
- [ ] CurrentConfigVersion UNCHANGED (FR-B4)

### Code Quality
- [ ] Per-role reads extend the EXISTING loops (no parallel loops); git.go's "timeout-only" comment updated
- [ ] Errors follow each layer's existing wrap style (env: var name; git: "git config <key>"; file: path/[role.X])
- [ ] The parser asymmetry (file-global = bare ParseDuration; per-role/env/git = parseTimeout) documented in comments

---

## Anti-Patterns to Avoid

- ❌ Don't use a snake_case git key (`stagecoach.idle_timeout`) — git REJECTS underscores in the final key segment; the key is un-settable. CAMELCASE `stagecoach.idleTimeout` / `stagecoach.role.<role>.idleTimeout` (§16.3's example; the autoStageAll convention).
- ❌ Don't use parseTimeout for the file-GLOBAL idle parse — mirror the seam per the item contract: BARE time.ParseDuration (file.go:196-204), accepting only "45s"-style durations. (The per-role file key DOES use parseTimeout — that's the existing timeout asymmetry, preserved.)
- ❌ Don't silent-ignore malformed env values — the env layer is HARD-error-on-malformed (`STAGECOACH_IDLE_TIMEOUT: %w`), mirroring STAGECOACH_TIMEOUT. Silent-ignore is a loadFlags-only asymmetry (S3's territory).
- ❌ Don't create parallel per-role loops — extend the existing roleNames loops in load.go (:316-325 area) and git.go (:158-177) with the idle read directly after the timeout read.
- ❌ Don't use a pointer guard (`!= nil`) — IdleTimeout is a plain time.Duration; the guard is `!= 0` (0 = inherit), mirroring Config.Timeout's guards exactly.
- ❌ Don't forget the ~16 materialize call sites — the signature gains idleTimeout; every 3-arg call breaks. Mechanical `0, 0` → `0, 0, 0`; grep `materialize(` to enumerate.
- ❌ Don't reuse timeout test values (90s/60s/120s/600s) for idle — use 45s so no assertion can pass through the wrong field.
- ❌ Don't add loadFlags/root.go flag reads or the bootstrap template body — the flag layer is S3 (item OUTPUT: "consumed by P1.M1.T1.S3 (flag surface)").
- ❌ Don't rewrite the existing bootstrap timeout prose's "120s" — S1 changed the default to 600s and S3 owns the prose sweep; S2 only ADDS the two idle lines.
- ❌ Don't bump CurrentConfigVersion — additive optional keys never bump (FR-B4).

---

## Confidence Score: 9/10

One-pass success is very high: every mirrored seam is quoted with verified line numbers from the
architecture doc AND re-confirmed in the live tree, the camelCase git-key constraint is explicit (the
one genuinely novel gotcha), the parser-choice consequence (file-global bare-int rejection) is pinned by
the item contract, the ~16-site materialize ripple is fully enumerated, and the test-twin symmetry rule
with distinct sentinels guards against cross-field assertion accidents. The -1 is the shared-file drift
risk (S1 landed config.go and may still be mid-flight in roles.go while S2 edits load/git/file — anchor
on constructs, re-grep before each edit) plus the ~17 test-file materialize call sites (mechanical, but
a missed one is a compile break caught immediately by `go build`). Both mitigated by the prerequisite
check and the Level-4 grep gate.
