# Research Findings — P1.M1.T1.S2 (precedence layers: env + git + file keys for idle_timeout)

## 1. S1 contract + status

S1 ("Implementing" in parallel) adds `Config.IdleTimeout` (toml "idle_timeout") + `RoleConfig.IdleTimeout`
(0 ⇒ inherit), seeds Defaults() IdleTimeout 60s / Timeout 600s, deletes defaultRoleTimeouts, adds
ResolveRoleDeadlines. S1 does NOT touch load.go/file.go/git.go (S2 owns the layers).
**S1 has NOT landed** (no IdleTimeout in config.go; defaultRoleTimeouts still at roles.go:11). S2 CONSUMES
the fields — S2's code references `IdleTimeout` on Config/RoleConfig, so S2 compiles only after S1.
(Also note: S1 reseeds Timeout 600s, so stale 120s prose in touched comments should say 600s.)

## 2. The verified seams S2 mirrors (all confirmed in the live tree)

### load.go
- `setRoleTimeout` helper :66-75 (lazy map alloc + field set) → twin `setRoleIdleTimeout`.
- Global env :272-278: `LookupEnv("STAGECOACH_TIMEOUT")` → parseTimeout → hard error `"STAGECOACH_TIMEOUT: %w"` → cfg.Timeout = d.
- Per-role env :316-325 (inside the roleNames loop, after _REASONING): `prefix + "_TIMEOUT"` → parseTimeout → hard error `"%s_TIMEOUT: %w"` → cfg.setRoleTimeout(role, d). The idle twin goes in the SAME loop right after.
- parseTimeout :673-684 — accepts "480s" AND bare "480" (seconds). Reused verbatim (its error wording says "invalid timeout" — shared; fine).

### git.go
- Global :147-156: `gitConfigGet(repoDir, "stagecoach.timeout")` → parseTimeout → wrapped error `"git config stagecoach.timeout: %w"` → c.Timeout = d.
- Per-role :158-177: `for _, role := range roleNames { key := "stagecoach.role." + role + ".timeout"; gitConfigGet → parseTimeout → "git config %s: %w" → setRoleTimeout }`. The idle twin EXTENDS THIS SAME LOOP (a second key read per role). Comment says "this loop is timeout-only" — update it.
- **CAMELCASE REQUIRED**: git REJECTS underscores in the final key segment. The keys are `stagecoach.idleTimeout` and `stagecoach.role.<role>.idleTimeout`. (FR36's snake_case `idle_timeout` spelling is git-infeasible; §16.3's example `idleTimeout = 60` is authoritative. Matches the existing camelCase autoStageAll convention.)

### file.go
- `fileRoleConfig.Timeout string` :28 → twin `IdleTimeout string` after it.
- `fileDefaults.Timeout string` :56 → twin `IdleTimeout string` after it.
- loadTOML global parse :196-204: **bare `time.ParseDuration`** (NOT parseTimeout — pre-existing seam), error `"parse config %s: invalid timeout %q: %w"`. The idle twin mirrors the SEAM exactly (bare time.ParseDuration; error `... invalid idle_timeout %q: %w`). Consequence: the file-global form accepts ONLY "45s"-style Go durations (not bare "45") — same as timeout today. (config_ladder.md flags this as a conscious choice; the ITEM's contract says mirror the seam.)
- materialize :234: `func materialize(fc *fileConfig, timeout, hookTimeout time.Duration) (*Config, error)`; `c := &Config{Timeout: timeout, HookTimeout: hookTimeout}` :235. S2: add `idleTimeout` param → `materialize(fc, timeout, idleTimeout, hookTimeout)`; `c := &Config{Timeout: timeout, IdleTimeout: idleTimeout, HookTimeout: hookTimeout}`.
- Per-role materialize loop :342-358: parseTimeout(frc.Timeout), error `"parse config: [role.%s].timeout: %w"`, `RoleConfig{..., Timeout: rt}`. Twin: parseTimeout(frc.IdleTimeout), error `...[role.%s].idle_timeout: %w`, `IdleTimeout: rit`.
- Overlay global guard :391-392: `if src.Timeout != 0 { dst.Timeout = src.Timeout }` → twin `if src.IdleTimeout != 0`.
- Overlay per-role guard :503-508: `if rc.Timeout != 0 { existing.Timeout = rc.Timeout }` → twin `if rc.IdleTimeout != 0`.

### materialize signature ripple (~16 call sites)
1 production (file.go:215) + tests: bootstrap_validate_test.go:68 (`120*time.Second, 10*time.Minute`), file_test.go:948/1004/1009/1117/1183/1191/1308/1340/1359 (`0, 0`), multiturn_test.go:46/88/108/116 (`0, 0`). All become 4-arg (mechanical: `0, 0` → `0, 0, 0`; bootstrap_validate → `120*time.Second, 0, 10*time.Minute` — keep its explicit timeout/hook semantics, idle 0).

## 3. bootstrap.go Mode A doc targets (S2's DOCS scope ONLY)

- Env block :359: `#   STAGECOACH_TIMEOUT    generation timeout, e.g. "120s" or 120 (seconds)` → ADD an idle line after it (e.g. `#   STAGECOACH_IDLE_TIMEOUT   idle deadline, e.g. "45s" or 45 (seconds)`).
- Git block :375: `#   git config stagecoach.timeout 120s` → ADD `#   git config stagecoach.idleTimeout 45s` after it.
- NOT in S2: the template body `# timeout = "120s"` line (~L174, the idle_timeout template line + 600s rewrite is S3), the flag doc block (~L380-386, S3), and docs/*.md tables (S3).

## 4. Test twins (the symmetry rule from config_ladder.md §7)

"For every existing `*Timeout*` test name, add the `*IdleTimeout*` twin using DISTINCT sentinels (e.g.
45s idle override vs the 90s/60s/120s/600s used for timeout) so no assertion can pass through the wrong
field."
- load_test.go: TestLoadEnv idle twin (STAGECOACH_IDLE_TIMEOUT=45s → cfg.IdleTimeout==45s; malformed → error contains STAGECOACH_IDLE_TIMEOUT); per-role idle twin (STAGECOACH_PLANNER_IDLE_TIMEOUT=45s duration AND bare-int forms; malformed → error contains the var name). Existing harness: t.Setenv + loadEnvSetup (L84-93).
- git_test.go: found (`setGitConfig(t, repo, "stagecoach.idleTimeout", "45")` → cfg.IdleTimeout==45*time.Second — CAMELCASE key); missing (zero untouched); bad (TestLoadGitConfig_BadIdleTimeout — error contains "stagecoach.idleTimeout").
- file_test.go: [defaults].idle_timeout="45s" → materialized IdleTimeout==45s; [role.X].idle_timeout (both "45s" and bare "45" — parseTimeout); malformed → error contains "[role.X].idle_timeout"; bare-int at the GLOBAL file key ("45") REJECTED (time.ParseDuration) — pins the seam; overlay global guard (src.IdleTimeout wins / zero src preserves); overlay per-role field-merge (idle-only merge preserves Timeout sibling).

## 5. Scope boundaries (do NOT do)
- Do NOT add loadFlags idle reads ("idle-timeout" / role+"-idle-timeout") — that's S3's flag surface (item OUTPUT: "consumed by P1.M1.T1.S3 (flag surface)"; config_ladder.md item 3 lists them but the ITEM's LOGIC scopes S2 to env+git+file).
- Do NOT touch root.go (S3 flags), the bootstrap template body L174 (S3), docs/*.md (S3).
- Do NOT touch the executor / activity infra (P1.M2), stubagent (P1.M3), or ResolveRoleDeadlines/callers (S1/P1.M2).
- Do NOT bump CurrentConfigVersion (additive optional keys never bump — FR-B4).
- Do NOT add env/git/file keys for any OTHER field.

## 6. Validation

`go build ./...` (after S1), `go vet`, `gofmt -l internal/config/`, `make lint`,
`go test ./internal/config/... -v` (new twins + the ~16 signature-rippled call sites green), `make test`.
Grep guards: camelCase git key present (`stagecoach.idleTimeout`), no snake_case git key
(`stagecoach.idle_timeout` ABSENT from git.go), env var present, file key present.
