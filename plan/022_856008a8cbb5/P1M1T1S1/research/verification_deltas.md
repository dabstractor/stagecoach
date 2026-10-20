# Research Notes — P1.M1.T1.S1 (IdleTimeout fields, 600s default, ResolveRoleDeadlines, planner 480s deleted)

Verification against the CURRENT working tree. The task description + `architecture/config_ladder.md`
§1–2 are line-for-line accurate. These notes record the verified anchors + the COMPLETE break-list.

## VERIFIED — config.go anchors
- **L85**: `Timeout      time.Duration \`toml:"timeout"\`        // generation timeout; Defaults: 120s`
  (inside `type Config struct` at L80; [defaults] block L82-89)
- **L210**: Defaults() doc list first line: "timeout 120s, auto_stage_all true, ..." — the prose contract
  to rewrite to "idle_timeout 60s, timeout 600s, ..."
- **L224**: `Timeout:              120 * time.Second,` inside `func Defaults() Config` (L219)
- **RoleConfig** L38-43; **L42**: `Timeout   time.Duration \`toml:"timeout"\`   // per-role generation
  timeout (FR-R7); 0 ⇒ inherit global [defaults].timeout (toml tag is cosmetic — Config.Roles is toml:"-")`
- **HookTimeout** (~L170 field, ~L229 seed) is the structural precedent for adding a new duration field.
- `time` already imported. NO new imports.

## VERIFIED — roles.go anchors
- **L5-13**: defaultRoleTimeouts doc + `var defaultRoleTimeouts = map[string]time.Duration{"planner": 480 * time.Second,}`
- **L70-95**: ResolveRoleTimeout doc (the ladder: per-role → built-in → global; heavy planner-480s prose)
- **L96-104**: the function:
  ```go
  func ResolveRoleTimeout(role string, cfg Config) time.Duration {
      if rc, ok := cfg.Roles[role]; ok && rc.Timeout != 0 { return rc.Timeout }
      if d, ok := defaultRoleTimeouts[role]; ok { return d }
      return cfg.Timeout
  }
  ```
- `defaultRoleTimeouts` is referenced ONLY in roles.go (L5/L11/L100 + doc) — deleting it touches
  nothing else. file.go:341 mentions ResolveRoleTimeout in a comment only.

## VERIFIED — ResolveRoleTimeout's 11 production callers (KEEP the function — deleting it breaks compile)
arbiter.go:83, message.go:145, planner.go:69, stager.go:98, generate.go:303, multiturn.go:153,
workdesc.go:71, hook/exec.go:167 (message×4 incl. one comment-only cite). The task allows "keep or
replace as a thin hard-cap accessor" — KEEP it (thin: per-role override → cfg.Timeout, no built-in)
so all 11 callers keep compiling; P1.M2.T1.S3 later threads them to ResolveRoleDeadlines.

## THE DESIGN (task-specified)
(a) `Config.IdleTimeout time.Duration \`toml:"idle_timeout"\`` immediately after L85
    (`; Defaults: 60s` style comment, two-deadline contract: idle = primary productivity signal,
    hard cap = absolute backstop, §9.30). `RoleConfig.IdleTimeout` after L42 mirroring `0 ⇒ inherit`.
(b) Defaults(): L224 `120*time.Second` → `600*time.Second`; add `IdleTimeout: 60 * time.Second,`
    immediately after; rewrite the L210 doc-list sentence.
(c) DELETE defaultRoleTimeouts (L5-13) entirely; delete the built-in branch (L100-102) from
    ResolveRoleTimeout; rewrite both doc comments (NO role ships a built-in deadline; the v2.8
    planner 480s existed because a 120s global starved a thinking planner — retired as moot under
    idle+600s).
(d) ADD exported `ResolveRoleDeadlines(role string, cfg Config) (idle, hard time.Duration)` — the
    existing ladder MINUS the built-in tier; both deadlines resolve INDEPENDENTLY:
  ```go
  func ResolveRoleDeadlines(role string, cfg Config) (idle, hard time.Duration) {
      idle, hard = cfg.IdleTimeout, cfg.Timeout
      if rc, ok := cfg.Roles[role]; ok {
          if rc.IdleTimeout != 0 { idle = rc.IdleTimeout }
          if rc.Timeout != 0     { hard = rc.Timeout }
      }
      return idle, hard
  }
  ```
  (task signature: two return values, not a struct). Defaults() seeds 60s/600s so the globals are
  non-zero in practice; a zero-value cfg keeps the existing leniency (returns 0s, no error).
(e) KEEP ResolveRoleTimeout as the thin hard-cap accessor — either inline the two-line ladder or
  delegate: `_, hard := ResolveRoleDeadlines(role, cfg); return hard` (single source of truth;
  RECOMMENDED). All 11 callers unchanged.

## THE COMPLETE BREAK-LIST (tests that MUST change in S1)
| Test | Line(s) | Breaks because | Fix |
|------|---------|----------------|-----|
| config_test.go TestDefaults (Timeout pin) | 20-21 | pins Defaults().Timeout==120s | → 600s (+ add IdleTimeout==60s assert) |
| load_test.go default assertion | 712-713 | pins resolved cfg.Timeout==120s (default) | → 600s |
| roles_test.go TestResolveRoleTimeout_PlannerBuiltinBeatsGlobal | 207-215 | asserts planner→480s BUILT-IN beats 120s global | REWRITE: planner now inherits global (120s in that test's explicit cfg) — rename/repurpose (e.g. _PlannerInheritsGlobal) |
| roles_test.go TestResolveRoleTimeout_PlannerDefaultFromDefaults | 304-311 | asserts Defaults().Timeout==120s AND planner→480s | REWRITE: Defaults().Timeout==600s; planner→600s (inherits; no built-in) |
| roles_test.go comments citing the 480s built-in | 199, 209, 210, 219, 274, 279, 281 | stale prose after retirement | comment-only updates (assertions still pass) |

**DOES NOT BREAK (verified — they set 120s EXPLICITLY as a test value, not via the default):**
roles_test.go :217 NonPlannerGlobalFallback, :228 FieldMergeTimeoutOnly, :238 UnknownRoleGlobalFallback,
:247 RolesNilGlobalFallback, :261 MessagePerRoleOverride, :277 AllCanonicalRoles (planner override 600s
still wins; message→120s global still); file_test.go :700/:731 (explicit arbiter 120s round-trip);
load_test.go :104-105 (parseTimeout table — tests the parse helper); bootstrap_validate_test.go:68
(explicit arg). **file_test.go:898** (`dst.Timeout != 120*time.Second`) — VERIFY the context: if dst
was constructed with an explicit 120s it passes; if dst := Defaults() it breaks → update to 600s.
**Outside internal/config: ZERO 120s/480s pins** (verified by grep across internal/ pkg/ cmd/ tests).

## NEW TESTS (task-specified)
- Defaults: Timeout==600s AND IdleTimeout==60s.
- ResolveRoleDeadlines: per-role override beats global for BOTH deadlines; override of ONE leaves the
  other at global (independent resolution); zero-value RoleConfig field inherits global; unknown role /
  nil Roles → globals; ResolveRoleTimeout ≡ ResolveRoleDeadlines' hard return (delegation contract).
- Planner retirement: ResolveRoleTimeout("planner", Defaults()) == 600s (was 480s).

## COMPILE-SAFETY of the field additions
RoleConfig is NEVER decoded from TOML (Config.Roles is toml:"-"); every RoleConfig literal in the tree
is named-field (decompose/roles.go setRole, config/file.go materialize, tests) ⇒ adding fields is
compile-safe. fileRoleConfig (the TOML twin) does NOT gain idle_timeout in S1 — that's S2 (the file
layer). The overlay role-merge guard (`if rc.Timeout != 0`) gains an IdleTimeout guard in S2; until
then a file-layer idle_timeout simply doesn't merge (deliberate split, not a bug).

## SCOPE BOUNDARIES (sibling subtasks — do NOT implement here)
- **P1.M1.T1.S2**: the precedence layers — STAGECOACH_IDLE_TIMEOUT env, stagecoach.idleTimeout git key,
  [defaults].idle_timeout + [role.X].idle_timeout file keys (fileRoleConfig field + materialize/overlay
  guards) + per-role variants. load.go/file.go/git.go UNTOUCHED in S1.
- **P1.M1.T1.S3**: CLI flags (--idle-timeout + 4 per-role), bootstrap template text, docs (cli.md,
  configuration.md). bootstrap.go UNTOUCHED in S1 (its `# timeout = "120s"` template comment is S3's).
- **P1.M2.T1.S1-S3**: activity infra, Execute rework, threading the 13 call sites.
- Do NOT: delete ResolveRoleTimeout (11 callers break); add env/flag/git/file plumbing (S2/S3); touch
  the executor; or bump CurrentConfigVersion (additive optional keys — FR-B4).
