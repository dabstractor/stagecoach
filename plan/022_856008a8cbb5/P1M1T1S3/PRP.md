name: "P1.M1.T1.S3 — CLI flags (--idle-timeout + 4 per-role), bootstrap template, and Mode-A docs (cli.md + configuration.md)"
description: >
  Complete the idle-deadline precedence ladder's TOP layer (CLI flags) plus every user-facing text surface:
  internal/cmd/root.go gains --idle-timeout + --{planner,stager,message,arbiter}-idle-timeout as STRING
  flags (zero defaults, mirroring the --timeout registrations exactly) with the --timeout help retitled
  to the 600s hard-cap wording; internal/config/load.go loadFlags gains the global + per-role idle arms
  (parseTimeout, silent-ignore-on-malformed — the documented flag-layer asymmetry; per-role via S2's
  setRoleIdleTimeout); internal/config/bootstrap.go's template gains # idle_timeout = "60s" + # timeout =
  "600s" with one-line comments and the existing :359/:375 prose sweeps 120s→600s; internal/cmd/config.go's
  inert --template literal (a third copy of the same user-facing strings) gets the same treatment (flagged
  adjacent surface); root_test.go:324's stale 120s assertion (red since S1) is fixed. Mode-A docs per
  docs_drift_inventory.md's exact line list: docs/cli.md (--timeout row retitle, new --idle-timeout + 4
  per-role idle rows, map-table rows, exit-124 rewording) + docs/configuration.md (template comment,
  planner-role comment, defaults table, env table + per-role rows, git example/table/NOTE, decompose prose).
  S2 must have landed (S3's loadFlags calls setRoleIdleTimeout).

---

## Goal

**Feature Goal**: Make the idle deadline settable from the CLI (global `--idle-timeout` + four
`--<role>-idle-timeout` flags — the top of the five-layer precedence ladder) and make every user-facing
text surface (flag help, bootstrap template, inert `--template`, docs/cli.md, docs/configuration.md)
tell the v3.5 truth: idle 60s default (kill on silence, the primary signal), hard cap 600s default
(the backstop), planner 480s retired.

**Deliverable**: (1) root.go — 5 new StringVars + storage vars + the `--timeout` help retitle + the
per-role timeout help reword; (2) load.go loadFlags — global `"idle-timeout"` arm + per-role
`role+"-idle-timeout"` arm; (3) bootstrap.go — the template-body line (`# timeout = "600s"` +
`# idle_timeout = "60s"` with one-line comments) + the :359/:375 prose sweep; (4) cmd/config.go — the
inert-template trio (:649/:675/:694) reworded + idle variants added; (5) root_test.go:324 → 600s;
(6) load_test.go — newFlagSet extension + the flag test twins (90s sentinels); (7) a bootstrap template
content test asserting both lines; (8) the docs/cli.md + docs/configuration.md edits per the drift
inventory's exact anchor list.

**Success Definition**:
- `--idle-timeout 90s` / `--idle-timeout 90` / `--planner-idle-timeout 90s` parse via parseTimeout into
  `cfg.IdleTimeout` / `cfg.Roles["planner"].IdleTimeout`; a malformed value is SILENTLY IGNORED (the
  flag-layer asymmetry — env is the hard-error layer, S2's); an unpassed flag never overrides (Changed-only).
- `stagecoach --help` shows `--idle-timeout` + the four per-role idle flags, and `--timeout`'s help says
  hard-cap/default 600s (no stale "default 120s").
- `config init`'s generated template contains `# idle_timeout   = "60s"` AND `# timeout        = "600s"`
  (with the kill-on-silence / hard-cap one-line comments); `config init --template`'s inert file likewise.
- docs/cli.md and docs/configuration.md match the drift inventory's required changes line-for-line;
  `grep -rn '480s' docs/cli.md docs/configuration.md` is empty; no stale `default 120s` anywhere user-facing.
- `go test ./internal/cmd/` is GREEN (it is currently RED — root_test.go:324 pins the pre-S1 120s default).
- `go build ./...`, `go vet ./...`, `gofmt -l`, `go test ./internal/config/...`, `make test`, `make lint` pass.

## User Persona (if applicable)

**Target User**: A stagecoach user whose agent occasionally hangs silently mid-generation. They want a
short idle kill (`--idle-timeout 90s` for a chatty agent, or per-role `--planner-idle-timeout 120s` for a
thinking planner) while a productive-but-slow run proceeds to the 600s hard cap.

**Use Case**: `stagecoach --idle-timeout 90s` — or persistently via any of S2's layers
(`STAGECOACH_IDLE_TIMEOUT`, `git config stagecoach.idleTimeout 90`, `[defaults].idle_timeout = "90s"`).
The flag wins over all of them.

**Pain Points Addressed**: Before S3, the idle deadline is settable only via env/git/file (S2) — the
flag layer (the most discoverable, per-invocation surface) is missing, and every help/template/doc
surface still describes the retired v3.4 semantics (120s elapsed-only kill, planner 480s built-in).

## Why

- **FR25 / FR-R7 / §9.30 (P0, → G30)**: each role resolves TWO deadlines (idle + hard cap) on the same
  ladder as provider/model/reasoning. S1 built the fields + resolver; S2 built the env/git/file layers;
  S3 completes the ladder's top layer (flags) — without it the idle deadline is not settable from the CLI
  and the five-layer surface is incomplete.
- **Docs truthfulness (Mode A)**: S1 already changed the shipped default to idle 60s / hard cap 600s and
  retired the planner 480s. Every surface S3 owns (flag help, both bootstrap templates, cli.md,
  configuration.md) currently tells the OLD story. The drift inventory enumerates the exact anchors —
  "Execute the exact line list in docs_drift_inventory.md."
- **Flag-layer consistency**: the timeout flags' contract (pflag STRING, zero default, parsed ONLY in
  loadFlags via Changed→GetString→parseTimeout, silent-ignore on malformed) is a deliberate design
  (§15.2; config_ladder FINDING 7). The idle flags mirror it exactly, so the two ladders stay
  structurally identical.

## What

**User-visible behavior**: `stagecoach --help` lists `--idle-timeout` and the four per-role idle flags;
`--timeout`'s help describes the 600s hard cap. `config init` writes a template documenting both
deadlines. cli.md/configuration.md document the full flag↔env↔git map, defaults table, and exit-124
semantics for both deadlines.

**Technical change (root.go + load.go + bootstrap.go + cmd/config.go + root_test.go + load_test.go +
bootstrap test + docs/cli.md + docs/configuration.md):** five new flags + their loadFlags arms + the
120s→600s/480s-retirement prose sweep across all owned surfaces + the docs anchor list.

### Success Criteria
- [ ] root.go: `--idle-timeout` + 4 `--<role>-idle-timeout` StringVars (zero defaults, STRING type)
- [ ] root.go: `--timeout` help retitled (hard cap, default 600s); per-role timeout help reworded (hard-cap; message/arbiter examples stop implying 120s)
- [ ] loadFlags: global + per-role idle arms (parseTimeout; silent-ignore malformed; Changed-only)
- [ ] bootstrap.go: template line = `# timeout = "600s"` + `# idle_timeout = "60s"` (one-line comments); :359/:375 prose 120s→600s
- [ ] cmd/config.go inert template (:649/:675/:694) reworded + idle variants (flagged adjacent surface)
- [ ] root_test.go:324 → 600s; `go test ./internal/cmd/` green
- [ ] newFlagSet registers both idle flags; flag test twins (90s) incl. malformed-ignored + flag-beats-env
- [ ] Template content test asserts BOTH lines in `GenerateBootstrapConfig` output
- [ ] docs/cli.md: :27 retitle + idle row; :60-63 reword + 4 idle rows; map :468/:499 additions; :452/:454 exit rewording
- [ ] docs/configuration.md: :91, :99, :138, :193, :212-215, :232, :240-241, :254, :264 per the inventory
- [ ] `go build ./...`, `make test`, `make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — every flag registration is quoted verbatim (live line numbers), the loadFlags arms are quoted,
the exact template WriteString and its column-alignment arithmetic are given, the S2 boundary (which
lines S2 already added vs which S3 rewords) is resolved explicitly, the three cmd/config.go string sites
are located, the stale-test fix is identified, the test harness helpers and mirror-tests are named with
lines, and the docs work is enumerated anchor-by-anchor with the required new text.

### Documentation & References

```yaml
- file: internal/cmd/root.go
  why: "THE flag-registration file. Config-backed vars block :52-59 (flagTimeout :56 — 'STRING —
        config.Load reads via fs.GetString' FINDING 7); per-role vars :61-84 (timeout vars :81-84);
        pf := rootCmd.PersistentFlags() :164; --timeout StringVar :169 (help embeds 'default 120s' —
        RETITLE); the four --<role>-timeout StringVars :258-266 (message/arbiter examples say 120s —
        reword per drift-inventory root.go:264/:266)."
  pattern: "New flags mirror the timeout registrations EXACTLY: pf.StringVar(&var, \"name\", \"\",
            \"help (env STAGECOACH_X; git stagecoach.y)\") — STRING type, zero default, root.go never parses."
  gotcha: "Anchor on constructs (grep 'planner-timeout') — S1/S2 drift lines. The &flagVar address IS the
           var's use (satisfies the `unused` linter) — add the 5 new vars to the existing var blocks."

- file: internal/config/load.go
  why: "THE loadFlags parsing. Global timeout arm :433-439 (Changed→GetString→parseTimeout→cfg.Timeout=d,
        silent-ignore malformed); per-role arm :466-479 inside `for _, role := range roleNames` (→
        cfg.setRoleTimeout). S3 adds the idle twin of each arm — the per-role one calls S2's
        setRoleIdleTimeout. parseTimeout :673-684 accepts \"90s\" AND bare 90."
  pattern: "Silent-ignore-on-malformed is loadFlags-ONLY (no error return); the env layer (S2) hard-errors.
            Keep the asymmetry — mirror the existing arms' comments."
  gotcha: "S3 DEPENDS ON S2: prerequisite `grep -c setRoleIdleTimeout internal/config/load.go` ≥ 1 (S2 adds
           the helper + env/git/file reads). Do NOT re-add S2's seams."

- file: internal/config/bootstrap.go
  why: "THE generated-config template. buildBootstrapConfig :156-438; the template-body WriteString :174
        (`# model … / # timeout = \"120s\" / # auto_stage_all / # verbose`); env doc block :352-370
        (STAGECOACH_TIMEOUT :359); git doc block :372-377 (stagecoach.timeout 120s :375; :376 shows the
        camelCase autoStageAll convention); CLI-flag doc block :380-386. Public entry
        GenerateBootstrapConfig(prov) :22."
  pattern: "Name column padded to 15 after '# ' (idle_timeout = 12 chars + 3 spaces); the reasoning line
            :173 is the trailing-comment precedent. idle_timeout goes DIRECTLY BELOW timeout (drift
            inventory: 'directly below, mirroring the timeout line's style')."
  gotcha: "S2's contract ALREADY ADDS the two idle doc lines (STAGECOACH_IDLE_TIMEOUT after :359;
           stagecoach.idleTimeout after :375) — S3 VERIFIES them (grep), does NOT re-add. S3 owns: the
           template body :174 AND rewording the EXISTING :359/:375 timeout lines (120s→600s, hard-cap)."

- file: internal/cmd/config.go
  why: "ADJACENT SURFACE (flagged): the inert `config init --template` literal duplicates the header —
        env line :649 (STAGECOACH_TIMEOUT … \"120s\"), git line :675 (stagecoach.timeout 120s), [defaults]
        line :694 (# timeout = \"120s\" # generation timeout…). Drift-inventory MUST-CHANGE (:649/:675/:694);
        same user-facing nature as bootstrap.go; in S3's package. Include for coherence (P3.M1.T1.S2's
        end-of-plan grep would otherwise catch stale user-facing text)."
  pattern: "Same edits as bootstrap.go's blocks: reword the timeout lines to 600s hard-cap; add the idle
            variants (STAGECOACH_IDLE_TIMEOUT line, git config stagecoach.idleTimeout 60s line,
            # idle_timeout = \"60s\" template line)."
  gotcha: "NOT in the item's literal line list — keep the edit minimal (those three areas only)."

- file: internal/cmd/root_test.go
  why: "root_test.go:324 (TestRoot_LoadsConfigAndRunsStub :298) asserts cfg.Timeout == 120s — STALE since
        S1's 600s default; the cmd package is currently RED because of it. S3 owns root.go's tests → fix
        to 600s and sweep any other cmd default assertion S1 broke."
  pattern: "Plain assertion style: if cfg.Timeout != 600*time.Second { t.Errorf(\"Timeout=%v, want 600s
            (default)\", cfg.Timeout) }."

- file: internal/config/load_test.go
  why: "THE flag-test harness. newFlagSet :49-79 (timeout at :56; roleNames loop :64-68 registers
        role+\"-timeout\") — extend with \"idle-timeout\" + role+\"-idle-timeout\". Mirror tests:
        TestLoadFlags_TimeoutString :506, TestLoadFlags_PerRoleTimeout :539 (duration + bare-int +
        field-merge), _MalformedIgnored :567 (pins silent-ignore), TestLoad_PerRoleTimeout_FlagBeatsEnv
        :1145 (full Load; loadEnvSetup + chdir idiom)."
  pattern: "fs.Set(\"idle-timeout\", \"90s\") marks Changed; distinct 90s sentinels (S2 uses 45s env/git/file;
            timeout tests use 60s/120s/600s) so no assertion passes through the wrong field."
  gotcha: "Flag-beats-env needs flag 90s > env 45s — set STAGECOACH_IDLE_TIMEOUT=45s via t.Setenv, then
           fs.Set(\"idle-timeout\", \"90s\"), assert 90s."

- docfile: plan/022_856008a8cbb5/architecture/docs_drift_inventory.md
  why: "THE docs authority — the item says 'Execute the exact line list in docs_drift_inventory.md.' §1
        quotes every cli.md anchor verbatim (L27, L60-63, L452, L454, map L468/L496-499) with the required
        v3.5 change; §2 quotes every configuration.md anchor (L91, L99, L138, L193, L212-215, L232,
        L240-241, L254, L264)."
- docfile: plan/022_856008a8cbb5/architecture/config_ladder.md
  why: "§6 root.go (verbatim flag registrations) + §5 bootstrap.go (verbatim template line + doc blocks) +
        §7 test conventions + §8 the complete idle-seam checklist (items 6-8 are S3's)."
- docfile: plan/022_856008a8cbb5/P1M1T1S2/PRP.md
  why: "S2 is the CONTRACT: it adds setRoleIdleTimeout (S3's loadFlags per-role arm calls it) AND
        bootstrap.go's two idle doc lines (S3 verifies, doesn't re-add). Its scope fence: 'S3 owns the
        120s→600s prose sweep with the template + flag help.'"
- docfile: plan/022_856008a8cbb5/P1M1T1S3/research/findings.md
  why: "The live-tree verification: S1-landed/S2-not-landed state, the S2/S3 bootstrap boundary
        resolution, the verbatim registration/parsing arms, the cmd/config.go third copy, the red cmd
        package, the docs anchors re-verified."
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/
  root.go            # THE flag file: +flagIdleTimeout +4 per-role idle vars; +--idle-timeout +4 StringVars; --timeout help retitle; per-role timeout help reword
  root_test.go       # FIX :324 (120s→600s; currently RED from S1's residue)
  config.go          # ADJACENT: inert --template literal :649/:675/:694 reword + idle variants
internal/config/
  load.go            # loadFlags: +global "idle-timeout" arm, +per-role role+"-idle-timeout" arm (S2's setRoleIdleTimeout)
  load_test.go       # newFlagSet: +2 idle flag registrations; +flag test twins (90s sentinels)
  bootstrap.go       # template :174 (600s + idle_timeout 60s + comments); :359/:375 prose 120s→600s
  bootstrap_test.go  # +template content test (asserts both lines via GenerateBootstrapConfig)
docs/
  cli.md             # Mode A: :27, :60-63, :468, :496-499, :452, :454
  configuration.md   # Mode A: :91, :99, :138, :193, :212-215, :232, :240-241, :254, :264
```

### Desired Codebase tree with files to be added

```bash
(No new files — all edits to existing files.)
```

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (S3 depends on S2): loadFlags' per-role idle arm calls cfg.setRoleIdleTimeout — S2's helper.
//   PREREQUISITE: grep -c setRoleIdleTimeout internal/config/load.go ≥ 1. S2 also already added
//   bootstrap.go's two idle doc lines (env block + git block) — VERIFY via grep, do NOT re-add.

// CRITICAL (flag contract): the deadline flags are pflag STRINGs at ZERO default, parsed ONLY in
//   loadFlags via fs.Changed → fs.GetString → parseTimeout. root.go NEVER parses. An unpassed flag's
//   zero default never overrides (Changed-only). Do NOT use pflag's DurationVar — the string form
//   accepts both "90s" and bare 90 and keeps Changed semantics.

// CRITICAL (silent-ignore asymmetry): loadFlags has NO error return — a malformed --idle-timeout value
//   is silently ignored (same as --timeout today; the env layer is the hard-error layer). Mirror the
//   existing arms' comments; do NOT invent an error path.

// CRITICAL (camelCase git key in help text): the git home for the idle deadline is
//   stagecoach.idleTimeout (git rejects underscores in the final key segment — S2's FINDING A). Flag
//   help + docs must say idleTimeout, NEVER stagecoach.idle_timeout. (The TOML/env spellings ARE
//   snake/upper: idle_timeout / STAGECOACH_IDLE_TIMEOUT.)

// GOTCHA (bootstrap template alignment): the :174 WriteString pads the name column to 15 after "# "
//   (model+10sp, timeout+8sp, auto_stage_all+1sp). idle_timeout (12 chars) takes 3 spaces:
//   "# idle_timeout   = \"60s\"". idle_timeout goes DIRECTLY BELOW timeout (drift inventory).

// GOTCHA (root_test.go is RED at S3 start): S1 changed Defaults().Timeout to 600s but :324 still
//   asserts 120s — the cmd package fails today. Fix :324 to 600s; run go test ./internal/cmd/ and fix
//   any other cmd default assertion S1's change broke (they're in S3's territory).

// GOTCHA (docs example collision): configuration.md :193's env example "STAGECOACH_TIMEOUT=60s"
//   collides with the new idle default — change the example to 600s per the inventory.

// GOTCHA (test sentinels): use 90s for every S3 flag-test value (S2's env/git/file twins use 45s;
//   timeout tests use 60s/120s/600s) so no idle assertion can pass through the wrong field.

// SCOPE: S3 is root.go + load.go(loadFlags only) + bootstrap.go(template body + existing-line reword)
//   + cmd/config.go(three string sites, flagged adjacent) + root_test/load_test/bootstrap_test + the
//   two doc files. Do NOT touch S2's env/git/file layers, config.go/roles.go (S1), the executor or the
//   13 call sites (P1.M2), stubagent (P1.M3), README/how-it-works (P3.M1.T1.S1), providers.md (P2),
//   or CurrentConfigVersion (additive keys never bump — FR-B4).
```

## Implementation Blueprint

### Data models and structure

No types. Five string storage vars + five StringVar registrations + two loadFlags arms + string edits.

### Implementation Tasks (ordered by dependencies)

> **Prerequisite — S1 AND S2 have landed.** S1 (verified landed): `grep -n 'IdleTimeout'
> internal/config/config.go` → Config.IdleTimeout + RoleConfig.IdleTimeout; Defaults seeds 600s/60s.
> S2 (contract): `grep -c setRoleIdleTimeout internal/config/load.go` ≥ 1 AND
> `grep -c 'STAGECOACH_IDLE_TIMEOUT\|stagecoach.idleTimeout' internal/config/bootstrap.go` ≥ 2.
> If S2 hasn't landed, STOP — S3's loadFlags arm won't compile. Anchor on constructs (grep
> `planner-timeout`), not line numbers.

```yaml
Task 1: MODIFY internal/cmd/root.go — the 5 new flags + the help-text sweep
  - VARS: add `flagIdleTimeout string` to the config-backed var block (after flagTimeout, :56 — same
    "STRING" comment discipline); add `flagPlannerIdleTimeout/flagStagerIdleTimeout/
    flagMessageIdleTimeout/flagArbiterIdleTimeout string` after the timeout vars (:81-84).
  - GLOBAL FLAG (right after the --timeout StringVar :169):
        pf.StringVar(&flagIdleTimeout, "idle-timeout", "",
            "Idle deadline — kill the agent after N seconds with no output, e.g. \"60s\" or 60 (env STAGECOACH_IDLE_TIMEOUT, git stagecoach.idleTimeout; default 60s)")
  - RETITLE --timeout (:169) — help currently says "...; default 120s)":
        pf.StringVar(&flagTimeout, "timeout", "",
            "Hard-cap generation timeout (the backstop for the idle deadline), e.g. \"600s\" or 600 (env STAGECOACH_TIMEOUT, git stagecoach.timeout; default 600s)")
  - PER-ROLE FLAGS (right after :258-266, mirroring their shape):
        pf.StringVar(&flagPlannerIdleTimeout, "planner-idle-timeout", "",
            "Per-role idle deadline for the decomposition planner, e.g. \"90s\" or 90 (env STAGECOACH_PLANNER_IDLE_TIMEOUT; git stagecoach.role.planner.idleTimeout)")
        pf.StringVar(&flagStagerIdleTimeout, "stager-idle-timeout", "",
            "Per-role idle deadline for the (tooled) staging agent, e.g. \"90s\" or 90 (env STAGECOACH_STAGER_IDLE_TIMEOUT; git stagecoach.role.stager.idleTimeout)")
        pf.StringVar(&flagMessageIdleTimeout, "message-idle-timeout", "",
            "Per-role idle deadline for the message composer, e.g. \"90s\" or 90 (env STAGECOACH_MESSAGE_IDLE_TIMEOUT; git stagecoach.role.message.idleTimeout)")
        pf.StringVar(&flagArbiterIdleTimeout, "arbiter-idle-timeout", "",
            "Per-role idle deadline for the leftover arbiter, e.g. \"90s\" or 90 (env STAGECOACH_ARBITER_IDLE_TIMEOUT; git stagecoach.role.arbiter.idleTimeout)")
  - REWORD the four EXISTING --<role>-timeout helps (:258-266): "Per-role generation timeout" →
    "Per-role hard-cap generation timeout"; message/arbiter examples `"120s" or 120` → `"90s" or 90`
    (stop implying the retired default — drift-inventory root.go:264/:266).
  - DEPENDENCIES: none (compiles standalone; the vars' addresses are their use).

Task 2: MODIFY internal/config/load.go — the two loadFlags idle arms
  - GLOBAL (right after the timeout arm :433-439, mirroring it):
        if fs.Changed("idle-timeout") {
            if v, err := fs.GetString("idle-timeout"); err == nil {
                if d, perr := parseTimeout(v); perr == nil {
                    cfg.IdleTimeout = d
                }
            }
        }
  - PER-ROLE (INSIDE the same roleNames loop, right after the role+"-timeout" arm :466-479):
        // §9.30 FR-R7 — per-role idle deadline via CLI flag. Mirrors the -timeout arm above EXACTLY:
        // Changed→GetString→parseTimeout→set (setRoleIdleTimeout, S2). loadFlags has NO error return,
        // so a malformed value is silently ignored (same as --timeout / --idle-timeout); the env layer
        // is the hard-error layer. parseTimeout accepts "90s" and bare "90".
        if fs.Changed(role + "-idle-timeout") {
            if v, err := fs.GetString(role + "-idle-timeout"); err == nil {
                if d, perr := parseTimeout(v); perr == nil {
                    cfg.setRoleIdleTimeout(role, d)
                }
            }
        }
  - DEPENDENCIES: Task 1 (flag names) + S2's setRoleIdleTimeout.

Task 3: MODIFY internal/config/bootstrap.go — the template body + the existing-line reword
  - TEMPLATE BODY (:174) — replace the single WriteString:
        b.WriteString("# model          = \"\"\n# timeout        = \"600s\"  # hard cap — the absolute backstop (the idle deadline is the primary signal)\n# idle_timeout   = \"60s\"   # idle deadline — kill the agent after N seconds of silence\n# auto_stage_all = true\n# verbose        = false\n")
    (Column arithmetic: name padded to 15 — idle_timeout + 3 spaces. idle DIRECTLY BELOW timeout.)
  - ENV DOC BLOCK (:359) — reword the EXISTING line (S2's idle line sits right after it; do not touch it):
        #   STAGECOACH_TIMEOUT    hard-cap generation timeout, e.g. "600s" or 600 (seconds)
  - GIT DOC BLOCK (:375) — reword the EXISTING line (S2's idleTimeout line sits after it):
        #   git config stagecoach.timeout 600s
  - VERIFY S2's two idle lines exist (grep STAGECOACH_IDLE_TIMEOUT + stagecoach.idleTimeout in this
    file); do NOT re-add.
  - OPTIONAL (not required; closes a pre-existing gap): the CLI-flag doc block :380-386 lacks any
    --timeout row — may add two lines (`--timeout / --idle-timeout  generation deadlines (hard cap /
    idle)`). Skip if uncertain; the item does not require it.
  - DEPENDENCIES: S2 (the idle doc lines).

Task 4: MODIFY internal/cmd/config.go — the inert --template literal (ADJACENT, flagged)
  - :649 env line → `#   STAGECOACH_TIMEOUT    hard-cap generation timeout, e.g. "600s" or 600 (seconds)`
    + add after it: `#   STAGECOACH_IDLE_TIMEOUT   idle deadline (kill on silence), e.g. "60s" or 60 (seconds)`
  - :675 git line → `#   git config stagecoach.timeout 600s` + add after it: `#   git config stagecoach.idleTimeout 60s`
  - :694 [defaults] line → `# timeout        = "600s"   # hard-cap generation timeout (Go duration string, e.g. "10m", or bare seconds)`
    + add after it: `# idle_timeout   = "60s"   # idle deadline — kill after N seconds of silence`
  - JUSTIFICATION (record in the commit/PR note): drift-inventory MUST-CHANGE (:649/:675/:694); same
    user-facing nature as bootstrap.go; would otherwise ship stale 120s text until P3's end-of-plan sweep.
  - DEPENDENCIES: none.

Task 5: FIX internal/cmd/root_test.go:324 (+ any sibling stale default assertions)
  - `if cfg.Timeout != 120*time.Second {` → `if cfg.Timeout != 600*time.Second {` and the message
    "want 120s (default)" → "want 600s (default)". Run `go test ./internal/cmd/`; fix any OTHER
    default-value assertion S1's 600s change broke (they live in S3's package).
  - DEPENDENCIES: none.

Task 6: MODIFY internal/config/load_test.go — newFlagSet + the flag test twins (90s sentinels)
  - newFlagSet (:49-79): add `fs.String("idle-timeout", "", "")` after the timeout line (:56); inside
    the roleNames loop (:64-68) add `fs.String(role+"-idle-timeout", "", "")`. Update the helper's doc
    comment to mention the idle flags.
  - TestLoadFlags_IdleTimeoutString (mirror :506): fs.Set("idle-timeout", "90s") → cfg.IdleTimeout ==
    90s; a second case fs.Set("idle-timeout", "90") (bare int) → 90s (proves parseTimeout); a malformed
    "abc" case → cfg.IdleTimeout UNCHANGED (silent-ignore — mirror :567's discipline).
  - TestLoadFlags_PerRoleIdleTimeout (mirror :539): fs.Set("planner-idle-timeout", "90s") →
    cfg.Roles["planner"].IdleTimeout == 90s; bare-int "90" row; field-merge row (set
    "planner-provider" too — sibling fields untouched); malformed row silently ignored.
  - TestLoad_IdleTimeout_FlagBeatsEnv (mirror :1145): loadEnvSetup + chdir; t.Setenv
    STAGECOACH_IDLE_TIMEOUT=45s; fs.Set("idle-timeout", "90s"); Load(ctx, LoadOpts{RepoDir: repo,
    Flags: fs}) → cfg.IdleTimeout == 90s (flag > env).
  - DEPENDENCIES: Tasks 1-2 + S2.

Task 7: ADD the bootstrap template content test (internal/config/bootstrap_test.go)
  - TestGenerateBootstrapConfig_DeadlineDefaults: out := GenerateBootstrapConfig("claude"); assert
    strings.Contains(out, `# idle_timeout   = "60s"`) AND strings.Contains(out, `# timeout        = "600s"`)
    AND !strings.Contains(out, `"120s"`) (the stale default is gone from the populated template).
    Optionally also assert the S2 doc lines are present (STAGECOACH_IDLE_TIMEOUT, idleTimeout).
  - DEPENDENCIES: Task 3.

Task 8: EDIT docs/cli.md (Mode A — execute the inventory §1 list)
  - :27 --timeout row → `| `--timeout <dur>` | string | "600s" | `STAGECOACH_TIMEOUT` | `stagecoach.timeout` | Hard-cap generation timeout — the backstop for the idle deadline (e.g. `"600s"` or `600`) |`
  - ADD after :27: `| `--idle-timeout <dur>` | string | "60s" | `STAGECOACH_IDLE_TIMEOUT` | `stagecoach.idleTimeout` | Idle deadline — kills the agent after N seconds with no output (e.g. `"60s"` or `60`) |`
  - :60-63 the four --<role>-timeout rows → "Per-role hard-cap generation timeout for …"; message/arbiter
    examples `"120s"/"120"` → `"90s"/"90"`. Optionally fix their Git-config column `—` →
    `stagecoach.role.<role>.timeout` (pre-existing inaccuracy; the map table is authoritative).
  - ADD after :63 (before --version): four rows
    `| `--<role>-idle-timeout <dur>` | string | "" | `STAGECOACH_<ROLE>_IDLE_TIMEOUT` | `stagecoach.role.<role>.idleTimeout` | Per-role idle deadline for the … (e.g. `"90s"` or `90`) |`
  - Map table: ADD after :468 `| `--idle-timeout` | `STAGECOACH_IDLE_TIMEOUT` | `stagecoach.idleTimeout` |`;
    ADD after :499 (before the watchdog row) four `| `--<role>-idle-timeout` | `STAGECOACH_<ROLE>_IDLE_TIMEOUT` | `stagecoach.role.<role>.idleTimeout` |` rows.
  - :452 exit row → `| `124` | Deadline exceeded (idle or hard cap) — generation exceeded the idle deadline (`--idle-timeout`, default 60s of silence) or the hard cap (`--timeout`, default 600s). |`
  - :454 prose → "A deadline kill (idle or hard cap) is reported as `124` (matching GNU `timeout`), not `3`." (rest of the sentence unchanged).
  - DEPENDENCIES: none.

Task 9: EDIT docs/configuration.md (Mode A — execute the inventory §2 list)
  - :91 → `# timeout        = "600s"   # global hard-cap fallback for every role; every role also resolves a 60s idle deadline` and ADD below:
    `# idle_timeout   = "60s"   # idle deadline — kill after N seconds of silence (the primary productivity signal)` (drop the "planner role defaults to 480s" clause).
  - :99 → `# timeout = "600s"   # per-role hard-cap timeout; no planner special-case — roles resolve idle+hard-cap like model/provider`; optionally ADD `# idle_timeout = "90s"` (commented per-role example).
  - :138 → `| `timeout` | `600s` (hard cap) | `config.Defaults` |` + INSERT `| `idle_timeout` | `60s` | `config.Defaults` |` right after.
  - :193 → `| `STAGECOACH_TIMEOUT` | `--timeout` | Global hard-cap generation timeout — the fallback for every role; every role also resolves a 60s idle deadline (`STAGECOACH_IDLE_TIMEOUT`) | `STAGECOACH_TIMEOUT=600s stagecoach` |`
    + ADD `| `STAGECOACH_IDLE_TIMEOUT` | `--idle-timeout` | Global idle deadline — kill on N seconds of silence; the fallback for every role | `STAGECOACH_IDLE_TIMEOUT=45s stagecoach` |`
  - :212-215 → drop "(; planner built-in 480s)" and "(; inherits 120s)" → "(; inherits the 600s global hard cap)"; keep :214's "single-commit path's only role" note. ADD four
    `| `STAGECOACH_<ROLE>_IDLE_TIMEOUT` | `--<role>-idle-timeout` | Per-role idle deadline for the … (fresh idle clock per invocation) | `STAGECOACH_<ROLE>_IDLE_TIMEOUT=45s stagecoach` |` rows below.
  - :232 ini example → `timeout = 600s` + ADD `idleTimeout = 60s` beneath (CAMELCASE).
  - :240 → "Hard-cap generation timeout (duration string; default 600s)" + ADD `| `stagecoach.idleTimeout` | string | `git config --get stagecoach.idleTimeout` | Idle deadline (kill on silence; default 60s) |`;
    :241 note no planner 480s exception + ADD `| `stagecoach.role.<role>.idleTimeout` | string | `git config --get stagecoach.role.<role>.idleTimeout` | Per-role idle deadline; unset ⇒ inherit global `stagecoach.idleTimeout`. |`.
  - :254 NOTE first sentence → "…has per-role **timeout** and **idle-timeout** keys (`stagecoach.role.<role>.timeout` / `stagecoach.role.<role>.idleTimeout`)…".
  - :264 decompose prose → every role resolves an idle deadline (default 60s) and a hard cap (default 600s) via the same flag > env > `[role.<role>]` > `[defaults]` chain; no planner special-case; drop the built-in precedence tier.
  - DEPENDENCIES: none.

Task 10: VERIFY — gates (below) + `go run . --help | grep -c 'idle-timeout'` (expect 5: global + 4 roles)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the flag registration (STRING, zero default; help names env + git homes + the default)
pf.StringVar(&flagIdleTimeout, "idle-timeout", "",
	"Idle deadline — kill the agent after N seconds with no output, e.g. \"60s\" or 60 (env STAGECOACH_IDLE_TIMEOUT, git stagecoach.idleTimeout; default 60s)")

// PATTERN: the loadFlags arm (Changed-only; silent-ignore malformed — NO error path exists here)
if fs.Changed("idle-timeout") {
	if v, err := fs.GetString("idle-timeout"); err == nil {
		if d, perr := parseTimeout(v); perr == nil {
			cfg.IdleTimeout = d
		}
	}
}

// PATTERN: the per-role arm (extends the EXISTING roleNames loop — no parallel loop)
if fs.Changed(role + "-idle-timeout") {
	if v, err := fs.GetString(role + "-idle-timeout"); err == nil {
		if d, perr := parseTimeout(v); perr == nil {
			cfg.setRoleIdleTimeout(role, d)
		}
	}
}

// PATTERN: the bootstrap template line (name column = 15; idle DIRECTLY below timeout)
b.WriteString("# model          = \"\"\n# timeout        = \"600s\"  # hard cap — the absolute backstop (the idle deadline is the primary signal)\n# idle_timeout   = \"60s\"   # idle deadline — kill the agent after N seconds of silence\n# auto_stage_all = true\n# verbose        = false\n")
```

### Integration Points

```yaml
NO struct/signature/public-API changes. Five flags + two loadFlags arms + string edits + docs.

CODE:
  - internal/cmd/root.go — 5 vars + 5 StringVars + the --timeout/:264/:266 help sweep
  - internal/config/load.go — loadFlags global + per-role idle arms (loadFlags ONLY — S2 owns the rest of the file's idle seams)
  - internal/config/bootstrap.go — template :174 + existing :359/:375 reword
  - internal/cmd/config.go — the inert-template trio (:649/:675/:694) — flagged adjacent
TESTS:
  - internal/cmd/root_test.go — :324 → 600s (+ sibling stale defaults)
  - internal/config/load_test.go — newFlagSet + 3 flag-test twins (90s)
  - internal/config/bootstrap_test.go — +TestGenerateBootstrapConfig_DeadlineDefaults
DOCS (Mode A):
  - docs/cli.md — :27, +1 row; :60-63 reword, +4 rows; map :468/:499 additions; :452/:454
  - docs/configuration.md — :91, :99, :138, :193, :212-215, :232, :240-241, :254, :264

CONSUMED:
  - S1: Config.IdleTimeout / RoleConfig.IdleTimeout / Defaults 600s+60s (landed)
  - S2: setRoleIdleTimeout + bootstrap's two idle doc lines (contract — verify, don't re-add)

DOWNSTREAM (do NOT implement):
  - P1.M2.T1.S2/S3: executor Deadlines struct + 13 call sites → ResolveRoleDeadlines
  - P1.M3.T1.S4: every-layer observably binds per role (behavioral verification of S3's flags)
  - P3.M1.T1.S1: README + how-it-works prose; P3.M1.T1.S2: cross-file retirement grep

UNCHANGED: S2's env/git/file layers; config.go/roles.go (S1); the executor; CurrentConfigVersion (FR-B4).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...                  # the 5 new flags + both loadFlags arms compile
go vet ./...
gofmt -l internal/cmd/ internal/config/   # Expected: empty.
make lint                       # Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new flag twins + template content test
go test ./internal/config/ -run 'TestLoadFlags_IdleTimeout|TestLoadFlags_PerRoleIdleTimeout|TestLoad_IdleTimeout_FlagBeatsEnv|TestGenerateBootstrapConfig_DeadlineDefaults' -v
# Expected: PASS — 90s sentinels; malformed silently ignored; flag 90s beats env 45s; both template lines present; no "120s".

# Whole config package (S2's twins + the signature-rippled tests still green)
go test ./internal/config/... -v

# The cmd package — MUST BE GREEN (currently RED from S1's residue; S3 fixes root_test.go:324)
go test ./internal/cmd/ -v

# Whole suite
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# The real CLI surface — the 5 flags appear in --help with correct help text
go run . --help 2>&1 | grep -c 'idle-timeout'          # Expected: 5 (global + 4 per-role)
go run . --help 2>&1 | grep -- '--timeout' | grep -c '600s'   # Expected: ≥1 (the retitled help)

# The generated template (bootstrap)
d=$(mktemp -d) && cd "$d"
# (Fastest: the unit test already asserts content; a live `stagecoach config init` requires a repo.)
git init -q && git config user.email t@e && git config user.name t
go run . config init --force 2>/dev/null || true
grep -n 'idle_timeout\|timeout' "$(go run . config path 2>/dev/null | tail -1)" 2>/dev/null | head -4
rm -rf "$d"
# Expected: `# idle_timeout   = "60s"` and `# timeout        = "600s"` lines present.

# Full behavioral verification (idle-kill semantics through the pipeline) is P1.M3.T1.S4 — NOT S3.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: the 5 flags are registered as STRINGs at zero default
grep -n 'idle-timeout' internal/cmd/root.go
# Expected: 5 StringVar registrations + the var block (no DurationVar anywhere).

# Grep guard: loadFlags parses both arms
grep -n '"idle-timeout"\|idle-timeout"' internal/config/load.go
# Expected: the global Changed arm + the per-role role+"-idle-timeout" arm.

# Grep guard: NO snake_case git key ANYWHERE (user-facing or code)
grep -rn 'stagecoach.idle_timeout' internal/ docs/
# Expected: EMPTY (git rejects underscores; the key is idleTimeout — camelCase).

# Grep guard: S2's bootstrap doc lines exist (verify — don't re-add)
grep -c 'STAGECOACH_IDLE_TIMEOUT\|stagecoach.idleTimeout' internal/config/bootstrap.go
# Expected: ≥2 (S2's env + git lines).

# Grep guard: the template content + no stale 120s in the populated template
grep -n 'idle_timeout   = "60s"\|timeout        = "600s"' internal/config/bootstrap.go
# Expected: the rewritten :174 WriteString contains both.

# Grep guard: the stale default is gone from user-facing cmd/config text
grep -n '"120s"\|120s' internal/cmd/config.go internal/cmd/root.go internal/config/bootstrap.go
# Expected: no default-implying 120s (arbitrary example durations are acceptable only where the
#           inventory says so — the flag helps and templates must say 600s).

# Grep guard: docs match the inventory — no 480s, idle rows present
grep -rn '480s' docs/cli.md docs/configuration.md    # Expected: EMPTY
grep -c 'idle-timeout' docs/cli.md                   # Expected: ≥10 (flag rows + map rows + exit row)
grep -c 'idle_timeout\|idleTimeout\|IDLE_TIMEOUT' docs/configuration.md   # Expected: ≥8

# Scope-boundary guard
git diff --name-only
# Expected: internal/cmd/{root.go,root_test.go,config.go} + internal/config/{load.go,load_test.go,
#           bootstrap.go,bootstrap_test.go} + docs/{cli.md,configuration.md} — nothing else
#           (S1/S2's config-package files are already committed by their tasks).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `gofmt -l internal/cmd/ internal/config/` empty
- [ ] `make lint` zero errors
- [ ] `make test` all pass
- [ ] `go test ./internal/cmd/` GREEN (was red from S1's residue)

### Feature Validation
- [ ] 5 flags registered (STRING, zero default); `--help` shows them; `--timeout` help says hard-cap 600s
- [ ] loadFlags: global + per-role idle arms; parseTimeout (both forms); malformed silently ignored; Changed-only
- [ ] Bootstrap template: `# idle_timeout = "60s"` + `# timeout = "600s"` with one-line comments; content test asserts both
- [ ] cmd/config.go inert template reworded + idle variants (adjacent surface, flagged)
- [ ] docs/cli.md: retitle + 5 new rows + map additions + exit-124 rewording
- [ ] docs/configuration.md: all 9 anchor groups per the inventory
- [ ] Flag-beats-env twin (90s > 45s) green

### Scope-Boundary Validation
- [ ] S2's env/git/file layers + setRoleIdleTimeout NOT re-implemented (verified present, not duplicated)
- [ ] bootstrap's two idle doc lines NOT re-added (S2's)
- [ ] config.go/roles.go (S1), executor/call sites (P1.M2), stubagent (P1.M3) UNTOUCHED
- [ ] README.md / docs/how-it-works.md UNTOUCHED (P3.M1.T1.S1); docs/providers.md UNTOUCHED (P2)
- [ ] CurrentConfigVersion UNCHANGED (FR-B4)

### Code Quality
- [ ] Flag helps name the env + git homes + the default, mirroring the timeout flags' shape
- [ ] Comments on the new arms document the silent-ignore asymmetry
- [ ] Template comments concise (idle = kill on silence; timeout = hard cap)
- [ ] No default-implying "120s" remains in any surface S3 owns

---

## Anti-Patterns to Avoid

- ❌ Don't use pflag DurationVar for the idle flags — the deadline flags are STRINGs at zero default so
  `fs.Changed` reflects "user passed it" and parseTimeout can accept both "90s" and bare 90 (FINDING 7).
- ❌ Don't error on malformed flag values — loadFlags has NO error return; silent-ignore is the documented
  flag-layer asymmetry (env hard-errors, S2's layer). Mirror the existing arms.
- ❌ Don't write `stagecoach.idle_timeout` anywhere (help text, docs, code) — git rejects underscores in
  the final key segment; the key is `stagecoach.idleTimeout` / `stagecoach.role.<role>.idleTimeout`
  (camelCase). The TOML/env spellings ARE snake/upper (`idle_timeout` / `STAGECOACH_IDLE_TIMEOUT`).
- ❌ Don't re-add S2's bootstrap doc lines (the STAGECOACH_IDLE_TIMEOUT env line, the idleTimeout git
  line) — S2's contract adds them; S3 verifies via grep. S3's bootstrap edits: the template BODY (:174)
  and rewording the EXISTING :359/:375 timeout lines.
- ❌ Don't touch S2's loadEnv/loadGitConfig/file.go seams when adding the loadFlags arms — the flag layer
  is loadFlags ONLY; the per-role arm goes INSIDE the existing roleNames loop (no parallel loop).
- ❌ Don't forget root_test.go:324 — the cmd package is RED right now (120s assertion vs S1's 600s
  default); S3 owns root.go's tests and must bring the package green.
- ❌ Don't leave "default 120s" / "480s" in any surface S3 owns — sweep root.go helps, bootstrap's
  template + doc lines, cmd/config.go's trio, and all 9 configuration.md anchors + 5 cli.md anchor
  groups from the inventory. (README/how-it-works are P3's — leave them.)
- ❌ Don't put idle_timeout ABOVE timeout in the templates — the drift inventory specifies "directly
  below" the timeout line.
- ❌ Don't reuse 45s/60s/120s/600s as the flag-test sentinel — use 90s (S2's env/git/file twins use 45s;
  flag-beats-env needs 90s > 45s).
- ❌ Don't bump CurrentConfigVersion — additive optional keys never bump (FR-B4).

---

## Confidence Score: 9/10

One-pass success is very high: the five flag registrations and both loadFlags arms are verbatim mirrors of
quoted, live-verified patterns (config_ladder §6 + direct reads); the template rewrite is a single
WriteString with its column arithmetic spelled out; the docs work is enumerated anchor-by-anchor against
the drift inventory, which quotes the current text verbatim — S3 just applies the listed changes. The two
genuine risks are both handled explicitly: (1) the S2/S3 bootstrap.go boundary (both PRP texts mention the
env/git doc lines — resolved by treating S2 as the contract: verify, don't re-add; the prerequisite grep
gate catches a not-yet-landed S2 before compile failure); (2) the currently-RED cmd package from S1's
600s residue — surfaced by the Level-2 gate and fixed in Task 5, with the caveat that S1 may fix it first
(idempotent either way). The -1 covers the two judgment surfaces beyond the item's literal line list
(cmd/config.go's inert-template trio; the optional cli.md Git-column fix for the timeout rows) — both are
flagged with their drift-inventory justification so the implementer executes them deliberately rather
than discovering them mid-flight, and skipping the optional one is harmless.
