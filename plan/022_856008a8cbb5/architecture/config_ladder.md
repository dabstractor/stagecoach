# Configuration timeout ladder — end-to-end map (grounds spec v3.5)

Scope: READ-ONLY research at HEAD `df98a9a`. Purpose: ground the v3.5 changes (add `idle_timeout`
everywhere `timeout` exists, default 60s; hard-cap default 120s→600s; retire the planner 480s
built-in — spec/SPEC.md L28, the "This revision (v3.5)" row: idle deadline default 60s, hard cap
raised 120s→600s as backstop, "the v2.8 planner 480s special-case is retired as moot", both
deadlines resolve per role exactly like provider/model/reasoning, fresh idle clock per multi-turn
turn / work-description round).

Every line number below was verified against `df98a9a`. Verbatim excerpts are exact.

---

## 1. internal/config/config.go

### Config.Timeout field — L85 (inside `type Config struct {` at L80)

```go
	Timeout      time.Duration `toml:"timeout"`        // generation timeout; Defaults: 120s
```

Surrounding [defaults] block context (L82-89): Provider/Model/Reasoning/Timeout/AutoStageAll/Verbose
all plain-typed with trailing one-line comments.

### RoleConfig.Timeout — L42 (inside `type RoleConfig struct {` at L38)

```go
	Timeout   time.Duration `toml:"timeout"`   // per-role generation timeout (FR-R7); 0 ⇒ inherit global [defaults].timeout (toml tag is cosmetic — Config.Roles is toml:"-")
```

The RoleConfig doc comment (L21-37) states the inheritance discipline explicitly (L23-24):
"Any field "" (or Timeout 0) ⇒ the role inherits the global [defaults] (FR-R2); a non-empty/non-zero
value overrides just that field (FR-R3 field-merge across layers)."

### Where default 120s is set — L224 (inside `func Defaults() Config {` at L219)

```go
		Timeout:              120 * time.Second,
```

The `Defaults()` doc comment (L209-218) enumerates every built-in value and is the prose contract
for the hard-cap change:

```
// Defaults returns the built-in Layer-1 configuration (PRD §16.1): timeout 120s, auto_stage_all
// true, max_diff_bytes 300000, max_md_lines 100, max_duplicate_retries 3, subject_target_chars 50,
// max_commits 12. ...
```

### Doc-comment style on these fields

- Trailing single-line comments citing the PRD clause (`FR-R7`, `§16.x`, `§9.x`) on the field itself.
- Inheritance sentinels spelled out in the comment: `0 ⇒ inherit`, `"" ⇒ inherit`, `*bool — nil ⇒ inherit`.
- `Defaults:` annotations naming the built-in value (see L85's `; Defaults: 120s`).
- Big multi-line doc comments above types/functions (RoleConfig L21-37; Config L62-79; Defaults
  L209-218) explaining the design call (flat + plain-typed + RESOLVED) and which fields are
  toml:"-").
- A duration precedent for adding a *new* [generation]-style duration field exists: HookTimeout
  (field ~L170 `HookTimeout time.Duration \`toml:"hook_timeout"\``, default seed `10 * time.Minute`
  near L229) — the closest structural model for IdleTimeout.

### Where Config.IdleTimeout would naturally slot

1. Field: immediately after `Timeout` at L85 — `IdleTimeout time.Duration \`toml:"idle_timeout"\``
   with a `; Defaults: 60s` style trailing comment.
2. Default seed: immediately after L224 — `IdleTimeout: 60 * time.Second,`.
3. Prose: the Defaults() doc list (L210, "timeout 120s,") gains "idle_timeout 60s, timeout 600s"
   (the hard-cap change rewrites that sentence too).
4. RoleConfig.IdleTimeout: immediately after L42, mirroring the `0 ⇒ inherit` comment.

### Mechanism by which defaults are applied

`Defaults()` IS Layer 1. load.go L94: `cfg := Defaults() // Layer 1 (by value)` — the resolver starts
from the by-value Defaults snapshot, then each higher layer overwrites only what it carries:

- File layers (global, repo-local): decoded into separate intermediate structs (file.go fileConfig),
  materialized to a partial *Config, then merged by `overlay()` — which copies ONLY non-zero
  durations (file.go L391-392: `if src.Timeout != 0 { dst.Timeout = src.Timeout }`; per-role guard
  L503-508: `if rc.Timeout != 0 { existing.Timeout = rc.Timeout }`).
- Git layer: builds an ALL-ZERO partial `c := &Config{}` (git.go L112) and sets only found keys, so
  overlay's non-zero guards pass through exactly the user-set values.
- Env layer (load.go L262 `loadEnv`) and flag layer (load.go L417 `loadFlags`) set fields DIRECTLY on
  cfg (highest precedence; booleans can force false — the documented escape hatch).

There is no other place a built-in timeout default lives: everything funnels through `Defaults()` +
`defaultRoleTimeouts` (roles.go, below).

---

## 2. internal/config/roles.go AND internal/config/role_defaults.go

### defaultRoleTimeouts — roles.go L5-13 (planner 480s at L12)

```go
// defaultRoleTimeouts are the shipped per-role generation-timeout built-ins (PRD §16.1, FR-R7). The
// planner is the ONLY role with a built-in timeout (480s) — it does open-ended decomposition planning
// and needs longer than the message/stager/arbiter roles, which inherit cfg.Timeout. A user override
// (cfg.Roles[role].Timeout, set via [role.<role>].timeout / --<role>-timeout / env / git-config) ALWAYS
// beats this. This is a role×timeout axis, distinct from role_defaults.go's provider×role→MODEL table
// (FR-D4) — do not conflate the two. Add a role here only when shipping a non-global default.
var defaultRoleTimeouts = map[string]time.Duration{
	"planner": 480 * time.Second,
}
```

This map is the retirement target: v3.5 deletes it (planner 480s "retired as moot").

### ResolveRoleTimeout — full implementation, roles.go L96-104 (doc comment L70-95)

```go
func ResolveRoleTimeout(role string, cfg Config) time.Duration {
	if rc, ok := cfg.Roles[role]; ok && rc.Timeout != 0 {
		return rc.Timeout
	}
	if d, ok := defaultRoleTimeouts[role]; ok {
		return d
	}
	return cfg.Timeout
}
```

Exact precedence ladder (documented at L72-75):

```
//	[role.<role>].timeout  (CLI flag > env > file > git, all already merged into cfg.Roles by the loaders)
//	> built-in role default  (planner = 480s; FR-R7 — the planner needs more time than message/stager/arbiter)
//	> [defaults].timeout     (cfg.Timeout — the global; 120s)
```

Key documented properties (L82-95): the per-role entry is already fully merged by Load() (this
function never re-walks layers); a non-zero `cfg.Roles[role].Timeout` ALWAYS wins, even for planner;
zero-value sentinel mirrors ResolveRoleModel's "" fields; non-canonical role names miss both lookups
and inherit `cfg.Timeout` (no error). Companion resolver `ResolveRoleModel` sits at roles.go L46.

Post-v3.5 shape: delete `defaultRoleTimeouts` and the middle rung — the function collapses to
"per-role non-zero override, else cfg.Timeout" (and the idle twin needs no built-in rung at all).

### role_defaults.go — NOT a timeout site

`internal/config/role_defaults.go` is the FR-D4 provider×role→MODEL table only:
`DefaultModelsVerificationDate` const (L5-7), the FR-D4/FR-D5 verification block (L9-52),
`RoleModelDefaults` type (L54-62), the compiled `roleDefaults` map (pi/claude/agy/opencode/codex/
cursor × planner/stager/message/arbiter, L64-105), and `DefaultModelsForProvider(name)` copy
accessor (L107-118). Zero timeout content; roles.go L8-10 explicitly warns "do not conflate the two."

### All ResolveRoleTimeout callers (repo-wide grep at df98a9a)

Production call sites — 8 total, each resolving the deadline for ONE invocation:
- `internal/decompose/planner.go:69` — `plannerTimeout := config.ResolveRoleTimeout("planner", deps.Config)`
- `internal/decompose/stager.go:98` — `stagerTimeout := config.ResolveRoleTimeout("stager", deps.Config)`
- `internal/decompose/message.go:145` — `messageTimeout := config.ResolveRoleTimeout("message", deps.Config)`
- `internal/decompose/arbiter.go:83` — `arbiterTimeout := config.ResolveRoleTimeout("arbiter", deps.Config)`
- `internal/generate/generate.go:303` — `msgTimeout := config.ResolveRoleTimeout("message", cfg)` (single-commit path; comment L301 notes it replaces flat cfg.Timeout)
- `internal/generate/multiturn.go:153` — `msgTimeout := config.ResolveRoleTimeout("message", cfg)` (per-TURN bound, comment L150-152)
- `internal/generate/workdesc.go:71` — `msgTimeout := config.ResolveRoleTimeout("message", cfg)` (per read-loop round, comment L68-70)
- `internal/hook/exec.go:167` — `msgTimeout := config.ResolveRoleTimeout("message", cfg)` (hook one-shot, comment L164-165)

Non-production references: definition+docs in roles.go L70/L96; tests in
internal/config/roles_test.go (L195-310) and comments in internal/config/file.go:341,
internal/decompose/*_test.go, internal/generate/*_test.go, internal/hook/exec_test.go.
Flat `cfg.Timeout` direct reads (not via the resolver) appear only in comments (generate.go L87
ErrTimeout doc, multiturn.go L150, workdesc.go L68, exec.go L164) — the resolver is the single seam.

---

## 3. internal/config/load.go

### Layer assembly / overall precedence — Load() L88-170

Order applied to the Defaults() seed (L94), lowest→highest (comments at L103, L142, L156, L165, L170):

1. Layer 1 built-in Defaults (L94, by value)
2. Layer 2 global TOML — path from --config > STAGECOACH_CONFIG > discovery (L103-141; explicit-missing = hard error; absent+discovery triggers FR-B3 bootstrap write)
3. Layer 3 repo-local TOML `./.stagecoach.toml` (L142-147)
4. Layer 4 repo git config `stagecoach.*` (L156-161, via loadGitConfig → overlay)
5. Layer 5 STAGECOACH_* env vars, DIRECT set (L165-168)
6. Layer 7 (numbered "7 (6)" in the comment) CLI flags — ONLY fs.Changed ones (L170-172)

So: flag > env > git > repo-local file > global file > built-in. Validation happens once on the
RESOLVED value at the tail (format/template/diff_context/commits, L262-292) — the precedent for any
idle validation. `roleNames` (L17) is the canonical role list every loop iterates:

```go
var roleNames = []string{"planner", "stager", "message", "arbiter"}
```

### STAGECOACH_TIMEOUT env read — L272-278 (exact)

```go
	if v, ok := os.LookupEnv("STAGECOACH_TIMEOUT"); ok && v != "" {
		d, err := parseTimeout(v)
		if err != nil {
			return fmt.Errorf("STAGECOACH_TIMEOUT: %w", err)
		}
		cfg.Timeout = d
	}
```

Presence-semantic: present+non-empty overrides; unset/empty is a no-op. Malformed value = HARD error
wrapped with the var name (`VARNAME: %w` style).

### Per-role env — L304-326 (the loop), _TIMEOUT arm at L316-325 (exact)

```go
		// FR-R7 / §9.15 / §9.8 FR35 — per-role generation timeout via env (presence-semantic, DIRECT-set
		// via setRoleTimeout — bypasses overlay; mirrors the global STAGECOACH_TIMEOUT case). parseTimeout
		// accepts "480s" and bare "480" (cross-layer consistency with --timeout / stagecoach.timeout).
		if v, ok := os.LookupEnv(prefix + "_TIMEOUT"); ok && v != "" {
			d, err := parseTimeout(v)
			if err != nil {
				return fmt.Errorf("%s_TIMEOUT: %w", prefix, err)
			}
			cfg.setRoleTimeout(role, d)
		}
```

`prefix := "STAGECOACH_" + strings.ToUpper(role)` (L306). The writer is `setRoleTimeout` (L66-75,
func at L69): lazy map alloc + mandatory map-value-copy write-back (`rc := c.Roles[role]; rc.Timeout
= d; c.Roles[role] = rc`) so one field never clobbers siblings (FR-R3 field-merge).

### How the --timeout flag value flows in — loadFlags L417; global arm L433-439 (exact)

```go
	if fs.Changed("timeout") {
		if v, err := fs.GetString("timeout"); err == nil {
			if d, perr := parseTimeout(v); perr == nil {
				cfg.Timeout = d
			}
		}
	}
```

Per-role arm L466-479 is the same pattern through `setRoleTimeout` (comment L467-471: "Mirrors the
global --timeout flag handling above EXACTLY: Changed→GetString→parseTimeout→set. loadFlags has NO
error return, so a malformed value is silently ignored (same as the global --timeout); parseTimeout
accepts "600s" and bare "600"."). Asymmetry to preserve: env malformed = hard error; flag malformed
= silently ignored. Flags are pflag STRINGs registered in internal/cmd/root.go; the flag layer only
fires on `fs.Changed` (an unpassed flag's zero default never overrides).

### parseTimeout — L673-684 verbatim

```go
// parseTimeout parses a duration that may be EITHER a Go duration string ("120s", "2m") OR a bare
// integer (seconds: "120"). Used by both STAGECOACH_TIMEOUT (env) and --timeout (CLI). Returns a wrapped
// error if neither form parses.
func parseTimeout(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	if n, err := strconv.Atoi(s); err == nil {
		return time.Duration(n) * time.Second, nil
	}
	return 0, fmt.Errorf("invalid timeout %q (expected e.g. \"120s\" or 120)", s)
}
```

Error style: lower-case `invalid timeout %q (expected e.g. "120s" or 120)`, always wrapped by the
caller with the source name (`STAGECOACH_TIMEOUT: %w`, `git config stagecoach.timeout: %w`,
`parse config: [role.%s].timeout: %w`). Note the literal word "timeout" in the message — an idle
twin either reuses this verbatim (message stays "invalid timeout" even for idle values — acceptable,
callers wrap with the idle-specific name) or the message is generalized; pick one and stay
consistent. The FILE layer's global [defaults].timeout is the one duration that does NOT go through
parseTimeout — file.go L198-201 uses bare `time.ParseDuration` with its own wrapped error
(`parse config %s: invalid timeout %q: %w`); per-role file timeouts DO use parseTimeout (file.go
L347-350).

---

## 4. internal/config/git.go

### stagecoach.timeout — L147-156 (exact)

```go
	// --- timeout: accepts both "90" (seconds) and "90s" (Go duration) forms. ---
	if v, found, err := gitConfigGet(repoDir, "stagecoach.timeout"); err != nil {
		return nil, err
	} else if found {
		d, perr := parseTimeout(v) // parseTimeout handles both "90" and "90s"
		if perr != nil {
			return nil, fmt.Errorf("git config stagecoach.timeout: %w", perr)
		}
		c.Timeout = d
	}
```

### Per-role git keys — L158-177 (exact loop; doc comment L158-165)

```go
	// §9.15 FR-R7 / §9.8 FR36 / §16.1 layer 4 — per-role generation timeout via git config
	// (NEW infrastructure: git.go read NO per-role keys before this). Mirrors the global
	// stagecoach.timeout block above EXACTLY: gitConfigGet → parseTimeout → wrapped error →
	// setRoleTimeout. gitConfigGet maps a missing key (git exit 1) to found=false (no-op), so a
	// role with no stagecoach.role.<role>.timeout key is untouched. parseTimeout accepts "600s"
	// and bare "600". A malformed value is a HARD ERROR (loadGitConfig has an error return) —
	// the OPPOSITE of S2's loadFlags silent-ignore. Per-role provider/model/reasoning git keys
	// are intentionally NOT read here (file/env/flag only); this loop is timeout-only.
	for _, role := range roleNames {
		key := "stagecoach.role." + role + ".timeout"
		if v, found, err := gitConfigGet(repoDir, key); err != nil {
			return nil, err
		} else if found {
			d, perr := parseTimeout(v)
			if perr != nil {
				return nil, fmt.Errorf("git config %s: %w", key, perr)
			}
			c.setRoleTimeout(role, d)
		}
	}
```

Error wrapping style: `fmt.Errorf("git config <key>: %w", perr)` — the git key name itself is in the
message. Malformed = HARD error (opposite of the flag layer's silent ignore).

**CRITICAL naming constraint for the idle seam** (doc comment L103-104): "KEY NAMES ARE CAMELCASE
(FINDING A): git config rejects underscores ("invalid key")." So the git keys are
`stagecoach.idleTimeout` and `stagecoach.role.<role>.idleTimeout` — NOT snake_case, even though the
TOML/env spellings are `idle_timeout` / `STAGECOACH_IDLE_TIMEOUT`. (Single-word `timeout` never hit
this; two-word `idle_timeout` will.)

loadGitConfig contract (L97-112): returns a PARTIAL `*Config` with ONLY found keys set
(`c := &Config{}` at L112, all zero) — never pre-filled with Defaults(); missing key (git exit 1)
maps to found=false, not an error.

---

## 5. internal/config/bootstrap.go

### Generated template line — L174 (exact)

```go
	b.WriteString("# model          = \"\"\n# timeout        = \"120s\"\n# auto_stage_all = true\n# verbose        = false\n")
```

i.e. the emitted [defaults] block renders:

```
# model          = ""
# timeout        = "120s"
# auto_stage_all = true
# verbose        = false
```

This is where `# idle_timeout   = "60s"` slots (and where "120s" becomes "600s" for the hard cap).
Context: L165-174 build the populated [defaults] section (provider uncommented at L168, reasoning
written "off" at L173).

### Env-var doc comment block — L356-370 (STAGECOACH_TIMEOUT at L359)

```
# Environment variables — override this file, are overridden by CLI flags:
#   STAGECOACH_PROVIDER   default provider/agent (e.g. "pi", "claude", "agy")
#   STAGECOACH_MODEL      model override ("" -> provider manifest default_model)
#   STAGECOACH_TIMEOUT    generation timeout, e.g. "120s" or 120 (seconds)
#   STAGECOACH_CONFIG     path to a config file, overrides discovery
#   STAGECOACH_VERBOSE    "true"/"false" — print resolved command, raw output, retries
#   STAGECOACH_NO_COLOR   "true"/"false" — disable color (also honors NO_COLOR)
#   STAGECOACH_NO_PARENT_WATCHDOG=1   # opt out of the parent-death lock watchdog
#   STAGECOACH_PLANNER_PROVIDER / _MODEL   per-role override: decomposition planner
#   STAGECOACH_STAGER_PROVIDER  / _MODEL   per-role override: (tooled) staging agent
#   STAGECOACH_MESSAGE_PROVIDER / _MODEL   per-role override: bare commit-message agent
#   STAGECOACH_ARBITER_PROVIDER / _MODEL   per-role override: leftover arbiter
#   STAGECOACH_REASONING                  global reasoning effort: off|low|medium|high
#   STAGECOACH_<ROLE>_REASONING           per-role reasoning override (role = planner|stager|message|arbiter)
#   STAGECOACH_COMMITS                    force exactly N commits when nothing is staged; 1 == --single
```

**Existing gap**: the per-role `STAGECOACH_<ROLE>_TIMEOUT` env vars are NOT documented in this
header (nor are `STAGECOACH_PLANNER_MODEL`-style singles beyond the four lines above). An idle
addition should add its line after L359 and ideally close the timeout gap while there.

### Git-key doc comment block — L372-377 (stagecoach.timeout at L375)

```
# Git config keys — alternative to this file, scoped to one repo:
#   git config stagecoach.provider pi
#   git config stagecoach.model ""
#   git config stagecoach.timeout 120s
#   git config stagecoach.autoStageAll true
#   (read via `git config --get stagecoach.<key>`)
```

### CLI flag doc block — L380-386

Lists only --provider/--model/--<role>-provider/--<role>-model/--commits/--single/--max-commits.
**Existing gap**: neither `--timeout` nor the four `--<role>-timeout` flags are documented in the
generated header. (Also note L376 documents camelCase `autoStageAll` — the header already shows the
camelCase convention the idle git key must follow.)

Separately, GenerationSection (L390+) is the shared [generation] doc block for both templates —
timeout lives in [defaults], so idle's template prose belongs at L174/L359/L375, not there.

---

## 6. internal/cmd/root.go

### --timeout flag registration — L169 (exact)

```go
	pf.StringVar(&flagTimeout, "timeout", "", "Generation timeout, e.g. \"120s\" or 120 (env STAGECOACH_TIMEOUT, git stagecoach.timeout; default 120s)")
```

Registered on the root's PersistentFlags (L164 `pf := rootCmd.PersistentFlags()`), in the
"§15.2 config-backed flags (zero defaults; config.Load owns Layer-7 precedence via fs.Changed)"
block (L165). Help text embeds the default — the 120s→600s change must rewrite it.

### The four per-role flags — L258-266 (exact)

```go
	// §9.15 FR-R7 / §15.2 — per-role generation timeout flags (string, zero default; loadFlags reads via fs.Changed).
	pf.StringVar(&flagPlannerTimeout, "planner-timeout", "",
		"Per-role generation timeout for the decomposition planner, e.g. \"600s\" or 600 (env STAGECOACH_PLANNER_TIMEOUT; git stagecoach.role.planner.timeout)")
	pf.StringVar(&flagStagerTimeout, "stager-timeout", "",
		"Per-role generation timeout for the (tooled) staging agent, e.g. \"300s\" or 300 (env STAGECOACH_STAGER_TIMEOUT; git stagecoach.role.stager.timeout)")
	pf.StringVar(&flagMessageTimeout, "message-timeout", "",
		"Per-role generation timeout for the message composer, e.g. \"120s\" or 120 (env STAGECOACH_MESSAGE_TIMEOUT; git stagecoach.role.message.timeout)")
	pf.StringVar(&flagArbiterTimeout, "arbiter-timeout", "",
		"Per-role generation timeout for the leftover arbiter, e.g. \"120s\" or 120 (env STAGECOACH_ARBITER_TIMEOUT; git stagecoach.role.arbiter.timeout)")
```

### Flag value type and where parsed

STRING, not pflag Duration. Storage vars: `flagTimeout string` at L54 with the comment
`// STRING — config.Load reads via fs.GetString("timeout") (FINDING 7)`, inside the block comment
L48-49: "Config-backed flags (resolved by config.Load via fs.Changed; registered at ZERO default so
Changed reflects 'user passed it'). See design §2 (timeout is a STRING) and §3 (zero defaults)."
Per-role vars at L81-84 (`flagPlannerTimeout/flagStagerTimeout/flagMessageTimeout/flagArbiterTimeout
string`). Parsing happens ONLY in config.loadFlags (load.go L433-439 global, L466-479 per-role) via
`fs.GetString` + `parseTimeout`; root.go never parses. An `--idle-timeout` flag must follow the
identical string/zero-default contract.

---

## 7. Test conventions (config_test.go, roles_test.go, load_test.go, git_test.go)

- **config_test.go** — plain assertions on `Defaults()` (L20-21):
  `if c.Timeout != 120*time.Second { t.Errorf("Timeout = %v, want 120s", c.Timeout) }`.
  Idle additions: assert `IdleTimeout == 60s` and the new 600s cap symmetric to this.
- **roles_test.go** — named per-case tests, not table-driven (TestResolveRoleTimeout_PerRoleOverride
  L197, _PlannerBuiltinBeatsGlobal L207, _NonPlannerGlobalFallback L217, _FieldMergeTimeoutOnly
  L228, _UnknownRoleGlobalFallback L238, _RolesNilGlobalFallback L247, _MessagePerRoleOverride L261,
  _AllCanonicalRoles L277 — the one loop-over-roles test — _PlannerDefaultFromDefaults L304).
  Disciplines: DISTINCT sentinel values per rung (global 120s vs built-in 480s vs override 600s so
  an assertion can't pass via the wrong rung); explanatory `[bracketed rationale]` inside error
  strings; GAP-closing doc comments naming the closed gap. v3.5 rewrite: the 480s tests become
  retirement tests (planner inherits global; per-role override still wins).
- **load_test.go** — env via `t.Setenv` (e.g. TestLoadEnv_StringsTimeoutBools L145 sets
  STAGECOACH_TIMEOUT=60s; TestLoadEnv_PerRoleTimeout L368 sets duration AND bare-int forms plus a
  same-role `_PROVIDER` to prove field-merge; bad-value tests assert the error CONTAINS the var
  name, L390-400). Flags via the `newFlagSet(t)` helper (L49-79): a fresh pflag.FlagSet with every
  Config-backed flag pre-registered at zero defaults — including `fs.String(role+"-timeout", "", "")`
  in the roleNames loop at L65 — then `fs.Set("planner-timeout", "600s")` marks Changed
  (TestLoadFlags_TimeoutString L506, TestLoadFlags_PerRoleTimeout L539,
  TestLoadFlags_PerRoleTimeout_MalformedIgnored L567 — pins the silent-ignore asymmetry). Full-Load
  precedence via `Load(ctx, LoadOpts{RepoDir: repo, Flags: fs})` after `loadEnvSetup(t)` + `chdir`
  (TestLoad_PerRoleTimeout_FlagBeatsEnv L1145-1160 — the template for flag>env assertions).
  parseTimeout itself is table-driven (TestParseTimeout_DurationAndSeconds L99, TestParseTimeout_
  Invalid L123). Isolation: loadEnvSetup (L84-93) temp HOME/XDG_CONFIG_HOME + `initRepo` git repo.
- **git_test.go** — real temp repo + `setGitConfig(t, repo, "stagecoach.timeout", "90")` helper;
  TestLoadGitConfig asserts `cfg.Timeout != 90*time.Second` fails (L107-108); a missing-key test
  asserts untouched zero (L166); TestLoadGitConfig_BadTimeout (L211-222) asserts non-nil error
  containing "stagecoach.timeout". Mirror all three for `stagecoach.idleTimeout` (camelCase key!).
- **file_test.go / merge_test.go** — the file-layer loadTOML/materialize/overlay home; the
  duration-non-zero overlay guards (`if src.Timeout != 0`, L391-392; `if rc.Timeout != 0`, L507)
  get idle twins here. (Note: hook_timeout has no file_test coverage — durations are under-tested
  at the file layer; don't replicate that for idle.)

Symmetry rule for idle variants: for every existing `*Timeout*` test name, add the `*IdleTimeout*`
twin using distinct sentinels (idle 60s default vs 600s hard cap vs e.g. 45s override) so no
assertion can pass through the wrong field.

---

## 8. Complete idle-seam checklist (every distinct place) + drift notes

Config package (the subsystem proper):

1. **config.go** — `Config.IdleTimeout` field after L85 (comment `; Defaults: 60s`); seed
   `IdleTimeout: 60 * time.Second,` after L224; rewrite the Defaults() doc sentence (L210) for
   60s idle + 600s cap; `RoleConfig.IdleTimeout` after L42 with `0 ⇒ inherit` comment; extend the
   RoleConfig doc comment's inheritance sentence (L23-24).
2. **file.go** — `fileConfig` [defaults] string field (~L56 pattern: `IdleTimeout string
   \`toml:"idle_timeout"\``); `fileRoleConfig.IdleTimeout` (~L28); up-front parse in loadTOML
   (~L196-215, mirroring Timeout/hook_timeout, error `parse config %s: invalid idle_timeout %q: %w`);
   `materialize` signature + `c := &Config{IdleTimeout: idleTimeout, ...}` (~L234-235); per-role
   parse + error `parse config: [role.%s].idle_timeout: %w` (~L347-350, L358); overlay global guard
   `if src.IdleTimeout != 0` (L391-392); overlayRoles per-role guard (L503-508).
3. **load.go** — `setRoleIdleTimeout` helper (mirror L66-75); `STAGECOACH_IDLE_TIMEOUT` env
   (mirror L272-278, error `STAGECOACH_IDLE_TIMEOUT: %w`); per-role `prefix + "_IDLE_TIMEOUT"`
   (mirror L316-325); loadFlags global `"idle-timeout"` (mirror L433-439); per-role
   `role + "-idle-timeout"` (mirror L466-479); reuse parseTimeout (decide: keep its "invalid
   timeout" wording or generalize).
4. **git.go** — `stagecoach.idleTimeout` block (mirror L147-156; CAMELCASE per FINDING A — git
   rejects `stagecoach.idle_timeout`); per-role `stagecoach.role.<role>.idleTimeout` loop entries
   (extend L166-177); errors `git config stagecoach.idleTimeout: %w` / `git config %s: %w`.
5. **roles.go** — add `ResolveRoleIdleTimeout` (two-rung ladder: per-role non-zero → cfg.IdleTimeout,
   NO built-in rung); DELETE `defaultRoleTimeouts` (L5-13) and collapse `ResolveRoleTimeout`'s
   middle rung (L99-101) per the v3.5 retirement; update both doc comments (L5-13, L70-95).
   (role_defaults.go needs NO idle change.)
6. **cmd/root.go** — `flagIdleTimeout string` var (L54 block); `--idle-timeout` StringVar after L169
   (help: `"Idle deadline, e.g. \"60s\" or 60 (env STAGECOACH_IDLE_TIMEOUT, git stagecoach.idleTimeout; default 60s)"`);
   four `--<role>-idle-timeout` StringVars (mirror L258-266) + vars after L84; update the L169
   `--timeout` help's `default 120s` → `default 600s`.
7. **bootstrap.go** — template line L174 (add `# idle_timeout   = "60s"`, change `"120s"`→`"600s"`);
   env doc after L359; git doc after L375 (`git config stagecoach.idleTimeout 60s`); close the two
   pre-existing header gaps if in scope (per-role _TIMEOUT env vars absent from L356-370;
   --timeout/--<role>-timeout flags absent from L380-386).
8. **Tests** — config_test defaults; roles_test ladder + 480s-retirement; load_test env/flag/
   per-role/malformed-ignored/flag-beats-env; git_test found/missing/bad (camelCase key);
   file_test/merge_test overlay guards; load_test newFlagSet gains `fs.String(role+"-idle-timeout",
   "", "")` + global `"idle-timeout"` registration (L56-66).

Outside the config package (consumer side, listed for completeness):

9. **The 8 ResolveRoleTimeout call sites** (§2 list) each need an idle counterpart; v3.5 requires a
   FRESH idle clock per invocation — multiturn.go:153 is already per-turn and workdesc.go:71 per
   round, matching the spec's "fresh idle clock per multi-turn turn / work-description round".

Docs (out of code but will drift the moment defaults change):

10. **docs/cli.md** L27 (--timeout row: default "120s", STAGECOACH_TIMEOUT, stagecoach.timeout) and
    the per-role rows L62-63 (and the planner/stager rows nearby); **docs/configuration.md** L91
    (template comment incl. "planner role defaults to 480s"), L138 (defaults table `timeout 120s`),
    L213-215 (env table), L232 (git example `timeout = 120s`), L264 (planner-480s prose). Every
    "120s"/"480s" string in docs is a change site for 600s/60s/retirement.

### Drift from the brief's line anchors — none material

All anchors verified exact at df98a9a: config.go ~L224 = L224 exact; load.go ~L272 = L272-278;
per-role env ~L317+ = L316-325; parseTimeout ~L674 = L673-684 (func at L676); git.go ~L148 = L147-156
+ per-role L158-177; bootstrap ~L174 = L174 exact, env block ~L359 = L359, git block ~L375 = L375;
root.go ~L169 = L169 exact, per-role ~L259-266 = L258-266. Two discoveries beyond the brief: the
bootstrap header's missing timeout-flag/env documentation (item 7), and that the file layer's global
timeout parse uses bare time.ParseDuration (file.go L198-201), not parseTimeout — the one spot
where an idle twin must consciously choose which parser to mirror (parseTimeout is the
cross-layer-consistent choice; note file.go L198's error wording differs).
