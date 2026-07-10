name: "P1.M2.T2.S1 — Change global default 480s→120s + fix all pinning tests (FR-R7 global fallback)"
description: >
  Flip the Layer-1 built-in global generation timeout from 480s to 120s (`config.go` `Defaults().Timeout`), then sweep and fix every
  mirror of that default (4 source comments + 1 generated-template line + 1 CLI help text) and every test that PINS the old 480s
  default (5 test sites across config_test.go / file_test.go / load_test.go / root_test.go), plus the 3 docs rows that state the
  default (docs/cli.md, docs/configuration.md). The planner is UNAFFECTED because P1.M2.T1.S1 already landed its 480s BUILT-IN in
  `defaultRoleTimeouts` (roles.go:12), which `ResolveRoleTimeout` returns for the planner role regardless of `cfg.Timeout` — that
  built-in + its tests + the parseTimeout "480s" examples + the arbitrary-value merge/role tests all STAY 480s. This is a surgical,
  grep-driven find-and-replace: the whole risk is (a) not missing a pinning test (root_test.go:246 was omitted from the contract's
  line list due to drift — caught by the grep sweep) and (b) NOT over-reaching into the planner built-in or the parser examples.
  No new code, no new types, no consumer wiring (P1.M3), no migration, no config-version bump. Docs configuration.md rides with the
  work per the contract; README has no 480 reference (verified).

---

## Goal

**Feature Goal**: Lower the global generation-timeout default (Layer-1 `config.Defaults().Timeout`) from 480s to 120s so the
message/stager/arbiter roles get a tighter bound, while the planner keeps its 480s headroom via the role-specific built-in that
P1.M2.T1.S1 already shipped. Every mirror of the old 480s default — source comments, the generated config template, the `--timeout`
help text, the docs tables, and the 5 tests that pin the default — is updated in lockstep so the build is green and the
documentation/CLI surface stops advertising 480s.

**Deliverable** (surgical edits, no new files):
1. **`internal/config/config.go`** — `Timeout: 120 * time.Second` in `Defaults()` (was 480) + 2 comment fixes (field doc :71, `Defaults` godoc :183).
2. **`internal/config/bootstrap.go`** — generated-template comment `# timeout = "120s"` (was "480s") at :161.
3. **`internal/cmd/root.go`** — `--timeout` help text `default 120s` (was 480s) at :169.
4. **5 default-pinning test fixes** — `config_test.go:20-21`, `file_test.go:898` (+ comment :113), `load_test.go:693-694`,
   `root_test.go:246-247`: each `480*time.Second`/`want 480s` → `120*time.Second`/`want 120s`.
5. **3 source-comment accuracy fixes** — `executor.go:28`, `models.go:144`, `pkg/stagecoach/stagecoach.go:40` (each describes the default).
6. **3 docs fixes** — `docs/cli.md:27`, `docs/configuration.md:86`, `docs/configuration.md:133` (default-value rows).
7. **2 integration_real comment fixes** — `realagent_test.go:62,136` (describe the default; behind `integration_real` tag, never run, but accurate).
8. **1 godoc accuracy fix** — `roles.go:75` comment "(global; 480s today, 120s after P1.M2.T2)" → "(global; 120s)". COMMENT ONLY — the
   planner built-in value on `roles.go:12` is UNCHANGED.

**Success Definition**:
- `config.Defaults().Timeout == 120 * time.Second`.
- `make test` (race) green: the 5 default-pinning tests now assert 120s; the planner built-in tests (`roles_test.go`,
  `load_test.go` planner tests, `file_test.go` role/merge/parseTimeout tests) are UNCHANGED and still green.
- `ResolveRoleTimeout("planner", Defaults())` still returns `480 * time.Second` (built-in beats the new 120s global — verified by the
  existing `TestResolveRoleTimeout_PlannerBuiltinBeatsGlobal`).
- `stagecoach --help` shows `default 120s` for `--timeout`; `stagecoach config init --template` (or the generated config) shows
  `# timeout = "120s"`; `docs/configuration.md` + `docs/cli.md` default rows say 120s.
- `make lint` + `make coverage-gate` green; `gofmt -l` empty; cross-build clean; `git diff` touches only the files in the
  Deliverable (no planner built-in change, no consumer wiring).

## User Persona (if applicable)

**Target User**: Every stagecoach user — the global timeout is the out-of-the-box bound on a single generation attempt for the
message/stager/arbiter roles. (The planner role is the exception, via its built-in.)

**Use Case**: A user runs `stagecoach` with no `--timeout` / no `[defaults] timeout` config. Today that bounds each attempt at 480s
(8 min) — generous, but most message-role runs finish well under 2 min, and a hung agent shouldn't occupy a lock for 8 min. 120s is
a tighter, safer default; users who need longer set `[defaults] timeout = "300s"` (file beats Layer-1) or `--timeout 300s`.

**User Journey**: `stagecoach` (no config) → resolves `cfg.Timeout = 120s` (Layer-1) → each Execute attempt bounded at 120s (message
role). If it times out → exit 124. (The planner, when decomposition runs, gets 480s via its built-in — unaffected by this change.)

**Pain Points Addressed**: An 8-minute default was too loose for the cheap message role; it held the per-repo lock (§18.5) for too long
on a hung agent. 120s is the new baseline; the planner keeps its headroom.

## Why

- **FR-R7 / §16.1 / §15.2**: the PRD states the global fallback is `120s` ("timeout 120s (global fallback for every role; **planner
  role default 480s**)"). The code currently ships 480s globally (a pre-split value). This task aligns the code with the PRD's stated
  120s, relying on P1.M2.T1.S1's planner built-in to preserve the planner's 480s.
- **Decoupled from the planner**: because `ResolveRoleTimeout` now returns the planner built-in (480s) regardless of `cfg.Timeout`,
  this global flip is SAFE — it tightens only the message/stager/arbiter roles. The split (T1.S1 built-in, then T2.S1 global) exists
  precisely so this flip doesn't regress the planner.
- **Bounded scope**: a grep-driven find-and-replace with a clear CHANGE list and an equally important KEEP list (the planner built-in,
  the parseTimeout examples, the arbitrary-value tests). No new code, no consumer, no migration.

## What

**User-visible behavior**: `stagecoach` with no timeout config now bounds each (non-planner) attempt at 120s instead of 480s. The
`--timeout` help, the generated config template, and the docs all say 120s.

**Technical change**: one literal flip (`480` → `120` in `Defaults().Timeout`) + a coordinated sweep of its mirrors, pinning tests,
default-describing comments, and docs rows. See the Implementation Blueprint for the exact before/after at every site.

### Success Criteria
- [ ] `config.go` `Defaults()` returns `Timeout: 120 * time.Second` (was `480`).
- [ ] `config.go:71` field comment + `config.go:183` `Defaults` godoc say `120s` (was `480s`).
- [ ] `bootstrap.go:161` generated-template comment is `# timeout = "120s"`.
- [ ] `root.go:169` `--timeout` help text says `default 120s`.
- [ ] `config_test.go:20-21`, `file_test.go:898` (+ comment `:113`), `load_test.go:693-694`, `root_test.go:246-247` all assert `120s`.
- [ ] `executor.go:28`, `models.go:144`, `pkg/stagecoach/stagecoach.go:40`, `realagent_test.go:62,136` comments say `120s`.
- [ ] `roles.go:75` godoc says "(global; 120s)" (was "480s today, 120s after P1.M2.T2"). Planner built-in `roles.go:12` UNCHANGED (480s).
- [ ] `docs/cli.md:27`, `docs/configuration.md:86`, `docs/configuration.md:133` say `120s`.
- [ ] `make test` (race) green; `make lint` + `make coverage-gate` green; `gofmt -l` empty; cross-build clean.
- [ ] Planner built-in (`roles.go:12`) still `480 * time.Second`; `ResolveRoleTimeout("planner", Defaults())` still returns 480s.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact before/after for every one of the ~17 edit sites (grouped into CHANGE / KEEP with line numbers and verbatim
strings), the verified grep sweep (`grep -rn '480' internal/ cmd/ pkg/ docs/`), the reason the planner is unaffected (the already-landed
`defaultRoleTimeouts` built-in + `ResolveRoleTimeout` precedence), the 5 default-pinning tests enumerated (including `root_test.go:246`,
which the contract's line-number list omitted due to drift), the explicit KEEP list so the implementer does not over-reach into the
planner built-in or the parseTimeout examples or the arbitrary-value merge/role tests, and the verified validation commands.

### Documentation & References

```yaml
# MUST READ — the authoritative research (the categorized CHANGE/KEEP table + the rationale)
- docfile: plan/015_b461e4720495/P1M2T2S1/research/findings.md
  why: "§1 the core change + 4 direct mirrors; §2 the 5 default-pinning tests (incl. root_test.go:246 caught by the sweep); §3 the
        default-describing comments; §4 the 3 docs rows; §5/§6/§7 the KEEP lists (planner built-in, parseTimeout examples, arbitrary-
        value tests) — as important as the CHANGE list; §8 why it's planner-safe; §9 validation; §10 the single subtlety."
  critical: "§5: roles.go:12 (the planner built-in) STAYS 480s. §7: file_test.go:577/699/1278 + load_test.go planner tests use 480 as
             an ARBITRARY value, not the default — LEAVE them. §2: root_test.go:246 is a default-pinning test the contract list missed."

# MUST READ — the planner built-in + ResolveRoleTimeout (already LANDED; consume, don't break it)
- docfile: plan/015_b461e4720495/P1M2T1S1/PRP.md
  why: "Defines defaultRoleTimeouts + ResolveRoleTimeout (the 3-tier precedence: per-role > built-in(planner 480s) > global). Proves
        the global flip is planner-safe: ResolveRoleTimeout('planner', cfg) returns the 480s built-in regardless of cfg.Timeout."
  critical: "Do NOT change roles.go:12 ('planner': 480 * time.Second). That is the planner built-in; lowering it would undo T1.S1."

# MUST READ — Finding 5 (the original test list) + the plan's research
- docfile: plan/015_b461e4720495/architecture/critical_findings.md
  why: "Finding 5 lists the default-pinning tests (config_test.go:20-21, file_test.go:113+787, load_test.go:589-590, root.go:133,
        bootstrap.go:161). NOTE: line numbers there are PRE-T1.S1-drift; the CURRENT lines are in findings.md §1/§2 (verified by grep)."
  critical: "Finding 5's list is a STARTING point, not exhaustive — root_test.go:246 + the 3 source comments + 3 docs rows + 2
             realagent comments are additional sites found by the grep sweep. Anchor edits by STRING (grep), not by Finding 5's lines."

# MUST EDIT — the core default + its source mirrors
- file: internal/config/config.go
  why: "Defaults() (~:200) is THE global default. Field doc (~:71) + Defaults godoc (~:183) describe it. All three → 120s."
  pattern: "`Timeout: 480 * time.Second,` (the only such line in Defaults) → `Timeout: 120 * time.Second,`."
  gotcha: "There is ALSO `480 * time.Second` in roles.go:12 (the planner built-in) — do NOT touch that one. Anchor by the Defaults() literal."

- file: internal/config/bootstrap.go
  why: ":161 is the generated config template's commented [defaults] line: `# timeout = \"480s\"`. → `\"120s\"`."
  gotcha: "bootstrap_test.go has ZERO references to 480/timeout (verified) — this commented line has no byte-exact test. Safe to change."

- file: internal/cmd/root.go
  why: ":169 is the --timeout flag help string ending `default 480s)`. → `default 120s)`."

# MUST EDIT — the 5 default-pinning tests (each asserts the resolved global default)
- file: internal/config/config_test.go   # :20-21  TestDefaults: c.Timeout != 480 → 120
- file: internal/config/file_test.go     # :898 TestOverlayNilSrc: dst.Timeout (dst=Defaults()) != 480 → 120; :113 comment → 120s
- file: internal/config/load_test.go     # :693-694: cfg.Timeout != 480 → 120
- file: internal/cmd/root_test.go        # :246-247: cfg.Timeout != 480 → 120  (NOT in the contract list — caught by grep)
  why: "These assert config.Defaults().Timeout (or a Load that falls back to it). They FAIL after the default flips unless updated."
  gotcha: "DISTINGUISH from the arbitrary-value tests in the SAME files (file_test.go:577/699/1278, load_test.go:327/370/1129) which use
           480 as a deliberate per-role/parser/merge value — LEAVE those. The default-pinning tests are the ones that call Defaults()
           or Load-with-no-timeout and assert the GLOBAL value. See Anti-Patterns."

# SHOULD EDIT — comments that describe the default (accuracy)
- file: internal/provider/executor.go    # :28  'The Config.Timeout default is 480s' → 120s
- file: internal/cmd/models.go           # :144 'default 480s' → 120s
- file: pkg/stagecoach/stagecoach.go     # :40  '0 → config default (480s)' → 120s
- file: internal/generate/realagent_test.go  # :62 '(480s/3)' → '(120s/3)'; :136 '(480s)' → '(120s)'  (integration_real tag; never runs)
- file: internal/config/roles.go         # :75 godoc '(global; 480s today, 120s after P1.M2.T2)' → '(global; 120s)'  (COMMENT ONLY)
  why: "Each describes the global default value; now stale. roles.go:75 was written forward-looking anticipating THIS task."
  gotcha: "roles.go:12 (the planner built-in VALUE) is NOT a comment about the global — it STAYS 480. Only :75 (a global-default
           comment) changes, and only its prose, not any value."

# MUST EDIT — docs (the contract: docs/configuration.md rides WITH the work)
- file: docs/cli.md                      # :27 table row '... "480s" ...' → '"120s"'
- file: docs/configuration.md            # :86 example '# timeout = "480s"' → '"120s"'; :133 table '| timeout | 480s | ...' → '120s'
  why: "The default-value tables/example must match the code. README.md has NO 480 reference (verified)."

# KEEP (READ-ONLY) — the planner built-in (P1.M2.T1.S1; do NOT change the value)
- file: internal/config/roles.go         # :6,12,74,81,84,85 — the planner 480s built-in + godoc. STAYS 480s.
  why: "The planner keeps 480s by design (FR-R7). Changing it regresses the planner and breaks roles_test.go / load_test.go planner tests."
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  config.go          # EDIT — Defaults() Timeout literal (:200) + 2 comments (:71, :183)
  bootstrap.go       # EDIT — generated-template comment (:161)
  roles.go           # EDIT (comment :75 ONLY) — the planner built-in (:12) STAYS 480s
  config_test.go     # EDIT — :20-21 (TestDefaults)
  file_test.go       # EDIT — :898 (TestOverlayNilSrc) + :113 comment; LEAVE :577/699/1278 (arbitrary values)
  load_test.go       # EDIT — :693-694; LEAVE :327/370/1129 (planner per-role, arbitrary)
internal/cmd/
  root.go            # EDIT — :169 (--timeout help)
  root_test.go       # EDIT — :246-247 (default pin)
  models.go          # EDIT — :144 (comment)
internal/provider/executor.go   # EDIT — :28 (comment)
internal/generate/realagent_test.go  # EDIT — :62, :136 (comments; integration_real tag)
pkg/stagecoach/stagecoach.go    # EDIT — :40 (comment)
docs/
  cli.md             # EDIT — :27 (table)
  configuration.md   # EDIT — :86, :133 (table + example)
# go.mod, Makefile, README.md — READ-ONLY (no 480 refs in README; verified)
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
# MODIFIED (no new files). ~10 files, each a 1-3 line literal/comment edit:
internal/config/config.go          # Defaults Timeout 480→120 + 2 comments
internal/config/bootstrap.go       # template comment 480s→120s
internal/config/roles.go           # godoc comment :75 only (planner built-in unchanged)
internal/config/config_test.go     # default-pin 480→120
internal/config/file_test.go       # default-pin :898 480→120 + comment :113
internal/config/load_test.go       # default-pin :693-694 480→120
internal/cmd/root.go               # --timeout help 480s→120s
internal/cmd/root_test.go          # default-pin :246-247 480→120
internal/cmd/models.go             # comment 480s→120s
internal/provider/executor.go      # comment 480s→120s
internal/generate/realagent_test.go# 2 comments 480s→120s
pkg/stagecoach/stagecoach.go       # comment 480s→120s
docs/cli.md                        # table 480s→120s
docs/configuration.md              # table + example 480s→120s
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the planner built-in is a DIFFERENT 480 — do NOT change it): roles.go:12 is
// `"planner": 480 * time.Second` — the FR-R7 planner built-in that ResolveRoleTimeout returns for the planner role
// regardless of cfg.Timeout. This task lowers the GLOBAL default (config.go Defaults), NOT the planner built-in. If you
// grep-and-replace `480` blindly you will break the planner and fail roles_test.go / load_test.go planner tests. The
// planner built-in + its godoc (roles.go:6,12,74,81,84,85) STAY 480s. Only the global-default COMMENT on roles.go:75 changes.

// CRITICAL (distinguish default-pinning tests from arbitrary-value tests): the token 480 appears in tests in THREE roles:
//   (a) DEFAULT-PIN: asserts config.Defaults().Timeout / a no-timeout Load == 480s  → CHANGE to 120
//       (config_test.go:20, file_test.go:898, load_test.go:693, root_test.go:246)
//   (b) PLANNER PER-ROLE: setRoleTimeout("planner", 480s) / [role.planner] timeout="480s" / STAGECOACH_PLANNER_TIMEOUT=480s
//       → LEAVE (480 is the planner's deliberate value, not the global) (load_test.go:327/370/1129, file_test.go:577)
//   (c) PARSER/MERGE ARBITRARY: parseTimeout("480s")→480s table; overlay merge using 480 vs 300 vs 120 as distinct values
//       → LEAVE (480 is an arbitrary test input proving the parser/merge) (file_test.go:699/1278)
// The PRP's per-site table (Implementation Tasks) labels each. When in doubt: a test is a DEFAULT-PIN iff it calls Defaults()
// (or Load with no timeout source) and asserts the GLOBAL cfg.Timeout.

// CRITICAL (root_test.go:246 was omitted from the contract's line list): the contract cited root.go:133 / load_test.go:589 etc.
// (PRE-T1.S1 line numbers). After T1.S1 landed (roles.go grew ~90 lines), lines drifted. root_test.go:246-247 is a real
// default-pinning test the contract list missed. Anchor EVERY edit by STRING (grep), not by line number. The grep sweep
// `grep -rn '480' internal/ cmd/ pkg/ docs/` is the source of truth.

// GOTCHA (no migration / no config-version bump): lowering a default is not a schema change. Old config files with no `timeout`
// key now resolve to 120s (Layer-1); a file with `timeout = "300s"` is untouched (file beats Layer-1). Do NOT touch migrate.go
// or CurrentConfigVersion.

// GOTCHA (bootstrap.go:161 has no byte-exact test): bootstrap_test.go references neither 480 nor timeout (verified) — the
// generated-template commented line is substring/valid-TOML tested, so flipping 480s→120s is safe.

// GOTCHA (docs ride with the work): docs/configuration.md :86 + :133 and docs/cli.md :27 state the default and MUST move with
// the code (the contract's DOCS clause). README.md has no 480 reference (verified) — do not edit it for this.

// GOTCHA (the comment on roles.go:75 is now stale-forward-looking): T1.S1 wrote "(cfg.Timeout — the global; 480s today, 120s
// after P1.M2.T2)" anticipating this task. Now that this task lands, update the PROSE to "(global; 120s)". Do NOT touch the
// planner built-in value on roles.go:12 — only the :75 global-default comment.
```

## Implementation Blueprint

### Data models and structure

None. No type changes, no new fields, no new functions. A literal value flip (`480` → `120`) in `Defaults().Timeout` plus a
coordinated sweep of its mirrors, pinning tests, default-describing comments, and docs rows.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/config/config.go — THE core default flip (+ 2 comments)
  - (a) Defaults() Timeout:  `Timeout: 480 * time.Second,`  →  `Timeout: 120 * time.Second,`  (anchor: the ONLY `480 * time.Second`
        inside func Defaults; grep -n 'Timeout:.*480 \* time.Second' internal/config/config.go → 1 hit).
  - (b) field doc (:71):  `Timeout time.Duration ... // generation timeout; Defaults: 480s`  →  `Defaults: 120s`.
  - (c) Defaults godoc (:183):  `// Defaults returns the built-in Layer-1 configuration (PRD §16.1): timeout 480s, ...`  →  `timeout 120s,`.
  - PRESERVE: every other Defaults() field; the RoleConfig.Timeout field (:42); CurrentConfigVersion.
  - GOTCHA: do NOT touch roles.go's planner built-in (different file, different purpose).

Task 2: EDIT internal/config/bootstrap.go — generated-template comment (:161)
  - `b.WriteString("# model          = \"\"\n# timeout        = \"480s\"\n# auto_stage_all = true\n# verbose        = false\n")`
    → change `\"480s\"` to `\"120s\"`.
  - SAFE: bootstrap_test.go has no 480/timeout reference (verified).

Task 3: EDIT internal/cmd/root.go — --timeout help text (:169)
  - `...default 480s)`  →  `...default 120s)`  (the StringVar flagTimeout help string).

Task 4: EDIT the 5 DEFAULT-PINNING tests (480 → 120). Anchor each by STRING:
  - config_test.go:20-21 (TestDefaults):
        `if c.Timeout != 480*time.Second {`  →  `if c.Timeout != 120*time.Second {`
        `t.Errorf("Timeout = %v, want 480s", c.Timeout)`  →  `want 120s`
  - file_test.go:898 (TestOverlayNilSrc — dst is Defaults(), so dst.Timeout IS the global):
        `if dst.Timeout != 480*time.Second {`  →  `120*time.Second`
  - file_test.go:113 (comment in TestOverlayPartial — describes Defaults baseline):
        `// Layer-1 baseline (AutoStageAll=true, MaxDiffBytes=300000, Timeout=480s, …)`  →  `Timeout=120s`
  - load_test.go:693-694 (a Load falling back to Defaults):
        `if cfg.Timeout != 480*time.Second {`  →  `120*time.Second`
        `t.Errorf("Timeout=%v want 480s (default)", cfg.Timeout)`  →  `want 120s (default)`
  - root_test.go:246-247 (Config() resolves Defaults):
        `if cfg.Timeout != 480*time.Second {`  →  `120*time.Second`
        `t.Errorf("Timeout=%v, want 480s (default)", cfg.Timeout)`  →  `want 120s (default)`
  - DO NOT TOUCH the arbitrary-value tests in the same files: file_test.go:577/584, :699/706, :1290/1291, :1341; load_test.go:328/334/377/1129.
    (These use 480 as a planner/parser/merge value, not the global default — see Known Gotchas.)

Task 5: EDIT default-describing COMMENTS (accuracy):
  - internal/provider/executor.go:28 — `The Config.Timeout default is 480s (PRD FR25).` → `120s`.
  - internal/cmd/models.go:144 — `timeout = cfg.Timeout // bound (FR25 knob; default 480s)` → `default 120s`.
  - pkg/stagecoach/stagecoach.go:40 — `Timeout time.Duration // ...; 0 → config default (480s)` → `(120s)`.
  - internal/generate/realagent_test.go:62 — `// Timeout/MaxDuplicateRetries inherit config.Defaults() (480s/3).` → `(120s/3)`.
  - internal/generate/realagent_test.go:136 — `// RUN: ... Timeout per attempt = cfg.Timeout (480s).` → `(120s)`.
  - internal/config/roles.go:75 — godoc `> [defaults].timeout     (cfg.Timeout — the global; 480s today, 120s after P1.M2.T2)`
    → `(cfg.Timeout — the global; 120s)`. COMMENT PROSE ONLY.
  - GOTCHA: roles.go:12 (`"planner": 480 * time.Second`) is the planner built-in VALUE — UNCHANGED. Only :75 (a global comment) moves.

Task 6: EDIT docs (the contract: configuration.md rides with the work):
  - docs/cli.md:27 — table cell `| \`--timeout <dur>\` | string | "480s" | ...` → `"120s"`.
  - docs/configuration.md:86 — example `# timeout        = "480s"` → `"120s"`.
  - docs/configuration.md:133 — table `| \`timeout\` | \`480s\` | \`config.Defaults()\` |` → `\`120s\``.
  - README.md: NO change (no 480 reference — verified).

Task 7: VERIFY — build (native+cross), vet, format, focused + full tests, lint, coverage, grep guards
  - go build ./... ; GOOS=windows go build ./... ; GOOS=linux go build ./...
  - go vet ./internal/config/... ./internal/cmd/...
  - gofmt -l internal/config/*.go internal/cmd/*.go internal/provider/*.go pkg/stagecoach/*.go   # must be empty
  - go test ./internal/config/ -run 'Defaults|Timeout|Overlay' -v
  - go test ./internal/cmd/ -run 'Config|Timeout' -v
  - go test ./internal/config/ -run 'ResolveRoleTimeout|ResolveRoleModel' -v   # planner built-in tests stay green
  - make test ; make lint ; make coverage-gate
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the core flip (anchor by STRING — there are TWO `480 * time.Second` in the package; only the Defaults one changes).
// internal/config/config.go, inside func Defaults():
//   BEFORE:  Timeout: 480 * time.Second,
//   AFTER :  Timeout: 120 * time.Second,
// The OTHER `480 * time.Second` (internal/config/roles.go:12, the planner built-in) is UNCHANGED.

// PATTERN: a default-pinning test (calls Defaults() / no-timeout Load, asserts the GLOBAL) — flip the literal + the message:
//   BEFORE:  if cfg.Timeout != 480*time.Second { t.Errorf("... want 480s (default)", cfg.Timeout) }
//   AFTER :  if cfg.Timeout != 120*time.Second { t.Errorf("... want 120s (default)", cfg.Timeout) }

// PATTERN: an arbitrary-value test (uses 480 as a deliberate per-role/parser/merge value) — LEAVE UNCHANGED:
//   file_test.go:699  src.Roles["planner"] = {Timeout: 480*time.Second}  // higher layer sets TIMEOUT only (vs dst 300s)
//   file_test.go:1290 {"duration_string", "480s", false, 480*time.Second, ""}  // parseTimeout table
//   load_test.go:327  cfg.setRoleTimeout("planner", 480*time.Second)  // planner per-role (arbitrary)
// These are NOT default pins — 480 is a valid test input regardless of the global default.

// PATTERN: planner-safety proof (unchanged test that PINS the planner still gets 480s after the global drops to 120s):
//   roles_test.go:213  TestResolveRoleTimeout_PlannerBuiltinBeatsGlobal sets cfg.Timeout=120s, asserts planner → 480s.
//   It must STILL PASS after this task (it already uses 120s for the global). Do not touch it.
```

### Integration Points

```yaml
CONFIG (internal/config):
  - config.go: Defaults().Timeout 480s → 120s (the Layer-1 global); + field/godoc comments.
  - bootstrap.go: generated-template commented timeout 480s → 120s.
  - roles.go: godoc comment :75 only (the planner built-in :12 is UNCHANGED).

CLI (internal/cmd):
  - root.go: --timeout help default 480s → 120s.
  - root_test.go: default-pin → 120s.
  - models.go: comment → 120s.

CONSUMERS (NO change this task — they read cfg.Timeout via the existing path):
  - provider.Execute (executor.go: comment only) bounds each attempt at cfg.Timeout (now 120s default for non-planner).
  - ResolveRoleTimeout (roles.go) returns the planner 480s built-in regardless — planner unaffected.

DOCS:
  - docs/cli.md :27, docs/configuration.md :86 + :133 → 120s. README.md unchanged.

NO database / migration / routes / new types / new dependency / config-version bump. A default-value change is backward-compatible:
old files with no `timeout` key now resolve to 120s; a file setting `timeout` is untouched (file beats Layer-1).

SCOPE FENCES: NO planner built-in change (roles.go:12 stays 480s); NO consumer wiring (P1.M3); NO ResolveRoleTimeout change; NO
  migration; NO config-version bump; NO README edit (no 480 ref); NO arbitrary-value test churn (file_test.go:577/699/1278,
  load_test.go:327/370/1129 stay 480).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Native + cross build (a literal/comment change must build everywhere).
go build ./...
GOOS=windows go build ./...
GOOS=linux   go build ./...
# Expected: clean. A failure means a comment got malformed or an edit strayed into code.

# Vet.
go vet ./internal/config/... ./internal/cmd/...
# Expected: clean.

# Format.
gofmt -l internal/config/*.go internal/cmd/*.go internal/provider/*.go pkg/stagecoach/*.go internal/generate/*.go
# Expected: empty. (Only literal/comment edits → gofmt should be a no-op; if a file is listed, gofmt -w it.)

# Lint.
make lint      # golangci-lint (staticcheck/gosimple/govet/errcheck/ineffassign/unused)
# Expected: zero errors. Comment/literal edits do not affect lint.

# Scope guard: only the expected files changed.
git diff --name-only
# Expected: the ~14 files in the Desired Codebase tree (config.go, bootstrap.go, roles.go, config_test.go, file_test.go,
#           load_test.go, root.go, root_test.go, models.go, executor.go, realagent_test.go, stagecoach.go, cli.md, configuration.md).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The flipped default-pinning tests (the canary — they FAIL if config.go changed but a test didn't, and vice versa).
go test ./internal/config/ -run 'TestDefaults' -v
go test ./internal/config/ -run 'TestOverlayNilSrc' -v
go test ./internal/config/ -run 'TestLoad' -v          # covers the load_test.go:693 default pin
go test ./internal/cmd/   -run 'TestConfig|Timeout' -v # covers root_test.go:246
# Expected: all PASS with the new 120s assertions.

# Regression: the planner built-in is UNCHANGED — ResolveRoleTimeout still returns 480s for the planner.
go test ./internal/config/ -run 'ResolveRoleTimeout' -v
# Expected: green — esp. TestResolveRoleTimeout_PlannerBuiltinBeatsGlobal (cfg.Timeout=120s → planner 480s).

# Regression: the arbitrary-value tests (parseTimeout, role parsing, overlay merge) are UNCHANGED.
go test ./internal/config/ -run 'TestLoadTOMLRoleTimeout|TestOverlay|parseTimeout' -v
# Expected: green (they still use 480 as their arbitrary value).

# Full race suite.
make test
# Expected: green (race detector). This is the master gate: if ANY default-pin was missed, a test fails here.

# Coverage gate (PRD §20.3: ≥85% on internal/{git,provider,generate,config}).
make coverage-gate
# Expected: passes (a default-value flip is coverage-neutral; the 5 updated tests still execute the same code).
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary.
make build

# Prove the resolved default is 120s end-to-end (a --dry-run reaches config.Load → Defaults).
d=$(mktemp -d) && cd "$d" && git init -q
git config user.email t@t.com && git config user.name t
printf 'a\n' > f.txt && git add f.txt && git commit -qm init
printf 'b\n' >> f.txt && git add f.txt

# (a) --help advertises the new default.
SC=/home/dustin/projects/stagecoach/bin/stagecoach
"$SC" --help 2>&1 | grep -i 'timeout' | grep '120s'
# Expected: a line showing "default 120s".

# (b) config init --template shows the commented 120s default.
"$SC" config init --template 2>/dev/null | grep 'timeout' | grep '120s'
# Expected: a commented `# timeout = "120s"` line.

cd - && rm -rf "$d"
# (The actual timeout-bounds-a-run behavior is covered by the executor's existing timeout tests; this task only moves the default.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Scope guard 1: Defaults().Timeout is now 120s.
grep -n 'Timeout:.*120 \* time.Second' internal/config/config.go
# Expected: 1 hit (Defaults). And:
grep -n 'Timeout:.*480 \* time.Second' internal/config/config.go
# Expected: ZERO hits (the global default is no longer 480).

# Scope guard 2: the planner built-in is STILL 480s (UNTOUCHED).
grep -n '"planner": 480 \* time.Second' internal/config/roles.go
# Expected: 1 hit (the planner built-in — do NOT change it).

# Scope guard 3: NO default-pinning test still asserts 480 for the GLOBAL (cfg/c/dst.Timeout).
grep -rn 'Timeout != 480\|want 480s (default)\|want 480s"' internal/ --include='*_test.go' | grep -iv 'planner\|Roles\['
# Expected: ZERO hits (all global default-pins moved to 120). Planner/role assertions (which legitimately use 480) are excluded.

# Scope guard 4: the remaining 480s in tests are ONLY planner/role/parser/merge values (arbitrary), not global default pins.
grep -rn '480' internal/config/*_test.go internal/cmd/*_test.go
# Expected: only in load_test.go:327/334/377/1129 (planner per-role), file_test.go:577/584/699/706/1290/1291/1341 (role/parser/merge),
#           multiturn_test.go (48000 — different number). NONE of these assert the GLOBAL cfg.Timeout default.

# Scope guard 5: help text + generated template + docs say 120s.
grep -n 'default 120s' internal/cmd/root.go
grep -n 'timeout        = "120s"' internal/config/bootstrap.go
grep -n '"120s"' docs/cli.md
grep -n '| `120s` |' docs/configuration.md
# Expected: 1 hit each.

# Scope guard 6: roles.go:75 godoc updated (global now 120s); planner built-in comment still 480s.
grep -n 'global; 120s' internal/config/roles.go          # the updated global comment
grep -n 'planner is the ONLY role with a built-in timeout (480s)' internal/config/roles.go   # planner built-in godoc UNCHANGED
# Expected: 1 hit each.

# Scope guard 7: planner-safety — ResolveRoleTimeout("planner", Defaults()) returns 480s even though global is now 120s.
go test ./internal/config/ -run 'TestResolveRoleTimeout_PlannerBuiltinBeatsGlobal' -v
# Expected: PASS (it sets cfg.Timeout=120s and asserts planner→480s; proves the global flip didn't regress the planner).

# Scope guard 8: no accidental consumer wiring (this task changes no Execute call site).
git diff --name-only | grep -E 'executor.go|planner.go|stager.go|message.go|arbiter.go|generate.go|multiturn.go|workdesc.go'
# Expected: only executor.go (a COMMENT) — NO Execute-arg change. (P1.M3 owns consumer wiring.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + `GOOS=windows/linux go build ./...` clean
- [ ] `go vet ./internal/config/... ./internal/cmd/...` clean
- [ ] `gofmt -l` empty on all edited files
- [ ] `make lint` zero errors
- [ ] `make test` (race) green — the 5 flipped default-pins pass; arbitrary-value tests unchanged
- [ ] `make coverage-gate` ≥85% on the 4 core packages

### Feature Validation
- [ ] `config.Defaults().Timeout == 120 * time.Second`
- [ ] 5 default-pinning tests assert 120s (config_test.go:20, file_test.go:898, load_test.go:693, root_test.go:246 + comment :113)
- [ ] `--timeout` help / generated template / docs (cli.md, configuration.md ×2) all say 120s
- [ ] Planner UNAFFECTED: `ResolveRoleTimeout("planner", Defaults())` still returns 480s; `TestResolveRoleTimeout_PlannerBuiltinBeatsGlobal` green

### Scope-Boundary Validation
- [ ] `git diff --name-only` == only the ~14 files listed (config.go, bootstrap.go, roles.go[:75 comment], 4 test files, root.go,
      models.go, executor.go, realagent_test.go, stagecoach.go, cli.md, configuration.md)
- [ ] Planner built-in `roles.go:12` UNCHANGED (still `480 * time.Second`)
- [ ] Arbitrary-value tests UNCHANGED (file_test.go:577/699/1278, load_test.go:327/370/1129)
- [ ] NO consumer wiring (no Execute-arg change; executor.go is a comment only) — P1.M3
- [ ] NO migration / NO config-version bump / NO README edit

### Code Quality & Docs
- [ ] Comments describing the default (executor.go:28, models.go:144, stagecoach.go:40, realagent_test.go:62/136) say 120s
- [ ] roles.go:75 godoc updated to "global; 120s" (planner built-in godoc still 480s)
- [ ] All edits anchored by STRING (grep), not by line number (lines drifted after T1.S1)

---

## Anti-Patterns to Avoid

- ❌ Don't grep-and-replace `480` blindly. The token `480` appears in THREE roles in this codebase: the global default [CHANGE], the
  planner built-in `roles.go:12` [KEEP], and parseTimeout/arbitrary merge/role test values [KEEP]. A blanket replace breaks the
  planner (regressing FR-R7) and churns unrelated parser tests. Use the per-site table in Implementation Tasks.
- ❌ Don't change `roles.go:12` (`"planner": 480 * time.Second`). That is the FR-R7 planner built-in; lowering it undoes P1.M2.T1.S1
  and fails `TestResolveRoleTimeout_PlannerBuiltinBeatsGlobal`. Only the `roles.go:75` GLOBAL-default COMMENT changes (prose → "120s").
- ❌ Don't anchor edits to the contract's line numbers (config.go:197, root.go:133, load_test.go:589). Those are PRE-T1.S1 numbers;
  roles.go grew ~90 lines and everything drifted. Anchor every edit by its STRING via `grep -rn '480'`. root_test.go:246 is a real
  default-pin the contract list omitted — the grep sweep is the source of truth.
- ❌ Don't change the arbitrary-value tests. `file_test.go:577` (`[role.planner] timeout = "480s"`), `file_test.go:699` (overlay merge
  480-vs-300), `file_test.go:1290` (parseTimeout `"480s"→480s`), `load_test.go:327/370/1129` (planner per-role 480s) all use 480 as a
  deliberate test value, NOT the global default. They stay 480 and stay green. A test is a default-pin IFF it calls `Defaults()` (or a
  no-timeout `Load`) and asserts the GLOBAL `cfg.Timeout`.
- ❌ Don't skip the docs. The contract's DOCS clause makes `docs/configuration.md` (:86 + :133) and `docs/cli.md` (:27) ride with the
  work — they state the default and must match the code. README has no 480 ref (verified) — leave it.
- ❌ Don't add a migration or bump `CurrentConfigVersion`. Lowering a default is backward-compatible: old files with no `timeout` key
  resolve to 120s now; a file with `timeout = "300s"` is untouched (file beats Layer-1). migrate.go is field-specific and irrelevant.
- ❌ Don't wire any consumer (don't change Execute call sites, don't touch `ResolveRoleTimeout`). The global flows into `cfg.Timeout`
  automatically; the planner keeps its built-in. Consumer per-role wiring is P1.M3. executor.go gets a COMMENT edit only.
- ❌ Don't forget `root_test.go:246-247`. It's a default-pinning test the contract's line list missed (drift). If you skip it,
  `make test` fails in `internal/cmd`. The grep sweep (`grep -rn '480' internal/cmd/`) catches it.
- ❌ Don't couple this to a config-version change or a "deprecation notice" for the old 480s. There is none in the PRD; the default
  simply moves. Users who relied on 480s set `[defaults] timeout = "480s"` explicitly (and that still works — file beats Layer-1).

---

## Confidence Score: 9/10

This is a surgical, grep-driven literal+comment flip with a fully-enumerated CHANGE list (1 core line + 4 source mirrors + 5
default-pinning tests + 6 default-describing comments + 3 docs rows) and an equally important KEEP list (the planner built-in, the
parseTimeout examples, the arbitrary-value merge/role tests). The planner-safety is provable (`ResolveRoleTimeout` returns the 480s
built-in regardless of `cfg.Timeout`; `TestResolveRoleTimeout_PlannerBuiltinBeatsGlobal` pins it). Every edit is anchored by string
(not drifted line numbers), the validation gates are verified Makefile commands, and `make test` is the master canary (any missed
default-pin fails loudly). The -1 from 10/10 reflects that the grep sweep requires careful human judgment to classify each `480` hit
(CHANGE vs KEEP) — the PRP's per-site table removes that ambiguity, but it remains the one place an inattentive implementer could
over-reach (break the planner) or under-reach (miss root_test.go). The grep guards in Level 4 catch both failure modes.
