# System context — v3.5 delta: activity-based deadlines + event-stream output

## Base state (verified this session)

- Repo HEAD = `df98a9a` ("spec: add idle + hard-cap generation deadlines (v3.5)") — exactly the commit
  the PRD's implementation-surface table was verified against. Only untracked file: this plan dir.
- The **spec text is already committed**; code, tests, and user-facing docs lag. This delta is
  implementation-only. Spec §9.30 (FR-A1–A5, FR-S1–S4), §9.15 FR-R7, §9.5 FR25, §9.24 FR-T5/T7 were
  read verbatim from `spec/01-product.md` (§9.30 starts at L598) — FR numbering below matches it.
- Prior session `plan/021_086bbc57a2b2` covered only the v3.4 format subsystem — not applicable.
- `internal/provider/` contains: builtin.go, executor.go, manifest.go, merge.go, parse.go,
  procgroup_unix.go, procgroup_windows.go, registry.go, render.go (+ tests). No activity*.go or
  event-stream code exists yet (confirmed by directory listing; agent A double-checks).
- `internal/config/` has both `roles.go` and `role_defaults.go` (agent B maps which holds
  `defaultRoleTimeouts`). Test files exist for every seam (config/roles/load/git/bootstrap).

## The feature in one paragraph

Every agent invocation is bounded by TWO deadlines instead of one wall clock: an **idle deadline**
(default 60s — kill on no observable byte activity from the agent's process group; the primary kill
criterion) and a **hard cap** (default 600s, was 120s — the absolute backstop for false-alive
pathologies: timer heartbeats, file re-reads, retry loops; MUST NOT be removed). Activity channels:
(a) stdout/stderr pipe writes (portable, meaningful for progressive/event-mode providers) and
(b) Linux-only procfs aggregate `rchar` over the agent's process group (`pgrp == agent pid` —
guaranteed by existing `Setpgid`). One idle clock, reset by whichever channel moved last. Where no
channel exists (non-Linux + buffered print mode), idle is disarmed → exactly v3.4 behavior. A new
manifest field `event_stream` switches a provider to its native progressive event-stream mode,
making the pipe channel trustworthy on every platform and surfacing in-stream errors.

## Locked design decisions (from PRD §2 — binding for all subtasks)

- **D1** `provider.Execute(ctx, spec, dl Deadlines, vb)` — struct, not duration. `Idle <= 0` ⇒ idle
  not armed. All 13 role call sites updated mechanically.
- **D2** Idle kill preserves the error contract EXACTLY (FR-A5): returns an error satisfying
  `errors.Is(err, context.DeadlineExceeded)` (e.g. `ErrIdleDeadline` sentinel with `Is()` method) so
  every existing DeadlineExceeded branch (exit-124, ErrTimeout rescue, no-retry, FR-T7 abort) works
  unchanged. Only the verbose cause line differs.
- **D3** Activity channels: (a) counting writer wrapping the stdout/stderr buffers — atomic
  last-write timestamp + mutex-guarded buffer (buffer growth during `Wait` is the signal; no data
  races); (b) Linux poller summing `rchar` over `/proc/[0-9]*/stat` members with `pgrp == agentPID`,
  ~0.5s tick. One watcher goroutine resets a single idle deadline from whichever channel moved last;
  observes `ctx.Done()` and exits.
- **D4** Arming rule (mechanical degradation clause): idle armed iff `Idle > 0` AND
  (`runtime.GOOS == "linux"` OR `spec.ProgressiveStdout`). `CmdSpec.ProgressiveStdout` is set by
  `Render` iff manifest `event_stream` is non-empty. Stagecoach never fabricates activity.
- **D5** Defaults: `Config.Timeout` 120s→600s; new `Config.IdleTimeout` 60s. `defaultRoleTimeouts`
  (planner 480s) DELETED — `ResolveRoleDeadlines(role, cfg) (idle, hard)` returns
  per-role-override-or-global for every role; no role ships a built-in deadline.
- **D6** New `ui` verbose helper (e.g. `VerboseCause(reason)`) emitting
  `DEBUG: cause: idle deadline (60s without observable activity)` / `DEBUG: cause: hard cap (600s)`,
  called by Execute on a deadline kill.
- **D7** Multiturn: each turn's Execute gets the message role's resolved Deadlines (fresh idle clock
  per turn is automatic — one Execute per turn). Progress line multiplies the **hard cap** and
  appends "(worst case)".
- **D8** `event_stream` is data; extraction is code: manifest field carries flag tokens (rendered
  after the mode flag-set, before `print_flag`); message assembly from the terminal message event is
  a per-provider extractor registered in Go alongside built-in manifests. Manifest declaring
  `event_stream` for a provider with no registered extractor = hard Render-time config error.
  Field populated ONLY for FR-S2-verified providers; shipping `[]` everywhere is compliant (FR-S4).
- **D9** FR-S3: in event mode, an error/abort event with no terminal message event feeds the
  EXISTING StderrHint empty-output cause path (or a sibling with precedence when event mode produced
  no message) — replacing the opaque empty-stdout parse failure.
- **D10** No scope creep: exit codes (124 stays 124), rescue, CAS, lock, index, freeze semantics
  untouched. `docs/packaging.md` out of scope. §20 gains no new harness scenarios.

## Spec anchor map (for prd_selectors cross-reference)

| Spec | File:approx | Content |
| --- | --- | --- |
| §9.30 FR-A1–A5, FR-S1–S4 | 01-product.md L598+ | the whole feature |
| §6.1 G30 | 01-product.md ~L166 | goal mapping |
| §9.5 FR25 | 01-product.md L290 | two-deadline bound; rescue on either kill |
| §9.8 FR35/36 | 01-product.md L309+ | env + git-key lists gain idle keys |
| §9.15 FR-R7 | 01-product.md L379 | per-role two-deadline ladder; 480s retired |
| §9.24 FR-T5/T7 | 01-product.md ~L470–498 | per-turn deadlines; budget = hard-cap×(N+1); "deadline breach" |
| §12.1 manifest schema | 02-providers.md | `event_stream` field, default `[]` |
| §15.2/§15.4 | 04-interface.md | `--idle-timeout` flag row; exit 124 wording |
| §16.1–16.4 | 04-interface.md | defaults, config example, git keys, per-role section |
| §18.2 | 06-reliability.md | failure row: "Generation deadline (idle / hard cap) … kill agent process group" |

## Key facts treated as given (maintainer-measured; do NOT re-verify)

Print-mode CLIs (`pi -p`, `opencode run`) emit zero stdout on healthy 18–19s runs; group `rchar`
moves ~16KB/0.5s while streaming and is exactly flat on a blocked socket; worst legitimate silence
(300KB prompt upload + prefill) is ~13–14s → 60s carries ~4× margin. Event-mode flags per CLI
(pi `--mode json`; claude/cursor/agy `--output-format stream-json` (+partial-message switches);
codex `--json`; opencode `--format json`) — the json-vs-stream-json trap is verified in-spec.

## Companion research files (this dir)

- `provider_subsystem.md` — executor/manifest/render/parse/builtin anatomy (agent A)
- `config_ladder.md` — config/roles/load/git/bootstrap/root flag anatomy (agent B)
- `call_sites_and_error_contract.md` — 13 call sites, exit-124, ui, stub-test harness (agent C)
- `docs_drift_inventory.md` — per-line doc changes required (agent D)
- `external_procfs_notes.md` — procfs API semantics for the poller (agent E)

## Decomposition shape (mirrors PRD §3; IDs are Phase>Milestone>Task>Subtask)

- **P1** Activity-based deadlines: M1 config surface (T1: S1 core+ladder, S2 precedence layers,
  S3 flags+template+Mode-A docs), M2 executor engine (T1: S1 activity infra, S2 Execute rework,
  S3 thread 13 call sites + budget line + Mode-A docs), M3 integration parity (T1: S1 idle-kill
  parity, S2 false-kill + heartbeat regressions, S3 config observability).
- **P2** Event-stream mode: M1 field + execution path (T1: S1 manifest+render, S2 event-mode
  execution + FR-S3 + providers.md), M2 per-provider verification (T1: S1 harness/protocol,
  S2 pi, S3 claude/cursor/agy, S4 codex, S5 opencode).
- **P3** Changeset-level docs sweep (M1.T1: S1 README+how-it-works prose, S2 grep sweep + green
  gates), depending on ALL implementing subtasks (Mode B).

## Research confirmations & corrections (post-fanout, all at df98a9a)

1. **Call sites**: 13 role Execute sites + cmd/models.go:150 (14 total); 8 ResolveRoleTimeout
   consumers. Full table with roles, resolution lines, and verbatim error branches in
   `call_sites_and_error_contract.md` §1. PRD's "13" = role sites; models.go handled separately.
2. **Error contract**: Execute returns BARE `context.DeadlineExceeded` on timeout today
   (executor.go:96); every downstream branch is `errors.Is(err, context.DeadlineExceeded)` — the
   idle sentinel (D2) must satisfy exactly this. StderrHint mechanism lives in executor.go:23–38
   (not parse.go); the only existing `DEBUG: cause` emitter is default_action.go:381.
3. **Git-key camelCase (spec-gap note)**: git config rejects underscores in key names; the
   existing doc comment (config.go ~L103) mandates CAMELCASE. Idle git keys are therefore
   `stagecoach.idleTimeout` and `stagecoach.role.<role>.idleTimeout`, matching spec §16.3's
   example (`idleTimeout = 60`). FR-R7's `stagecoach.role.<role>.idle_timeout` spelling is
   git-infeasible — implementers follow §16.3/camelCase. Spec text NOT edited (AGENTS.md rule 2);
   discrepancy recorded here.
4. **file.go seam**: the FILE layer's global timeout is parsed with bare `time.ParseDuration`
   (file.go:198–201), not `parseTimeout` — the idle twin must mirror that seam, not load.go's.
5. **Render mirror**: token insertion (after mode flag-set :148–159, before print_flag :161–162)
   must ALSO be applied in `RenderMultiTurn` (render.go:275–278).
6. **Flags are strings**: all timeout flags are STRING flags parsed only in loadFlags
   (silent-ignore malformed; env vars are hard errors) — idle flags follow the same discipline.
7. **Exit-code asymmetry (existing, keep)**: one-shot/workdesc timeout → ErrTimeout exit 124;
   multiturn turn-deadline → ErrRescue exit 3. FR-A5 tests must assert per-path existing
   behavior, not blanket 124.
8. **Stub harness gap**: cmd/stubagent today expresses only hung (SleepMS); slow-emitting and
   chatty-heartbeat stubs need new env knobs (recipes + wiring constraints specified in
   `call_sites_and_error_contract.md` §harness/recipes) — first subtask of P1.M3.
9. **Manifest precedent**: SessionMode = pointer field + validate + Resolve + regime-1 merge +
   render gate + dated "VERIFIED" comment (builtin.go:61) — EventStream mirrors it (slice,
   regime-2 merge); extractor registry = new sibling file, Registry stays pure data.
10. **Procfs**: pgrp = stat field 5; parse after LAST ')'; ENOENT = member gone; rchar = bytes
    from successful read-family calls (pipes/tty/sockets count, failed reads count nothing);
    reaped-child ioac folds into parent rchar (inflates sums — cannot false-negative activity);
    /proc/<pid>/io readable for same-uid children (PTRACE_MODE_READ_FSCREDS), no root needed;
    ~13ms per full sweep of ~1100 pids ⇒ ~2.6% duty at 2Hz. Details: `external_procfs_notes.md`.
11. **Docs drift**: 89 hits for 480s|120s — 67 MUST-CHANGE (15 user-facing docs/README + 52
    internal comments/help/tests), 22 unrelated; docs/providers.md table says "22 fields" → 23.
    Line-by-line inventory: `docs_drift_inventory.md`.
