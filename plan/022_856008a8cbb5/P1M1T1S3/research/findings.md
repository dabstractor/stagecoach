# Research Findings — P1.M1.T1.S3 (CLI flags --idle-timeout + 4 per-role, bootstrap template, Mode-A docs)

Verified by direct source read at the live tree (S1 landed; S2 NOT yet landed — its PRP is the contract),
plus `plan/022_856008a8cbb5/architecture/config_ladder.md` (§5–6) and `docs_drift_inventory.md` (the docs
authority — "Execute the exact line list in docs_drift_inventory.md"). No files modified.

## 1. Live-tree state: S1 LANDED, S2 NOT YET

- **S1 landed** (git status: config.go/roles.go + their tests modified): `Config.Timeout` default is
  **600s** (config.go:86 comment + :229 seed), `Config.IdleTimeout` exists (:87, default **60s**, :230),
  `RoleConfig.IdleTimeout` (:43, `0 ⇒ inherit`), `ResolveRoleDeadlines(role, cfg) (idle, hard)` at
  roles.go:78, and `defaultRoleTimeouts` is **DELETED** (grep clean).
- **S2 NOT landed** (grep for `IDLE_TIMEOUT|setRoleIdleTimeout|idleTimeout` across
  internal/config/{load,git,file,bootstrap}.go → EMPTY). S2's PRP is the CONTRACT: it will add
  `setRoleIdleTimeout` (load.go, mirror of setRoleTimeout :66-75), the env reads, the git keys, the file
  keys, and **bootstrap.go's two idle DOC lines** (env block after :359, git block after :375).
- **S3 DEPENDS ON S2**: S3's loadFlags per-role arm calls `cfg.setRoleIdleTimeout` (S2's helper).
  Prerequisite grep: `grep -c setRoleIdleTimeout internal/config/load.go` ≥ 1.

## 2. ⚠️ The S2/S3 bootstrap.go boundary (contract resolution)

The S3 item text says "env/git doc comments gain the idle variants", but **S2's PRP Task 4 already adds
exactly those two lines** and explicitly fences: "DO NOT touch the template body (~L174) or the flag doc
block — those are S3" and "S3 owns the 120s→600s prose sweep with the template + flag help".
Resolution (treat S2 as landed, per the parallel contract):
- **S2 adds**: `#   STAGECOACH_IDLE_TIMEOUT   idle deadline (kill on silence), e.g. "45s" or 45 (seconds)`
  (after :359) and `#   git config stagecoach.idleTimeout 45s` (after :375). S3 does NOT re-add them —
  VERIFY via grep.
- **S3 owns**: the template body L174 (`# timeout = "120s"` → `"600s"` + add `# idle_timeout = "60s"`),
  and rewording the EXISTING timeout lines (:359 env example 120s→600s hard-cap; :375 git example 120s→600s).

## 3. root.go — the exact registration patterns to mirror (verified live)

- Vars: config-backed block L52-59 (`flagTimeout string` at :56, comment "STRING — config.Load reads via
  fs.GetString(\"timeout\") (FINDING 7)"); per-role block L61-84 ends with
  `flagPlannerTimeout/flagStagerTimeout/flagMessageTimeout/flagArbiterTimeout string` (:81-84).
- `--timeout` registration **:169** (on `pf := rootCmd.PersistentFlags()`, :164):
  `"Generation timeout, e.g. \"120s\" or 120 (env STAGECOACH_TIMEOUT, git stagecoach.timeout; default 120s)"`
  — the help EMBEDS the stale 120s default; S3 retitles to the 600s hard-cap wording.
- Per-role timeout flags **:258-266** (verified verbatim): planner/stager/message/arbiter StringVars,
  each `"Per-role generation timeout for …, e.g. \"…s\" or … (env STAGECOACH_<ROLE>_TIMEOUT; git
  stagecoach.role.<role>.timeout)"`. Message/arbiter examples say `"120s" or 120` — drift-inventory
  MUST-CHANGE (:264/:266): reword to "hard-cap" + stop implying the default.
- ALL are pflag STRINGs, zero default; root.go never parses — parsing is loadFlags-only.

## 4. load.go loadFlags — the exact arms to mirror (verified live, L417-479)

- Global `timeout` arm (L433-439): `if fs.Changed("timeout") { GetString → parseTimeout → cfg.Timeout = d }`
  — **silent-ignore on malformed** (loadFlags has NO error return; env is the hard-error layer — S2's).
- Per-role arm (L466-479, inside `for _, role := range roleNames`): `fs.Changed(role+"-timeout")` →
  GetString → parseTimeout → `cfg.setRoleTimeout(role, d)`. Comment documents the silent-ignore asymmetry.
- S3 adds: global `"idle-timeout"` arm right after the timeout arm; per-role `role+"-idle-timeout"` arm
  right after the timeout arm inside the SAME loop (→ `cfg.setRoleIdleTimeout(role, d)`, S2's helper).
- `parseTimeout` (load.go :673-684) accepts `"60s"` AND bare `60`.

## 5. bootstrap.go — the template line + doc blocks (verified live)

- **Template body :174** (inside `buildBootstrapConfig` :156-438):
  `b.WriteString("# model          = \"\"\n# timeout        = \"120s\"\n# auto_stage_all = true\n# verbose        = false\n")`
  — name column padded to 15 after "# " (`idle_timeout` = 12 chars + 3 spaces). The `reasoning` line
  above (:173) shows the trailing-comment precedent. S3 rewrites this ONE WriteString: timeout → "600s"
  with a hard-cap comment; add `# idle_timeout   = "60s"` directly BELOW timeout (drift-inventory: "directly
  below, mirroring the timeout line's style") with a kill-on-silence comment.
- **Env doc block :352-370** (STAGECOACH_TIMEOUT at :359: `generation timeout, e.g. "120s" or 120 (seconds)`)
  — S2 adds the idle line after it; S3 rewords :359's example to 600s + hard-cap phrasing.
- **Git doc block :372-377** (`#   git config stagecoach.timeout 120s` at :375; :376 shows the camelCase
  `autoStageAll` convention) — S2 adds the idleTimeout line; S3 rewords :375's 120s → 600s.
- **CLI flag doc block :380-386** lists only provider/model/role/commits flags — pre-existing gap: no
  --timeout/--<role>-timeout rows. NOT required by the item; optional 2-line close (--timeout +
  --idle-timeout) noted.
- Public entry: `GenerateBootstrapConfig(prov string) string` (:22) wraps `buildBootstrapConfig` — the
  template content test asserts on its output. bootstrap_test.go currently has NO timeout-line assertion
  (grep clean) — S3 ADDS the content test ("template content test asserts both lines").

## 6. ⚠️ internal/cmd/config.go — a THIRD copy of the user-facing strings (adjacent surface)

The inert `config init --template` literal (inside internal/cmd/config.go, ~:640-700) duplicates the
header text: env line `#   STAGECOACH_TIMEOUT    generation timeout, e.g. "120s" or 120 (seconds)` (:649),
git line `#   git config stagecoach.timeout 120s` (:675), and `[defaults]` line
`# timeout        = "120s"   # generation timeout (Go duration string…)` (:694). Drift-inventory marks all
three MUST-CHANGE (user-visible; they lie once S1's 600s lands). NOT in S3's item line list, but identical
in nature to bootstrap.go's surfaces and in S3's package — include as a flagged adjacent task (3-4 line
edits: reword to 600s + add the idle variants), rather than leaving wrong user-facing text for the
P3.M1.T1.S2 end-of-plan grep sweep.

## 7. ⚠️ The cmd package is currently RED from S1's residue

`go test ./internal/cmd/` FAILS at research time. `root_test.go:324`
(`TestRoot_LoadsConfigAndRunsStub`, :298) asserts `cfg.Timeout != 120*time.Second … want 120s (default)`
but `Defaults().Timeout` is now 600s. It's drift-inventory MUST-CHANGE and in S3's file territory —
S3 updates it to 600s (and sweeps any other cmd-package default assertion S1's change broke; S1 may fix
it before S3 starts — check `go test ./internal/cmd/` at start).

## 8. Test harness conventions (verified live)

- **newFlagSet helper** (load_test.go :49-79): pre-registers every Config-backed flag at zero defaults —
  `fs.String("timeout", "", "")` at :56; roleNames loop :64-68 registers `role+"-timeout"`. S3 adds
  `fs.String("idle-timeout", "", "")` + `fs.String(role+"-idle-timeout", "", "")` in the loop; update the
  helper's doc comment. Extending is behavior-neutral for tests that don't Set them.
- **Flag tests to mirror**: TestLoadFlags_TimeoutString (:506), TestLoadFlags_PerRoleTimeout (:539;
  duration + bare-int + field-merge via a sibling flag), TestLoadFlags_PerRoleTimeout_MalformedIgnored
  (:567 — pins the silent-ignore asymmetry), TestLoad_PerRoleTimeout_FlagBeatsEnv (:1145 — full Load
  after loadEnvSetup + chdir; the flag>env template).
- **Sentinels**: S2's idle twins use 45s (env/git/file). S3's flag tests use **90s** (distinct from both
  45s and the timeout tests' 60s/120s/600s; flag-beats-env needs 90s > 45s).

## 9. Docs anchors (verified live; both files untouched by S1/S2 — anchors stable)

**docs/cli.md** — flag table header `| Flag | Type | Default | Env var | Git config | Description |`:
- :27 `--timeout` row (default "120s") → retitle hard-cap 600s; ADD `--idle-timeout` row directly after.
- :60-63 the four `--<role>-timeout` rows → reword "Per-role hard-cap generation timeout…"; message/arbiter
  examples `"120s"/"120"` stop implying the default. ADD four `--<role>-idle-timeout` rows after :63
  (before `--version` :64). NOTE: existing per-role timeout rows carry `—` in the Git config column even
  though the keys exist (pre-existing inaccuracy vs the map table); the NEW idle rows carry the REAL key
  `stagecoach.role.<role>.idleTimeout`; optionally fix the timeout rows' `—` in passing.
- Map table (:462-500): `--timeout` row :468 → ADD `--idle-timeout | STAGECOACH_IDLE_TIMEOUT |
  stagecoach.idleTimeout` after it; per-role rows :496-499 → ADD four
  `--<role>-idle-timeout | STAGECOACH_<ROLE>_IDLE_TIMEOUT | stagecoach.role.<role>.idleTimeout` rows
  after :499 (before the NO_PARENT_WATCHDOG row).
- Exit table :452 `| 124 | Timeout (generation exceeded --timeout). |` → "Deadline exceeded (idle or hard
  cap)…" per the inventory; prose :454 "A timeout is reported as 124…" → "A deadline kill (idle or hard
  cap) is reported as 124…".

**docs/configuration.md** (inventory quotes each verbatim):
- :91 [defaults] template comment (120s + "planner role defaults to 480s") → 600s hard-cap wording,
  drop the 480s clause, ADD `# idle_timeout   = "60s"` directly below.
- :99 [role.planner] comment ("overrides the planner's 480s built-in") → no-built-in wording; optionally
  add a commented `# idle_timeout = "90s"` per-role example.
- :138 defaults table `| timeout | 120s | … |` → `600s (hard cap)` + INSERT `| idle_timeout | 60s | … |`.
- :193 STAGECOACH_TIMEOUT env row (has the "planner … 480s … NOT changed by this" clause + example 60s
  that now collides with the idle default) → hard-cap wording, drop the clause, example `600s`; ADD a
  STAGECOACH_IDLE_TIMEOUT row.
- :212-215 the four per-role env rows (drop 480s/120s-inherit clauses; "inherits the 600s global hard cap";
  keep L214's "single-commit path's only role" note) + ADD four `STAGECOACH_<ROLE>_IDLE_TIMEOUT` rows.
- :232 git ini example (`timeout = 120s`) → `timeout = 600s` + ADD `idleTimeout = 60s` (CAMELCASE);
  :240-241 git key table rows (reword :240, add `stagecoach.idleTimeout` + `stagecoach.role.<role>.idleTimeout`
  rows); :254 NOTE first sentence gains "and idle-timeout keys"; :264 decompose prose (planner-480s
  sentence) → every-role idle+hard-cap rewrite.

## 10. Scope fences (do NOT do in S3)
- env/git/file layers + setRoleIdleTimeout + bootstrap's two idle doc lines (S2 — verify, don't re-add).
- config.go/roles.go/ResolveRoleDeadlines (S1, landed).
- Executor/activity/13 call sites (P1.M2), stubagent (P1.M3), behavioral verification (P1.M3.T1.S4).
- docs/how-it-works.md + README.md (P3.M1.T1.S1), docs/providers.md event_stream (P2).
- CurrentConfigVersion (FR-B4: additive optional keys never bump).
- The bootstrap CLI-flag doc block gap (:380-386 lacks --timeout) — optional only.

## 11. Validation
`go build ./...`, `go vet ./...`, `gofmt -l internal/cmd/ internal/config/`, `go test ./internal/config/
-run 'LoadFlags|Bootstrap' -v`, `go test ./internal/config/...`, `go test ./internal/cmd/` (must be GREEN —
currently red from S1's residue), `make test`, `make lint`. Grep gates: the 5 new flags registered;
loadFlags idle arms; no snake_case git key `stagecoach.idle_timeout` anywhere; template contains both
lines; docs contain the idle rows; `go run . --help | grep idle-timeout` smoke.
