# Delta PRD — v3.5: Activity-based generation deadlines + event-stream output

**Base spec:** `spec/SPEC.md` at v3.5 (revision block committed in `df98a9a`)
**Prior session:** `plan/021_086bbc57a2b2` — implemented v3.4 (`+body` format variants); both milestones Complete.
**Scope of this delta:** implement the v3.5 spec change in CODE. The spec text is already committed; only the implementation, tests, and user-facing docs lag.

---

## 0. What changed (diff analysis)

The v3.3→v3.4 delta was small; this one is **medium**: a new P0 section that reworks how every agent invocation is bounded, plus a new manifest capability. Core spec diff (commit `df98a9a`, spec files only: `01-product.md` +26/−5, `02-providers.md` +14/−1, `03-generation.md` +2/−2, `04-interface.md` +13/−8, `06-reliability.md` +1/−1, `SPEC.md` +3/−2):

**Added:**
- **§9.30 (P0, → G30)** — *Activity-based generation deadlines & event-stream output*: **FR-A1** (idle deadline — primary kill criterion: no observable byte activity from the agent's process group for `idle_timeout`, default 60s), **FR-A2** (hard cap retained, default raised 120s→600s, MUST NOT be removed — activity is not progress: codex ~27B/2s stderr heartbeat, ~1.1MB file re-reads, SDK retry loops), **FR-A3** (activity channels: stdout/stderr pipe writes; Linux-only procfs aggregate `rchar` of the process group — one idle clock reset by whichever moved last; where no channel exists, only the hard cap applies, degrading to exactly v3.4 behavior), **FR-A4** (both deadlines resolve per role, bind each invocation individually — fresh idle clock per multi-turn turn / work-desc round), **FR-A5** (idle kill is downstream-identical to a timeout: exit 124, ErrTimeout, rescue, no-retry; `DEBUG: cause` line names which deadline fired), **FR-S1** (manifest `event_stream` field switching the invocation to the provider's native progressive event-stream mode; message assembled from the terminal message event; stdout becomes a portable activity channel), **FR-S2** (verification duty — never declare speculatively; requires a dated empirical run proving progressive emission + terminal-event shape + error surfacing; known pitfalls: heartbeat-only streams (codex), sparse emitters (opencode ~3s gaps)), **FR-S3** (in-stream errors surface through the existing empty-output cause line; a failed response does NOT reliably produce a nonzero exit — pi documents this), **FR-S4** (phasing: Linux read-counter needs no manifest changes; event-mode migration is per-provider, gated on verification; no provider is required to migrate).
- **G30** (§6.1) — the goal mapping for §9.30.
- **FR-R7 finally DEFINED** (§9.15) — closes the v2.8 editorial gap where FR25/FR-T5/§16 referenced an "FR-R7" that §9.15 never defined. Each role resolves TWO deadlines on the same ladder as provider/model/reasoning: idle (`--<role>-idle-timeout` / `STAGECOACH_<ROLE>_IDLE_TIMEOUT` / `[role.<role>].idle_timeout` / `stagecoach.role.<role>.idle_timeout`, global fallback `--idle-timeout` / `STAGECOACH_IDLE_TIMEOUT` / `[defaults].idle_timeout` / `stagecoach.idle_timeout`, default 60s) and hard cap (`--<role>-timeout` / `STAGECOACH_<ROLE>_TIMEOUT` / `[role.<role>].timeout` / `stagecoach.role.<role>.timeout`, global fallback unchanged, default 600s). **No role ships a built-in deadline anymore.**
- §15.2 — NEW `--idle-timeout` flag row.
- §12.1 — manifest schema gains the `event_stream` field (default `[]`).

**Modified:**
- **FR25** (§9.5) — one timeout → two deadlines (idle + hard cap); on either kill, terminate the role's agent **process group** and enter rescue; the 120s elapsed-time-only kill is superseded.
- **FR35/FR36** (§9.8) — env/git-key lists gain `stagecoach_IDLE_TIMEOUT` and per-role `stagecoach.role.<role>.{timeout,idle_timeout}`.
- **FR-T5** (§9.24) — per-turn deadlines (idle + hard cap, fresh idle clock per turn); budget = `message hard-cap × (N+1)`, progress line says "worst case". **FR-T7** — "deadline breach" replaces "timeout".
- §15.2 `--timeout` row (default 600s, "hard cap"); §15.4 exit 124 = "Deadline exceeded (idle or hard cap)"; §16.1 defaults; §16.2 config example (`idle_timeout` + `timeout = "600s"`); §16.3 git keys (`timeout = 600`, `idleTimeout = 60`); §16.4 retitle + deadline resolution + per-role example; §18.2 failure row ("Generation deadline (idle / hard cap) … kill agent process group"); §13.5 / §12.5.1.1 item 5 / §14 wording.
- §10.2 — bookkeeping: `+body` graduation recorded (v3.4, ALREADY IMPLEMENTED — no work).

**Removed:**
- The v2.8 planner 480s built-in wall clock (retired as moot under idle + 600s).
- The 120s global default.

**Not part of this delta:** `docs/packaging.md` (AUR runbook) rode in the same commit — docs-only, already landed, no implementation work.

**Measured basis baked into the spec** (maintainer, 2026-10-11/12 — treat as facts, do not re-verify): print-mode CLIs (`pi -p`, `opencode run`) emit ZERO stdout bytes on healthy 18–19s runs; the process group's `rchar` moves ~16KB/0.5s while streaming and is exactly flat on a blocked socket; worst legitimate silence (300KB/~75K-token prompt upload + prefill) is ~13–14s, so 60s carries ~4× margin.

---

## 1. Implementation surface (verified against current code at `df98a9a`)

| File | Current state | Change |
| --- | --- | --- |
| `internal/provider/executor.go` | `Execute(ctx, spec, timeout, vb)`; single `context.WithTimeout`; stdout/stderr to plain `bytes.Buffer`s; kill via `cmd.Cancel` SIGTERM-to-group + `WaitDelay` 3s (procgroup_unix.go); returns `context.DeadlineExceeded` on timeout | Take a `Deadlines{Idle, Hard}` struct; arm a resettable idle clock fed by two activity channels; idle kill returns a DeadlineExceeded-matching error; verbose cause line |
| `internal/provider/procgroup_unix.go` | `Setpgid` ⇒ PGID==PID (group membership = `pgrp == agent pid` — the spec's FR-A3(b) precondition already holds) | unchanged |
| *(new)* `internal/provider/activity_linux.go` + `activity_other.go` | — | Linux procfs poller: sum `rchar` over `/proc/[0-9]*/stat` entries with `pgrp == agentPID`, ~0.5s tick; non-Linux stub reports "no channel" |
| `internal/provider/manifest.go` / `merge.go` / `render.go` | `SessionMode` precedent for optional capability fields; `Render` composes args | `EventStream []string toml:"event_stream"` (default nil); field-merge; append tokens after the mode flag-set, before `print_flag`; CmdSpec gains `ProgressiveStdout bool` |
| `internal/config/config.go` | `Timeout` field + default `120s` (L224); `RoleConfig.Timeout` | `IdleTimeout` field + default 60s; `Timeout` default → 600s; `RoleConfig.IdleTimeout`; doc comments |
| `internal/config/roles.go` | `defaultRoleTimeouts{planner:480s}` + `ResolveRoleTimeout` | DELETE the built-in map; `ResolveRoleDeadlines(role, cfg) (idle, hard)` on the same ladder (0 ⇒ inherit global) |
| `internal/config/load.go` | `STAGECOACH_TIMEOUT` (L272), `--timeout` flag, per-role `STAGECOACH_<ROLE>_TIMEOUT`, `parseTimeout` (L674) | Add `STAGECOACH_IDLE_TIMEOUT`, `--idle-timeout`, `STAGECOACH_<ROLE>_IDLE_TIMEOUT`; reuse `parseTimeout` |
| `internal/config/git.go` | `stagecoach.timeout` (L148), per-role `stagecoach.role.<role>.timeout` | Add `stagecoach.idle_timeout`, `stagecoach.role.<role>.idle_timeout` |
| `internal/cmd/root.go` | `--timeout` (L169, help says 120s); `--<role>-timeout` ×4 (L259–266) | Update help (600s, "hard cap"); add `--idle-timeout` + `--<role>-idle-timeout` ×4 |
| `internal/config/bootstrap.go` | template `# timeout = "120s"` (L174); env/git-key doc comments (L359, L375) | Template gains `idle_timeout` + 600s; doc comments updated |
| Execute call sites (13) | `generate.go` L374, `multiturn.go` L172/183/194, `workdesc.go` L82/131/149, `decompose/{planner,stager,message,arbiter}.go`, `hook/exec.go` L187 — each resolves one role timeout today | Resolve `Deadlines` per role and pass both; multiturn budget line (generate.go L470–474) ×hard-cap + "(worst case)" |
| `internal/cmd/models.go` | runs `list_models_command` via Execute with a plain bound | NOT a role call — keeps a single hard bound, idle unarmed (note in code) |
| Docs | `docs/cli.md` L27, L60–63, L452; `docs/configuration.md` L91, L99, L138, L193, L212–215, L232, L240–241, L254; `docs/how-it-works.md` L173, L245, L346; `README.md` L252, L310; `docs/providers.md` manifest-field table (L29 area, `session_mode` precedent) | Mode A with the work; Mode B final sweep (Phase P3) |

**Prior research:** `plan/021_086bbc57a2b2/architecture/` covers only the v3.4 format subsystem — not applicable here. No existing research on the executor/deadlines; the anchors above were verified fresh.

---

## 2. Design decisions (locked)

1. **`Execute` takes a deadlines struct, not a duration.** `provider.Deadlines{Idle, Hard time.Duration}` replaces the `timeout` parameter; all 13 role call sites updated mechanically. `Idle ≤ 0` ⇒ idle not armed. The hard cap keeps the existing `context.WithTimeout` shadow.
2. **Idle kill preserves the error contract exactly (FR-A5).** The idle path cancels via a dedicated `context.WithCancel` and returns an error that satisfies `errors.Is(err, context.DeadlineExceeded)` — an `ErrIdleDeadline` sentinel with an `Is()` method, or equivalent — so every existing `errors.Is(execErr, context.DeadlineExceeded)` branch (generate.go L376, workdesc L340, root exit-124 mapping, no-retry-on-timeout, FR-T7 abort) works **unchanged**. Only the verbose line differs.
3. **Activity channels.** (a) *Pipe*: `cmd.Stdout`/`Stderr` become a small counting writer (mutex-guarded buffer + atomic last-write timestamp) — buffer growth while `Wait` runs is the signal; no data races on `bytes.Buffer`. (b) *Read counter* (Linux): poll `/proc/[0-9]*/stat` for `pgrp == agentPID`, sum `rchar` from each member's `/proc/<pid>/io`, ~0.5s tick (the group-leader fact is already guaranteed by `Setpgid`). One watcher goroutine resets a single idle deadline from whichever channel moved last; it also observes `ctx.Done()` (hard cap / parent cancel) and exits.
4. **Arming rule (FR-A3's degradation clause, made mechanical).** The idle deadline is armed iff `Idle > 0` AND (`runtime.GOOS == "linux"` OR `spec.ProgressiveStdout`). `CmdSpec.ProgressiveStdout` is set by `Render` when the manifest declares a non-empty `event_stream` — the pipe channel is only trustworthy for a provider that emits progressively. Non-Linux + print-mode provider ⇒ idle disarmed ⇒ exactly v3.4 behavior (hard cap only). Stagecoach never fabricates activity.
5. **Defaults + retirement.** `Config.Timeout` 120s→600s; new `Config.IdleTimeout` 60s; `defaultRoleTimeouts` (planner 480s) is DELETED — `ResolveRoleDeadlines` returns per-role-override-or-global for every role. `ResolveRoleTimeout` may remain as a thin hard-cap accessor or be replaced; either way no built-in role deadline exists.
6. **Verbose cause line.** New `ui.Verbose` helper (e.g. `VerboseCause(reason)`) emitting `DEBUG: cause: idle deadline (60s without observable activity)` / `DEBUG: cause: hard cap (600s)`, called by `Execute` on a deadline kill. Existing verbose surfaces unchanged.
7. **Multiturn (FR-T5).** Each turn's `Execute` gets the message role's resolved `Deadlines` — the fresh idle clock per turn is automatic (one Execute per turn). The progress line at generate.go L470–474 multiplies the **hard cap** and appends `(worst case)`.
8. **`event_stream` is data; extraction is code (FR-S1/S2).** The manifest field carries the flag tokens (rendered after the mode flag-set, before `print_flag`); assembling the final message from the terminal message event is a **per-provider extractor registered in Go alongside the built-in manifests** (a small `eventstream` parse helper; per-provider terminal-event shape recorded at verification time). A manifest (built-in or user) declaring `event_stream` for a provider with no registered extractor is a hard Render-time configuration error. The field is populated ONLY for providers that pass FR-S2 verification — shipping `[]` everywhere is spec-compliant (FR-S4) and is the fallback when a CLI can't be verified.
9. **FR-S3 error surfacing.** In event mode, when the stream carries an error/abort event and no terminal message event, the in-stream error text feeds the EXISTING `StderrHint` empty-output cause path (or a sibling that takes precedence when event mode produced no message) — replacing the opaque empty-stdout parse failure.
10. **No scope creep.** Exit codes, rescue, CAS, lock, index, freeze semantics all untouched. `docs/packaging.md` is out of scope. Spec §20 gains no new must-cover scenarios; this delta's tests extend §20.1 layers 1–3 naturally.

---

## 3. Backlog

### Phase P1 — Activity-based deadlines (FR-A1–A5, FR-R7, FR25, → G30)

The core rework: config surface, idle-detection engine, per-invocation binding across all 13 call sites. No commit/CAS/rescue/lock changes.

#### Milestone P1.M1 — Config surface: two deadlines, five precedence layers, planner 480s retired

**Task P1.M1.T1 — `idle_timeout` config plumbing end-to-end + flag/help/template text**

- **Subtask P1.M1.T1.S1 — config core + role resolution** (`internal/config/config.go`, `internal/config/roles.go`)
  Add `Config.IdleTimeout` (+ default 60s) and `RoleConfig.IdleTimeout` (0 ⇒ inherit); change `Config.Timeout` default to 600s; DELETE `defaultRoleTimeouts` and its planner branch; add `ResolveRoleDeadlines(role, cfg) (idle, hard time.Duration)` mirroring the existing ladder (per-role override > global; doc comments updated to the two-deadline contract, noting no role ships a built-in). Update `config.go` field doc comments (idle = primary, hard cap = backstop, §9.30).
- **Subtask P1.M1.T1.S2 — precedence layers** (`internal/config/load.go`, `internal/config/git.go`)
  Add `STAGECOACH_IDLE_TIMEOUT` + `--idle-timeout` flag read (string, zero-default, `parseTimeout` reused — identical discipline to `STAGECOACH_TIMEOUT`), `stagecoach.idle_timeout` git key, and per-role `STAGECOACH_<ROLE>_IDLE_TIMEOUT` + `stagecoach.role.<role>.idle_timeout` (mirroring the per-role timeout seams at load.go L317+, git.go per-role block). Errors wrapped in the existing per-layer style.
- **Subtask P1.M1.T1.S3 — CLI flags + template + doc comments** (`internal/cmd/root.go`, `internal/config/bootstrap.go`)
  `--timeout` help → default 600s, "hard cap"; add `--idle-timeout` global flag + `--{planner,stager,message,arbiter}-idle-timeout` ×4 (same string form as the timeout flags); bootstrap template line → `# idle_timeout = "60s"` + `# timeout = "600s"` with one-line comments; bootstrap env/git doc comments (L359/L375) gain the idle variants.
  **Docs (Mode A, ride here):** `docs/cli.md` flag table (L27 retitle + new `--idle-timeout` row + per-role idle rows after L60–63); `docs/configuration.md` defaults table (L138), config-file example (L91), env table (L193 + per-role L212–215), git-config example/table/notes (L232, L240–241, L254).
  **Tests riding:** `config_test.go`/`roles_test.go`/`load_test.go` — new-default assertions (600s/60s), planner-480s retirement, idle precedence per layer (flag > env > git > file > global), zero-value inheritance.

#### Milestone P1.M2 — Executor idle engine + call-site threading

**Task P1.M2.T1 — activity channels, two-deadline Execute, 13 call sites, multiturn line**

- **Subtask P1.M2.T1.S1 — activity infrastructure** (`internal/provider/activity.go` + `activity_linux.go` + `activity_other.go`, new)
  Counting writer (FR-A3(a)): wraps a buffer, exposes an atomic last-write monotonic timestamp, satisfies `io.Writer`. Linux poller (FR-A3(b)): `groupReadBytes(pgid int) (uint64, bool)` summing `rchar` across `/proc/[0-9]*/stat` members with `pgrp == pgid`; non-Linux stub returns `(_, false)`. Unit tests for the poller against the test's own process group (spawn a child that reads known bytes; assert the counter moves; a `GOOS`-gated build ensures the stub compiles elsewhere).
- **Subtask P1.M2.T1.S2 — Execute rework** (`internal/provider/executor.go`, `internal/ui/verbose.go`)
  Signature → `Execute(ctx, spec, dl Deadlines, vb)`; hard cap via existing WithTimeout; idle watcher goroutine (tick ~0.5s) per the arming rule (decision D4 — needs `spec.ProgressiveStdout`, added to CmdSpec here, set in Render under P2.M1; until then Render leaves it false and non-Linux idle is disarmed, Linux arms via the poller); idle kill cancels the group exactly as the hard cap does and returns the DeadlineExceeded-matching error (D2); `VerboseCause` helper (D6) emits the cause line. Extend the Execute doc comment (error contract now: idle OR hard cap ⇒ DeadlineExceeded-match). `internal/cmd/models.go` call site passes `Deadlines{Idle: 0, Hard: <existing bound>}` with a comment (not a role call).
- **Subtask P1.M2.T1.S3 — thread the 13 role call sites + multiturn budget line** (`internal/generate/generate.go`, `multiturn.go`, `workdesc.go`; `internal/decompose/{planner,stager,message,arbiter}.go`; `internal/hook/exec.go`)
  Each site: `msgTimeout := config.ResolveRoleTimeout(...)` → `dl := config.ResolveRoleDeadlines(role, cfg)` passed to Execute. generate.go L470–474: multiply the hard cap, append `(worst case)`. Hook exec (FR-V7 parity) and workdesc/multiturn (FR-A4 fresh clock) need only the new call shape. Update the stale FR-R7 comments at the former `ResolveRoleTimeout` sites to name both deadlines.
  **Docs (Mode A, ride here):** `docs/how-it-works.md` L346 (per-turn deadlines prose); `docs/cli.md` L452 exit-124 row ("Deadline exceeded (idle or hard cap)").
  **Tests riding:** `executor_test.go` — hard-cap test updated for the new signature; new: idle kill (silent stub, tiny idle), healthy-slow NOT killed (stub emitting a byte every ~0.5s for ~3× idle with `ProgressiveStdout` forced true), Linux read-counter keeps a stdout-silent stdin-reader alive past idle (build-tagged).

#### Milestone P1.M3 — Integration + regression parity

**Task P1.M3.T1 — deadline behavior through the real pipeline**

Stub-agent integration tests over `generate.CommitStaged` (and one decompose case): (a) hung agent → idle kill → exit-124 semantics (ErrTimeout rescue, tree SHA printed, no retry — FR-A5 parity with the old timeout path); (b) slow-but-emititng agent (progressive stub) succeeds where v3.4's 120s clock would have killed it — the motivating false-kill regression; (c) hard cap still fires on a chatty-forever stub (heartbeat pathology, FR-A2); (d) `--verbose` output contains the `DEBUG: cause:` line naming idle vs hard cap; (e) config: `--idle-timeout`/`[role.message].idle_timeout`/env override each observable in behavior. Assert no lock/index/HEAD mutation on any deadline path (existing invariants cover this — one explicit assertion).

### Phase P2 — Event-stream output mode (FR-S1–S4)

#### Milestone P2.M1 — Manifest field + event-mode execution path

**Task P2.M1.T1 — `event_stream` manifest field, render, terminal-event assembly, error surfacing**

- **Subtask P2.M1.T1.S1 — manifest + render** (`internal/provider/manifest.go`, `merge.go`, `render.go`, `builtin_test.go`, `render_test.go`)
  `EventStream []string toml:"event_stream"` (nil ⇒ `[]`), field-merged across layers like every manifest field; `Render` appends the tokens after the bare/tooled flag-set, before `print_flag`, and sets `CmdSpec.ProgressiveStdout = len(m.EventStream) > 0`; no built-in manifest declares it yet (all ship `[]` — FR-S2: never speculative). Render-time error if `event_stream` is set for a provider with no registered extractor (D8).
- **Subtask P2.M1.T1.S2 — event-mode execution** (`internal/provider/executor.go` or new `eventstream.go`, `parse.go`)
  When `spec.ProgressiveStdout`: stdout is parsed as a sequence of JSON events (all five native modes are NDJSON-ish); the final message is assembled from the **terminal message event** via the per-provider extractor registry (D8); an error/abort event with no message event surfaces its text through the StderrHint empty-output cause path (D9/FR-S3). Stub tests: a fake JSONL agent emitting progressive events + terminal message → assembled message, NOT idle-killed with a tiny idle on every platform (portable pipe activity — the FR-A3 payoff); error-event stream → surfaced cause, empty message handled by the existing parse-retry path.
  **Docs (Mode A, ride here):** `docs/providers.md` manifest-field table gains the `event_stream` row + a `session_mode`-style capability section (verification bar, json-vs-stream-json trap, heartbeat/sparse pitfalls verbatim from FR-S2).

#### Milestone P2.M2 — Per-provider verification + population (FR-S2)

**Task P2.M2.T1 — verify each installed agent CLI's event mode; populate only what passes**

For each of pi (`--mode json`), claude/cursor/agy (`--output-format stream-json` + partial-message switches), codex (`--json`), opencode (`--format json`) — where the CLI is installed/authenticated on this machine: run a dated empirical generation proving (i) progressive stdout emission DURING generation, (ii) the terminal-event shape (record it as the provider's extractor), (iii) how in-stream errors surface. Record findings + date in each manifest's header comment (the FR-T9/CHROME-DISABLE precedent) and populate `event_stream` + the extractor only for passing providers. The spec's own recorded pitfalls are hypotheses to confirm, not assumptions: codex's heartbeat stream is liveness-not-progress (read counter stays the honest signal even in event mode — its event_stream may still qualify for FR-S3 error surfacing IF emission is otherwise progressive; judgment recorded with the date); opencode's ~3s gaps are fine for a 60s idle clock. A provider that cannot be verified (absent, unauthenticated, non-progressive) ships `[]` — fully supported in print mode (FR-S4); nothing is ever declared speculatively. If NO provider passes, the P2.M1 machinery stays stub-tested and all built-ins ship `[]` — spec-compliant.

### Phase P3 — Sync changeset-level documentation (Mode B)

**Task P3.T1 — final coherence sweep across the v3.5 delta**

Depends on all prior tasks. `README.md` L252 (config example: idle_timeout + 600s) and L310 (exit-124 blurb: "deadline (idle or hard cap)"); `docs/how-it-works.md` L173 (per-role deadline prose — the "planner defaults to 480s" claim MUST NOT survive anywhere) and L245 (failure-table row); cross-file grep sweep: `rg -n "480s|\"120s\"|default 120s|120s default" README.md docs/ internal/` returns only intentional hits (legacy `max_md_lines`-class values are unrelated; the flag help, template, env tables, and defaults tables all state idle 60s / hard cap 600s); `go test ./...` green, `go vet ./...` clean; every user-facing surface that names a generation bound names BOTH deadlines.

---

## 4. Out of scope (explicit)

- No commit/CAS/rescue/lock/index/freeze changes (FR-A5: downstream invariants unchanged).
- Exit-code table semantics (124 stays 124; only wording).
- `docs/packaging.md` AUR runbook (already landed with the spec commit).
- macOS/Windows idle detection for print-mode providers — spec-mandated degradation to hard-cap-only (FR-A3); the portability path is event-stream migration (P2), not a procfs port.
- §20.5 harness scenario additions beyond the riding tests (the v3.5 spec added none).
