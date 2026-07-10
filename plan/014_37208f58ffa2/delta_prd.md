# Delta PRD — v2.7: Orphaned-Run Lock Reclamation + Config-Write Hardening

**Delta from:** session 013 (v2.6 spec) → **Delta to:** v2.7 spec
**Scope:** two reliability-hardening themes added in the v2.7 revision, both bug/incident-motivated. No new agent, no new commit-path feature, no change to the snapshot/CAS/rescue core.

## What actually changed (diff analysis)

The v2.7 revision adds **two themes**. Both are real but bounded — neither is a headline product feature on the scale of decomposition or work-description mode.

### Theme 1 — Orphaned-run lock reclamation (the headline v2.7 addition)
A third lock state the §18.5 model did not contemplate: a holder whose **launcher closed without killing it** (closing the lazygit TUI, quitting an IDE, a detaching terminal). The child is reparented to init and keeps running — its `pid` stays alive (so pid-liveness reaping never fires) and §18.4's `SIGINT`/`SIGTERM`-only handler receives neither, so the lock file outlives the launcher indefinitely. The fix is **self-termination, never contender-side force-breaking**.

- **NEW §9.27** with **FR-K1–K7** (→ new **G24**):
  - FR-K1 parent-death self-watchdog (records parent pid, routes through existing rescue+lock-release exit on parent death)
  - FR-K2 detection by parent-pid change (reparenting), not `getppid()==1`; Linux `prctl(PR_SET_PDEATHSIG)` fast path + `getppid()` polling fallback
  - FR-K3 `SIGHUP` joins caught signals → rescue path (terminal-hangup complement to the watchdog's detach case)
  - FR-K4 new read-only `stagecoach lock status` subcommand
  - FR-K5 Busy message reformatted (lock path on own line) + orphan flag
  - FR-K6 `no_parent_watchdog` opt-out (env/git-config) for intentional detachment (`nohup`/`setsid`/`systemd-run`)
  - FR-K7 Unix-only watchdog; Windows unchanged (`flock` already a no-op there)
- **AMENDED FR52** (§9.9) — adds the launcher-closed case to the lock description + cross-refs §9.27/§18.5.
- **AMENDED §18.4** (signal handling) — extended from `{SIGINT, SIGTERM}` to `{SIGINT, SIGTERM, SIGHUP}` (Unix); new SIGHUP paragraph.
- **AMENDED §18.5** (concurrency/lock) — new "Orphaned-but-alive" paragraph; contention (Busy) message gains an orphan hint + lock path on its own line.
- **NEW** `stagecoach lock status` subcommand in §15.3; **NEW** `noParentWatchdog` git-config key in §16.3; **NEW** e2e scenario in §20.5.

### Theme 2 — Config-write hardening (incident-motivated; FR-B8, FR-B9)
Surfaced by the "upgrade-clobber incident": `config upgrade` on an already-inert (all-commented) file left it all-commented and appended a stray `config_version` line while reporting success. Root cause: an inert file's commented `config_version` was mis-classified as "legacy," the load notice nudged the user to `config upgrade`, and upgrade clobbered. Two new FRs close it.

- **NEW FR-B8** — every config-writing command (`init`, `init --force`/`--template`, `upgrade`, install/first-run bootstrap) preserves every active (uncommented) `key = value` grouped by its `[table]` heading + writes a timestamped backup. Config analogue of FR-H2's never-clobber rule.
- **NEW FR-B9** — load-time migration notice must NOT fire on inert files (zero active settings); `config upgrade` on an inert file is a no-op reported as such.
- **AMENDED FR-B2/B4/B5** — `config init --force` now preserves active settings (per FR-B8); `config_version` bumped only on a genuinely breaking change (additive changes never advisory); load-time advisory points only at `config upgrade` (never `config init --force`); `config upgrade` is the sole non-destructive schema migration and must round-trip every active line.
- Appendix E item 13 (config upgrade mechanics) marked **Resolved**; Appendix F gains FR-B8/B9 decision-log entries.

### Sizing note
Theme 1 is the bulk (~7 FRs, 2 packages touched heavily + a new subcommand + a config key + a signal change). Theme 2 is smaller (2 new FRs + 3 amendments, confined to config-write paths). Both are medium and bounded. **Two phases**, each focused. No commit/CAS/rescue/index behavior changes in either theme.

---

## Current codebase state (verified by research)

The lock + signal + config-write subsystems already exist and are the targets. Nothing here is greenfield.

| Area | File(s) | Current state | Delta target |
|------|---------|---------------|--------------|
| Lock core | `internal/lock/lock.go` | `Acquire`/`Release`/`SetSnapshot`/`ReleaseCurrent`/`reapStaleLocks`; pid-liveness reaping; no watchdog, no orphan detection | Add parent-death watchdog (FR-K1/K2), orphan-status read (FR-K4) |
| Lock platform | `internal/lock/lock_unix.go`, `lock_windows.go` | `flock`/`processAlive` (Unix); no-op stub (Windows) | Add prctl/getppid detection (Unix, FR-K2/K7); Windows no-op |
| Signal | `internal/signal/signal.go`, `signal_unix.go`, `signal_windows.go` | Catches `{SIGINT, SIGTERM}`; has `OnRescueExit` seam + `RestoreDefault` | Add `SIGHUP` (FR-K3); watchdog arming seam |
| main wiring | `cmd/stagecoach/main.go` | `signal.Install(... OnRescueExit: lock.ReleaseCurrent ...)` | Arm watchdog after lock Acquire; pass `no_parent_watchdog` |
| Busy message | `internal/cmd/default_action.go` `handleLockContention` | Inline `Lock: %s.` mid-sentence | Lock path on own line + orphan hint (FR-K5) |
| Config writes | `internal/cmd/config.go` (`runConfigInit`/`writeBootstrapFile`/`runConfigUpgrade`), `internal/config/bootstrap.go` (`bootstrapWriteConfig`), `internal/config/migrate.go` | Fresh-content writes; surgical upgrade; migrationNotice fires unconditionally | Preserve active settings + backup (FR-B8); suppress notice on inert (FR-B9) |
| e2e | `internal/e2e/lock_scenarios_test.go` | `TestE2ELockContention` only | Add orphan/SIGHUP/watchdog scenarios (§20.5) |

**Build + tests pass today** (`go build ./...`, `go test ./...`). The prior session (013) was the provider-lineup correction; its research (`plan/013_b8a415cc6e79/architecture/`) is provider-focused and does **not** cover these lock/config areas — fresh research is needed here, but the subsystem entry points above are already located.

---

# PHASE 1 — Orphaned-Run Lock Reclamation (§9.27, FR-K1–K7, → G24)

**Description:** Close the "lock stays forever" report that bites stagecoach's primary launch path (lazygit `<c-a>`, IDE, detaching terminal): when a holder's launcher closes without killing it, a parent-death watchdog self-exits the run through the existing rescue+lock-release path; `SIGHUP` joins the caught signals; a read-only `stagecoach lock status` surface lets a blocked user decide to `kill`/`rm` themselves. Self-termination only — FR52's "never force-break" guarantee is preserved unchanged (the guarantee is that _another_ process never breaks a live lock; the watchdog is the _same_ process abandoning its own unwanted work).

**Milestone P1.M1 — Core self-termination mechanism (watchdog + SIGHUP + opt-out)**

The parent-death watchdog + SIGHUP handling + the `no_parent_watchdog` opt-out. This milestone delivers the headline fix (FR-K1, K2, K3, K6, K7). It is Unix-only; Windows is a no-op (FR-K7).

Task **P1.M1.T1 — Parent-death self-watchdog + SIGHUP + opt-out**
- Subtask **P1.M1.T1.S1** — Add the parent-death watchdog to the lock package (FR-K1, K2). On `Acquire` (or via an explicit arming seam called from main after Acquire), record the current `os.Getppid()` as the startup parent pid and start a watchdog. Detection is by **parent-pid change** (reparenting), never the brittle `getppid()==1` test. Implement `prctl(PR_SET_PDEATHSIG, SIGTERM)` as the fast kernel-delivered path on Linux (set as early as possible; set on a locked OS thread because `prctl` is per-thread and the Go runtime migrates goroutines; treat as best-effort). On Darwin (no `prctl`) and as the Linux fallback, poll `os.Getppid()` at a bounded interval (default ~1s) and trigger when the value differs from the captured startup pid. On parent death, route through the _existing_ exit path: cancel the generation context, run the rescue recipe if a snapshot is armed, release the lock file via the same `ReleaseCurrent`/`OnRescueExit` seam the signal handler uses, and exit. Reuse `internal/signal`'s `Active()`/`RestoreDefault` machinery so the watchdog no-ops past `RestoreDefault` (the `update-ref` window, where abandonment would lose committed work). The watchdog is a new goroutine owned by the lock package (or a tiny new `internal/watchdog` leaf); it must NOT import `internal/signal`'s private state — it calls the same nil-safe package wrappers (`signal.Active()`, `RestoreDefault`, the rescue path) or an injected seam. NO contender-side logic — the watchdog only ever self-exits.
- Subtask **P1.M1.T1.S2** — Add `SIGHUP` to the caught signals (FR-K3). Extend `internal/signal/signal.go` `Install`'s `signal.Notify` set from `{os.Interrupt, syscall.SIGTERM}` to `{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}` on Unix (`signal_unix.go` provides the platform-specific signal set; Windows omits SIGHUP). On receipt, take the same rescue path as SIGINT/SIGTERM (forward to child group, cancel ctx, rescue-or-exit, `OnRescueExit` lock release). Add `SIGHUP` to `exitCodeForSignal` (→ 128+1 = 129). `RestoreDefault` must disarm all three for the `update-ref` window. `SIGHUP` covers the terminal-hangup case where the signal _is_ delivered; the watchdog (S1) covers the detach/orphan case where no signal is delivered at all. (§18.4)
- Subtask **P1.M1.T1.S3** — Add the `no_parent_watchdog` opt-out (FR-K6) and wire the watchdog in `main.go`. New config surface: env `stagecoach_NO_PARENT_WATCHDOG=1` (FR35 convention), git-config `stagecoach.no_parent_watchdog` (FR36), default **off** (watchdog runs by default). Follow the `no_verify`/`no_color` opt-out convention. When set, the watchdog from S1 is not armed — `SIGHUP` handling (S2) and `lock status` (P1.M2) are independent and always on. Wire in `cmd/stagecoach/main.go`: after the default action acquires the lock, arm the watchdog unless the opt-out resolves true. Reuse the existing config-precedence resolution (the default action already reads config; pass the resolved bool to the arming seam). Document the new env var in §9.8 FR35's env-var list mentally (no separate task — it rides with the config key).
  - *Mode A docs (ride with this work):* `docs/how-it-works.md` "Safety and the rescue protocol → Per-repo run lock (FR52)" subsection (currently lines ~166–179) — extend the "Auto-release + file reaping" paragraph to cover the orphaned-but-alive case, the parent-death watchdog, and SIGHUP. This is the architecture-overview doc that already describes the lock model in detail.

**Milestone P1.M2 — Diagnostic surface (`lock status` + Busy orphan flag)**

The read-only diagnostic + the reformatted contention message. Lets a blocked user see the holder's path/liveness/orphan-status and decide to act themselves (FR-K4, K5). Never auto-breaks.

Task **P1.M2.T1 — `stagecoach lock status` + Busy message orphan flag**
- Subtask **P1.M2.T1.S1** — Add `lock status` subcommand (FR-K4). New command `internal/cmd/lock.go` registered on rootCmd (parallel to `providers`/`hook`/`integrate`). It is read-only and works outside a commit path (add to `shouldSkipConfigLoad` like the other diagnostic subcommands). For the current repo's lock it prints: the lock path; the holder's parsed `pid`/`hostname`/`repo`/`timestamp`/`snapshot` (reuse `lock.parseContents` — expose a read path, or read the file directly via the lock dir/hash resolver); whether the holder process is alive (`kill(pid, 0)` via the platform `processAlive`); and (Unix) whether it **appears orphaned** (its own parent pid is init/a subreaper — i.e. `os.FindProcess(pid).Signal(0)`-alive AND `getppid-of(pid)` indicates reparenting; on Windows report "unknown"). With no lock held, print `"no run lock for <repo>"`. Changes nothing. Expose the orphan-status helper (it is also consumed by S2's Busy message). (§15.3)
- Subtask **P1.M2.T1.S2** — Reformat the Busy contention message + orphan flag (FR-K5). In `internal/cmd/default_action.go` `handleLockContention`, put the lock path on its own line (so a script/hurried user can copy it directly), and when the holder appears orphaned (the S1 orphan-status helper) add a line saying the holder's launcher has exited and the user may safely `kill <pid>` or `rm <path>`. Keep the existing fallback diagnostics for a partial/empty lock file (the "Issue 4b" guard) and the no-op fast path unchanged. (§18.5 contention behavior)
  - *Mode A docs (ride with this work):* `docs/cli.md` "## Subcommands" section — add a `### lock status` entry parallel to the existing `hook status`/`providers` entries (path, holder pid/host/repo/timestamp/snapshot, alive, orphan status; "changes nothing"). This is the user-facing CLI reference for the new subcommand.

**Milestone P1.M3 — Tests + cross-cutting verification**

Task **P1.M3.T1 — Lock-reclamation e2e + unit tests**
- Subtask **P1.M3.T1.S1** — Add the §20.5 orphaned-run-lock-reclamation e2e scenarios. In `internal/e2e/lock_scenarios_test.go` (extend the existing `TestE2ELockContention` harness), add: (a) launch stagecoach (or a stub holder) from a short-lived parent that exits mid-generation and assert the holder self-exits via the parent-death watchdog, the lock file is removed, and HEAD + index are unchanged (no commit landed); (b) drive a launcher that delivers `SIGHUP` on close and assert the rescue path + lock release fire; (c) assert `lock status` reports holder-liveness and orphan-status correctly for a live holder, a dead holder, and a reparented (orphaned) holder; (d) assert the `no_parent_watchdog` opt-out suppresses the watchdog without affecting SIGHUP or `lock status` (FR-K6). Use the existing stub-agent harness where a real agent isn't feasible; the watchdog trigger is deterministic (parent pid changes), not timing-fragile. (§20.5)
- Subtask **P1.M3.T1.S2** — Unit tests for the new lock/signal seams. Parent-pid-change detection (FR-K2) under a controllable clock/ppid injection; `SIGHUP` routing through the rescue path in `internal/signal` (extend the existing in-process `handle(sig)` test seam); `lock status` output formatting (golden) for live/dead/orphaned/no-lock; Busy message reformat (own-line lock path + orphan hint). Build + `go test ./internal/lock/... ./internal/signal/... ./internal/cmd/... ./internal/e2e/...` must pass.

---

# PHASE 2 — Config-Write Hardening (FR-B8, FR-B9; amended FR-B2/B4/B5)

**Description:** Close the upgrade-clobber incident: every config-writing command preserves active (uncommented) settings + writes a timestamped backup (FR-B8); the load-time migration notice is suppressed on inert files and `config upgrade` is an honest no-op on them (FR-B9); the load-time advisory points only at `config upgrade` (never `config init --force`). Confined to the config-write paths and the load-time notice. No commit/CAS/rescue/lock changes.

**Milestone P2.M1 — Active-setting preservation + backup on all write paths (FR-B8)**

Task **P2.M1.T1 — Preserve active settings + timestamped backup across every config write**
- Subtask **P2.M1.T1.S1** — Implement a shared "preserve active settings" write helper (FR-B8). A new helper (in `internal/config` or `internal/cmd/config.go`) that, before any config write, parses the existing file's active (uncommented) `key = value` lines grouped by their `[table]` heading (top-level keys form their own implicit group), and carries those values verbatim into the written file under their headings. An active setting whose section the new template lacks is appended in its own `[section]` block. A key the new schema has removed is **commented out with a note** (as FR-B5 already requires for `config upgrade`). Always writes a timestamped backup `<file>.stagecoach-backup.<unix-ts>` of the prior file first (mirror the FR-I3 no-mangle backup protocol). Wire this into **every** write path: `config init` (no-force, when file exists → already refuses; force path), `config init --force`, `config init --template`, `config upgrade`, and the `bootstrapWriteConfig` first-run fallback (FR-B3). The plain `config init` (file missing) path writes fresh content with no prior settings to preserve (backup is a no-op / skipped when no prior file). (§9.17 FR-B8)
- Subtask **P2.M1.T1.S2** — Make `config init --force` preserve active settings (amended FR-B2). `runConfigInit`'s `--force` path currently overwrites wholesale from `GenerateBootstrapConfig`/`exampleConfigTemplate`. Change it to route through the S1 preserve-active-settings helper: regenerate the template structure but carry every active setting the user already had — only the surrounding template/comments are refreshed. It is **not** upgrade remediation (point users at `config upgrade` for that, FR-B5). (§9.17 FR-B2; §15.3 `config init` entry)
  - *Mode A docs (ride with this work):* `docs/cli.md` `### config init` and `### config upgrade` entries — update the `--force` description ("overwrites" → "regenerates the template structure while preserving existing active settings (FR-B8)") and the `config upgrade` description (sole non-destructive schema migration; round-trips active settings). `docs/configuration.md` if it describes `config init`/`upgrade` behavior.

**Milestone P2.M2 — Inert-file notice suppression (FR-B9)**

Task **P2.M2.T1 — Suppress the legacy alarm on inert files; honest no-op upgrade**
- Subtask **P2.M2.T1.S1** — Suppress the load-time migration notice on inert files (FR-B9). In `internal/config/load.go` / `migrate.go`, gate the `migrationNotice` so it fires ONLY when the file has at least one active (uncommented) setting AND its declared (uncommented) `config_version` is older than `CurrentConfigVersion` (or an active file genuinely missing a version). An inert file (zero active settings, e.g. the all-commented template from `config init --template`) emits nothing — a commented `# config_version = 3` is not "missing," and there is no `default_provider` to fold. This kills the false alarm that nudged the user toward `config upgrade` in the incident. Reuse the S1 active-setting parser (a pure function over lines). (§9.17 FR-B9)
- Subtask **P2.M2.T1.S2** — Make `config upgrade` on an inert file an honest no-op (FR-B9). In `runConfigUpgrade`/`upgradeConfigVersion`: when the input file has zero active settings, report `"file is inert — nothing to upgrade"` and make no change (never a silent append of a stray `config_version` line dressed up as success). For an active file, the existing surgical edit applies (and now round-trips active settings via the S1 helper from P2.M1). Also ensure the load-time advisory (amended FR-B4) points only at `config upgrade`, never `config init --force`. (§9.17 FR-B4/B5/B9)
  - *Mode A docs (ride with this work):* `docs/configuration.md` (the load-time notice behavior) — state that an inert/template file emits no notice and `config upgrade` on it is a no-op.

**Milestone P2.M3 — Tests for config-write hardening**

Task **P2.M3.T1 — Config-write preservation + inert-suppression tests**
- Subtask **P2.M3.T1.S1** — Tests for FR-B8/B9. (a) `config init --force` over a file with active `[defaults]`/`[role.*]`/`[provider.*]` settings preserves them verbatim (round-trip golden), writes a timestamped backup, and refreshes only template/comments. (b) `config upgrade` round-trips every active line and never leaves an all-commented/inert file; on a genuinely-inert input it reports the no-op and changes nothing. (c) Load emits no migration notice on an inert file (zero active settings) regardless of its commented version; emits the notice only on an active file with an old/unset version. (d) A removed-schema key is commented-out-with-a-note, not deleted. Extend `internal/cmd/config_test.go` and `internal/config` tests; `go test ./internal/cmd/... ./internal/config/...` must pass.

---

# CROSS-CUTTING — Sync changeset-level documentation (Mode B)

**Description:** Once both phases land, reconcile the cross-cutting doc surfaces that only make sense as a whole: the docs that summarize the lock model and the config model top-to-bottom, and the README's reliability/config positioning. The per-requirement Mode A docs (noted under each task above) ride with the implementing work; this is the final coherence pass.

Task **DOC.T1 — Sync changeset-level documentation**
- Subtask **DOC.T1.S1** — Final doc coherence pass. After Phases 1 and 2 are complete: (a) `docs/how-it-works.md` lock subsection — ensure the orphaned-but-alive case, the watchdog, SIGHUP, and `lock status` are all present and consistent with the PRD §18.4/§18.5/§9.27 wording; (b) `docs/cli.md` subcommand list — `lock status` present and ordered consistently; `config init`/`config upgrade` entries reflect FR-B8/B9; (c) `docs/configuration.md` — `noParentWatchdog` git-config key documented alongside the other keys; config-write/notice behavior consistent; (d) `README.md` — if it has a reliability/lock or config-bootstrap blurb, ensure it isn't contradicted (the README in session 013 was confirmed clean of lock/config-write claims, so this is likely a no-op — verify, don't assume); (e) grep the repo (excluding `plan/`) for any remaining reference to the lock model or config-write behavior that predates the watchdog/FR-B8/B9 and is now stale. Smoke test: `make build && ./bin/stagecoach lock status` (must report no-lock cleanly) and `./bin/stagecoach config upgrade` on a temp inert file (must report the no-op). This task depends on all of Phase 1 and Phase 2 being complete.

---

## Out of scope / non-goals for this delta

- **No contender-side force-breaking** (FR52 preserved). The watchdog is the holder self-exiting; `lock status` never auto-breaks. A depth-1 subtractive queue (auto-commit the second batch) remains deferred (Appendix F).
- **No commit/CAS/rescue/index behavior changes** in either theme. Abandoning an in-flight run is safe precisely because HEAD moves only at `update-ref` (§13.2); the snapshot is a gc'able orphan whose SHA the rescue recipe prints.
- **No config schema-version bump.** FR-B4 clarifies that `config_version` is bumped only on a genuinely breaking change; FR-B8/B9 are additive hardening and do not bump it.
- **No Windows watchdog** (FR-K7). `flock` is already a no-op on Windows and the CAS is the guarantee; the parent-death concept (init-reparenting) has no Windows analog.
