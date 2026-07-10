# Delta PRD — v2.8: Per-Role Generation Timeouts + Decompose Freeze Hardening

| Field              | Value                                                                                                                                                                                                                                                                                                                                                                                                                              |
| ------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Base PRD**       | Stagecoach v2.7 (orphaned-run lock reclamation shipped)                                                                                                                                                                                                                                                                                                                                                                            |
| **Target PRD**     | Stagecoach v2.8                                                                                                                                                                                                                                                                                                                                                                                                                    |
| **Delta size**     | Small-to-medium. One feature that mirrors an existing, heavily-implemented pattern (per-role provider/model/reasoning), plus two tiny hardening edits.                                                                                                                                                                                                                                                                            |
| **This revision**  | **(1) Per-role generation timeouts** (§9.15 new FR-R7; amended FR25 / FR-T5 / §16.1–§16.4): each role (planner, stager, message, arbiter) resolves its OWN timeout — there is no single shared budget. The global `--timeout` (default `120s`) is the fallback; `planner` ships a built-in role default of `480s` (the heavy role). Per-role overrides `--<role>-timeout` / `STAGECOACH_<ROLE>_TIMEOUT` / `[role.<role>].timeout` mirror provider/model/reasoning. **(2) Decompose freeze hardening** (§9.14 new FR-M1e; amended FR-M1c): the `Decompose` entrypoint re-asserts the empty-index precondition (defense-in-depth) with a clear actionable error, and the freeze-violation error names the offending paths AND the remedy. |
| **Out of scope**   | No commit/CAS/rescue/lock changes. No new git primitives. No provider-manifest changes. No CLI surface change beyond four new `--<role>-timeout` flags. No per-role git-config layer (`stagecoach.role.<role>.timeout`) — the existing per-role resolution does not implement a git-config layer for provider/model/reasoning either (git.go reads globals only); timeout mirrors that (flag/env/file + global), keeping scope minimal and consistent. |
| **Prior research** | `plan/014_37208f58ffa2/architecture/system_context.md` documents the per-role resolution architecture. The per-role provider/model/reasoning pattern (the template for timeout) is fully implemented in `internal/config/{config.go,roles.go,load.go,file.go}` and `internal/cmd/root.go`. |

---

## Motivation

A real `decompose: planner failed: context deadline exceeded` with a thinking-high planner over a large
diff. Today every role shares one global timeout (`cfg.Timeout`, currently seeded to `480s` in
`Defaults()`). That single budget is wrong in both directions: it is over-generous for the fast bare
roles (stager/message/arbiter should be snappy) and still not independently tunable for the planner
(the heavy role that reasons over the entire frozen `T_start` diff before emitting its JSON partition).
The fix gives each role its own timeout, mirrors the proven per-role provider/model/reasoning
resolution exactly, and drops the global default to `120s` while keeping the planner at `480s`.

The freeze hardening (FR-M1e + FR-M1c amend) is a small defense-in-depth cleanup surfaced by the same
large-diff investigation: a stale/buggy decompose trigger that routes a non-empty index into `Decompose`
currently fails with the opaque `freeze violation: … not traceable to T_start`; the entrypoint should
re-assert its precondition and the freeze error should name a remedy.

---

## Phase 1 — Per-Role Generation Timeouts (FR-R7; amended FR25, FR-T5)

### Functional requirement

**FR-R7 (new, §9.15). Per-role generation timeout.** Each role (planner, stager, message, arbiter)
resolves its OWN generation timeout, mirroring provider/model/reasoning (FR-R1–R6). There is no single
shared budget.

- **Layers (highest wins, per role):** `--<role>-timeout` flag > `STAGECOACH_<ROLE>_TIMEOUT` env >
  `[role.<role>].timeout` config-file table > built-in role default > global `[defaults].timeout`
  (`--timeout` / `STAGECOACH_TIMEOUT` / `stagecoach.timeout`, default `120s`).
- **Built-in role defaults:** `planner = 480s` (the heavy role — reasoning over the full frozen
  `T_start` diff routinely exceeds the lighter roles' budget). `stager` / `message` / `arbiter` have NO
  built-in role default ⇒ they inherit the global `120s`. To change the planner's timeout, set
  `--planner-timeout` / `[role.planner].timeout` (the built-in 480s is a per-role floor; the global
  `--timeout` does NOT override it — it governs the other three roles and is the fallback for any role
  without a built-in default).
- **Consumer wiring:** each role's `provider.Execute` call uses ITS resolved timeout, not the global.
  `FR25` is amended: "a configurable **per-role** generation timeout … on timeout, kill that role's
  agent process and enter the rescue path." `FR-T5` is amended: each multi-turn turn uses the **message**
  role's resolved timeout (`message-timeout × (N+1)` total budget).
- **Back-compatibility:** on the single-commit path the only active role is `message`, which inherits the
  global `120s` (was `480s` globally — this is a deliberate default change; users who relied on the old
  480s set `--timeout 480s` or `[role.message].timeout`).

### Milestone M1 — Config resolution layer

Add the timeout field to the per-role resolution machinery, mirroring provider/model/reasoning field-for-field,
and change the global default from 480s to 120s while introducing the planner's 480s built-in role default.

**Task M1.T1 — Per-role timeout config field + 7-layer precedence + defaults**

- **Subtask M1.T1.S1** (SP 2) — Add `Timeout` to `RoleConfig` + `fileRoleConfig`; add `ResolveRoleTimeout`; change the global default to 120s; add the planner 480s built-in role default.
  - **Reference pattern:** `internal/config/roles.go:ResolveRoleModel` and the `setRoleProvider`/`setRoleModel`/`setRoleReasoning` trio in `internal/config/load.go`.
  - **LOGIC:**
    1. `internal/config/config.go`: add `Timeout time.Duration` to `RoleConfig` (toml `timeout`, with a doc comment referencing FR-R7). Change `Defaults().Timeout` from `480 * time.Second` → `120 * time.Second` (the global fallback; FR-R7). Add a new unexported `roleTimeoutDefaults` map `{ "planner": 480 * time.Second }` (the built-in per-role floor; documented as FR-R7). NOTE: `RoleConfig.Timeout` is a `time.Duration`, but the FILE layer decodes durations as strings — so the file twin (`fileRoleConfig.Timeout string`) holds the raw string and is parsed in `materialize` (mirroring how `fileDefaults.Timeout` string is parsed at `file.go:181-185` via `time.ParseDuration`).
    2. `internal/config/roles.go`: add `func ResolveRoleTimeout(role string, cfg Config) time.Duration`. Resolution: (a) if `cfg.Roles[role].Timeout > 0` → return it; (b) else if `roleTimeoutDefaults[role]` exists → return it; (c) else → return `cfg.Timeout` (the global fallback). This puts the built-in role default (planner 480s) ABOVE the global, so the planner keeps 480s unless explicitly overridden per-role; stager/message/arbiter fall through to the global 120s.
    3. `internal/config/file.go`: add `Timeout string \`toml:"timeout"\`` to `fileRoleConfig`; in `materialize` (where `fileRoleConfig` → `RoleConfig`), parse the string via `time.ParseDuration` (mirrors `fileDefaults.Timeout` parsing at lines 181-185; on parse error return a wrapped error naming the `[role.<role>]` table); in `overlay`, propagate non-zero durations field-by-field (mirrors the existing `fileRoleConfig` overlay).
    4. `internal/config/load.go`: (a) env loop (~line 290) — add `STAGECOACH_<ROLE>_TIMEOUT` parsed via `parseTimeout` (the existing helper at line 615 that accepts both "120s" and bare "120"), calling a new `cfg.setRoleTimeout(role, d)` setter (mirrors `setRoleReasoning`); (b) flag loop (~line 425) — add `fs.Changed(role+"-timeout")` → `fs.GetString` → `parseTimeout` → `cfg.setRoleTimeout(role, d)`.
    5. `internal/cmd/root.go`: register `--<role>-timeout` for all four roles inside the existing per-role flag block (~lines 184-252), mirroring `--<role>-reasoning` exactly (StringVar, zero default `""`, help text naming the env + global fallback + the planner's 480s default for the planner flag). Update the global `--timeout` help text (~line 165) from "default 480s" to "default 120s; planner role defaults to 480s (FR-R7)".
  - **OUTPUT:** `config.ResolveRoleTimeout(role, cfg)` resolves a per-role timeout through all layers. `cfg.Timeout` default is 120s; planner gets 480s by default. Consumed by M1.T2.
  - **DOCS (Mode A):** inline godoc on `ResolveRoleTimeout`, `RoleConfig.Timeout`, and `roleTimeoutDefaults` citing FR-R7; update the `Defaults().Timeout` comment (480s→120s) and the `--timeout` flag help in root.go.

### Milestone M2 — Consumer wiring + verification

Each `provider.Execute` call site passes its role's resolved timeout instead of the shared `cfg.Timeout`.

**Task M2.T1 — Route each role's Execute call through ResolveRoleTimeout**

- **Subtask M2.T1.S1** (SP 1) — Update the decompose role files + the single-commit/multiturn/workdesc paths to use the resolved role timeout.
  - **INPUT:** `config.ResolveRoleTimeout(role, cfg)` from M1.T1.S1. The `provider.Execute(ctx, spec, timeout, vb)` signature (timeout is already an explicit parameter — no executor change needed).
  - **LOGIC — replace `cfg.Timeout`/`deps.Config.Timeout` with the role-resolved timeout at each call site:**
    1. `internal/decompose/planner.go:124` → `config.ResolveRoleTimeout("planner", deps.Config)`.
    2. `internal/decompose/stager.go:110` → `config.ResolveRoleTimeout("stager", deps.Config)`.
    3. `internal/decompose/message.go:155` → `config.ResolveRoleTimeout("message", deps.Config)`.
    4. `internal/decompose/arbiter.go:100` → `config.ResolveRoleTimeout("arbiter", deps.Config)`.
    5. `internal/generate/generate.go:335` (single-commit message role) → `config.ResolveRoleTimeout("message", cfg)`.
    6. `internal/generate/multiturn.go:165,176,187` (FR-T5 — multi-turn is a message-role fallback) → `config.ResolveRoleTimeout("message", cfg)`. Also update the total-budget progress line (~line 426 `cfg.Timeout * turns`) to use the message timeout. Update the doc comment at multiturn.go:132 (`Per-turn timeout = cfg.Timeout`) to reference `ResolveRoleTimeout("message", cfg)`.
    7. `internal/generate/workdesc.go:75,106,122` (work-description mode is message-role) → `config.ResolveRoleTimeout("message", cfg)`.
    8. Leave `internal/cmd/models.go:144` (`cfg.Timeout` for `list_models_command`) UNCHANGED — that is not an agent role.
  - **OUTPUT:** every role's generation is bounded by its own timeout. The planner gets 480s by default; the bare roles get 120s. The single-commit message role gets 120s (the global) unless overridden.
  - **DOCS (Mode A):** update the inline comments at each call site and the multiturn.go total-budget comment to reference FR-R7.

- **Subtask M2.T1.S2** (SP 1) — Tests for per-role timeout resolution + consumer wiring.
  - **LOGIC:** (a) extend `internal/config/roles_test.go` with `ResolveRoleTimeout` cases: planner-default → 480s; message-default → 120s (global); per-role `[role.planner].timeout` override wins; env `STAGECOACH_PLANNER_TIMEOUT` wins over file; global `--timeout` override does NOT change planner (stays 480s) but DOES change message. (b) Add a decompose test asserting the planner Execute receives 480s and the message Execute receives 120s by default (stub provider records the timeout it was given, or assert via the existing executor seam). (c) Assert FR-T5: a multiturn run's per-turn timeout is the message role's resolved timeout.

---

## Phase 2 — Decompose Freeze Hardening (FR-M1c amend + FR-M1e new; §9.14)

Tiny defense-in-depth cleanup. Two edits.

### Functional requirements

- **FR-M1e (new, §9.14). Entrypoint empty-index re-assertion (defense-in-depth).** The `Decompose`
  entrypoint re-asserts FR-M1's empty-index precondition (`HasStagedChanges` false) and refuses with a
  clear, actionable error if the index is non-empty. The trigger (CLI router, `shouldDecompose`) remains
  the primary guard; this re-assert ensures a stale/buggy trigger fails loudly with a remedy ("unstage
  first or re-run from a clean trigger") instead of the opaque `freeze violation: … not traceable to
  T_start`.
- **FR-M1c (amended, §9.14).** The freeze-violation hard error now NAMES the offending paths AND the
  remedy (unstage them or re-run from a clean trigger). (The current errors already name the paths; this
  adds the remedy text.)

**Task P2.T1 — Decompose entrypoint re-check + freeze error remedy (single task, SP 1)**

- **Subtask P2.T1.S1** — Add the empty-index re-assertion to `Decompose()`; add remedy text to the freeze-violation errors.
  - **LOGIC:**
    1. `internal/decompose/decompose.go` `Decompose()` — immediately AFTER the single escape-hatch routing (`if deps.Config.Single || deps.Config.Commits == 1 { return runSingleEscape(...) }`, ~line 143) and BEFORE baseTree derivation, add: `if has, err := deps.Git.HasStagedChanges(ctx); err != nil { return DecomposeResult{}, fmt.Errorf("%w: check staged: %w", ErrDecomposeFailed, err) } else if has { return DecomposeResult{}, fmt.Errorf("%w: decomposition requires an empty index, but staged changes are present. Unstage them first (git reset) or commit them, then re-run; or use --single to commit the staged set as one commit", ErrDecomposeFailed) }`. Update the PRECONDITION comment (~line 121) from "Decompose does NOT re-check this" to "Decompose re-asserts this (FR-M1e, defense-in-depth)".
    2. `internal/decompose/stager.go:verifyFreezeSubset` — append remedy text to BOTH error returns: the "staged paths not present in T_start" error (~line 189) and the "staged content not traceable to T_start" error (~line 192). Add: `; unstage the offending path(s) or re-run from a clean trigger (nothing staged)`. The paths are already named via the `%s` join.
  - **OUTPUT:** a non-empty index routed into `Decompose` fails immediately with a clear, actionable message; freeze violations name both paths and remedy.
  - **DOCS (Mode A):** update the inline comments at the re-check site and the two error sites citing FR-M1e / FR-M1c.
  - **TESTS:** extend `internal/decompose/decompose_test.go` — assert `Decompose` with a staged file returns the new actionable error (not a freeze violation); extend `internal/decompose/stager_test.go` — assert the freeze-violation error string contains the remedy text.

---

## Phase 3 — Sync changeset-level documentation (Mode B)

Cross-cutting doc updates that only make sense once Phases 1–2 land. Mirrors the breakdown agent's Mode B catch-all.

**Task P3.T1 — Sync docs for v2.8 (SP 1)**

- **Subtask P3.T1.S1** — Update the bootstrap config template, README, and any timeout/per-role reference docs.
  - **LOGIC:**
    1. `internal/config/bootstrap.go`: the `[defaults]` template line `# timeout = "480s"` (~line 161) → `# timeout = "120s"` with a comment noting the planner role defaults to 480s (FR-R7); the env-var reference block (~line 252 `STAGECOACH_TIMEOUT`) → note per-role `STAGECOACH_<ROLE>_TIMEOUT` variants and the planner 480s default; the git-config example (~line 268) unchanged.
    2. `README.md`: add `--<role>-timeout` to the CLI flags reference (or note per-role timeouts exist); update any "default 480s" mention to "default 120s (planner 480s)".
    3. `docs/`: if a configuration or per-role reference doc exists, document the per-role timeout resolution and the planner 480s default. Run a grep for stale "480s"/"default 480s"/"single timeout" claims and reconcile them with the per-role model.
  - **DOCS (Mode B):** this IS the documentation sync task.

---

## What is NOT changing

- **No commit/CAS/rescue/lock changes.** A per-role timeout expiry still routes through the existing
  rescue path (FR-V7/§18.3); the only difference is which budget bounded the call.
- **No provider-manifest changes.** Timeout is not a manifest field (it is a stagecoach-side budget).
- **No executor changes.** `provider.Execute` already takes `timeout time.Duration` as an explicit
  parameter — the call sites just pass a different value.
- **No per-role git-config layer.** The existing per-role resolution (provider/model/reasoning) does not
  implement `stagecoach.role.<role>.*` git-config reading (`git.go` reads globals only). Timeout mirrors
  that exactly (flag/env/file + global). Adding a per-role git-config layer is a separate, larger change
  that would apply to all four fields uniformly — out of scope for this delta.
- **No FR-R7 "manifest default" layer.** Unlike model (which has a manifest `default_model`), timeout has
  no manifest field. The built-in role default map (`roleTimeoutDefaults`) is the timeout analogue.

## Risk

| Risk                                                                   | Likelihood | Impact | Mitigation                                                                                                                                                |
| ---------------------------------------------------------------------- | ---------- | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Global default drop 480s→120s surprises single-commit users            | Medium     | Low    | Deliberate (PRD FR-R7); documented in help text + bootstrap. Users who need the old budget set `--timeout 480s` or `[role.message].timeout`.              |
| Planner 480s floor not overridden by a raised global (footgun)         | Low        | Low    | Documented in flag help + FR-R7. The floor is intentional (planner is heavy); per-role override (`--planner-timeout`) is the documented escape.           |
| Missed an Execute call site (a role silently keeps the global)         | Low        | Low    | The call-site list (M2.T1.S1) enumerates all 7; a grep for `cfg.Timeout`/`deps.Config.Timeout` in `generate`+`decompose` is the verification gate.        |
| Entrypoint re-check races a concurrent `git add` between router and Decompose | Very Low | Low    | Defense-in-depth only; the freeze (FR-M1b/c/d) remains the real content guarantee. The re-check just improves the error message for the stale-trigger case. |
