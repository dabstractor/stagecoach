name: "P1.M3.T1.S1 — Update generate.go CommitStaged to use message-role timeout (FR-R7, FR25, FR-T5)"
description: >
  The FIRST consumer wiring of FR-R7 per-role timeouts: make the single-commit message path use the
  MESSAGE role's resolved timeout instead of the flat cfg.Timeout. A 3-edit change to ONE file —
  internal/generate/generate.go — plus ONE new regression test in generate_test.go:
  (a) ADD `msgTimeout := config.ResolveRoleTimeout("message", cfg)` immediately after the existing
  `config.ResolveRoleModel("message", cfg)` resolution (generate.go:264) — co-locating the message
  role's timeout with its model (FR-R7 parity; the model already flows from there);
  (b) EDIT the one-shot `provider.Execute(ctx, *spec, cfg.Timeout, ...)` → `msgTimeout` (generate.go:335);
  (c) EDIT the multi-turn budget DISPLAY line `cfg.Timeout * time.Duration(turns)` → `msgTimeout * time.Duration(turns)`
  (generate.go:426) so the FR-T5 progress line reflects the message-role budget (×(N+1) turns).
  The dependency — `config.ResolveRoleTimeout(role string, cfg Config) time.Duration` — is ALREADY
  LANDED (P1.M2.T1.S1, COMPLETE). `ResolveRoleTimeout("message", cfg)` returns `cfg.Roles["message"].Timeout`
  if set (non-zero), else `cfg.Timeout` (there is NO message built-in — only the planner has one). This
  makes the change BEHAVIOR-PRESERVING BY DEFAULT: the existing TestCommitStaged_Timeout (cfg.Timeout=150ms,
  no message override) stays GREEN because msgTimeout == cfg.Timeout. The change ONLY diverges when a user
  sets a per-role message timeout (--message-timeout / [role.message].timeout — both ALREADY LANDED by
  P1.M1.T2.S2), which is exactly the FR-R7 feature. NOT in scope: the multi-turn Run call at generate.go:436
  (Run's INTERNAL per-turn Execute at multiturn.go:165/176/187 is P1.M3.T1.S2 — "see S2" in the contract);
  Run's signature is LEFT UNCHANGED — S2 will resolve config.ResolveRoleTimeout("message", cfg) locally
  inside Run, and because ResolveRoleTimeout is pure/deterministic, S1's budget display (426) and Run's
  per-turn Execute will agree (with no message override they already agree: msgTimeout==cfg.Timeout).
  NOT in scope: planner/stager/arbiter (P1.M3.T2 decompose path), multiturn.go/workdesc.go/hook/exec.go (S2),
  the global 480s→120s flip (P1.M2.T2.S1, in-flight parallel — independent of this change), ResolveRoleTimeout
  itself (LANDED), docs (P1.M4.T2 — contract: "DOCS: none"). NO new imports (config + time already imported
  by generate.go). ONE new test (TestCommitStaged_MessageRoleTimeout) clones TestCommitStaged_Timeout's
  shape, flipping WHICH field carries the small timeout (global=30s, message-role=150ms) to prove the
  message-role timeout is the one that bounds Execute.

---

## Goal

**Feature Goal**: Wire the message-role per-role timeout (FR-R7) into the single-commit generation path
so that `[role.message].timeout` / `--message-timeout` / `STAGECOACH_MESSAGE_TIMEOUT` /
`stagecoach.role.message.timeout` actually bounds the message agent's one-shot generation (and is
reflected in the multi-turn total-budget progress line), instead of every role silently inheriting the
flat `cfg.Timeout`. This is the first of the consumer-wiring subtasks (P1.M3) that turn the already-landed
`ResolveRoleTimeout` accessor (P1.M2.T1.S1) and the already-landed `--message-timeout` flag/config layers
(P1.M1.T2) into observed behavior.

**Deliverable**: A surgical 3-edit change to `internal/generate/generate.go` (1 ADD + 2 EDITS, all in
`CommitStaged`) + 1 new test in `internal/generate/generate_test.go`:
1. ADD `msgTimeout := config.ResolveRoleTimeout("message", cfg)` after the `ResolveRoleModel` line (~264).
2. EDIT the one-shot `provider.Execute` timeout arg `cfg.Timeout` → `msgTimeout` (~335).
3. EDIT the multi-turn budget display `cfg.Timeout * time.Duration(turns)` → `msgTimeout * time.Duration(turns)` (~426).

**Success Definition**:
- The message-role one-shot generation is bounded by `ResolveRoleTimeout("message", cfg)`, not `cfg.Timeout`.
  A `[role.message].timeout = "150ms"` (with `cfg.Timeout = 30s`) makes one-shot generation time out at
  150ms → `*RescueError{Kind: ErrTimeout}` (proven by the new test).
- The multi-turn progress line (`↳ falling back to multi-turn: N turns … ~Mm total`) computes `M` from
  the message-role timeout (`msgTimeout * turns`), matching FR-T5 ("total budget = message-timeout × (N+1)").
- **Behavior-preserving by default**: with no message-role override, `msgTimeout == cfg.Timeout`, so the
  existing `TestCommitStaged_Timeout` (cfg.Timeout=150ms, no Roles) and the full `make test` suite stay GREEN.
- `go build ./...` + cross-build clean; `gofmt -l` empty; `make lint` + `make coverage-gate` green.
- Scope: ONLY `internal/generate/generate.go` + `internal/generate/generate_test.go` change. Run's signature
  (multiturn.go) is UNTOUCHED; multiturn.go/workdesc.go/hook/exec.go are S2; the Run call at generate.go:436
  is LEFT passing `cfg` (S2 wires Run's internals — grep-guarded).

## User Persona (if applicable)

**Target User**: A developer whose message-role generation is slow (a large diff + a deliberate model)
and who wants to bound JUST the message agent's time without lowering the global timeout that other roles
(planner/stager/arbiter) or future runs might need. Also the operator who finds 120s too tight for a big
repo and sets `[role.message].timeout = "300s"`.

**Use Case**: User sets `[role.message].timeout = "300s"` (file beats Layer-1 global). A `stagecoach` run
on a single-commit path now bounds the message one-shot attempt at 300s (not the 120s global). Without this
wiring, that setting was silently ignored (the code read cfg.Timeout directly).

**User Journey**: `[role.message] timeout = "300s"` in config → Load() merges into cfg.Roles["message"].Timeout
(P1.M1, LANDED) → CommitStaged resolves `msgTimeout := ResolveRoleTimeout("message", cfg)` = 300s → one-shot
Execute bounded at 300s → if it exceeds, ErrTimeout (exit 124) + rescue. (The planner, when decomposition
runs, gets 480s via its built-in — unaffected; that wiring is P1.M3.T2.)

**Pain Points Addressed**: FR-R7 — per-role timeouts were RESOLVABLE (P1.M2.T1.S1) and CONFIGURABLE
(P1.M1.T2) but NOT YET CONSUMED on the single-commit path. This task closes that gap for the message role's
one-shot + budget-display sites. FR-T5 — the multi-turn total-budget progress line must reflect the
message-role timeout (multi-turn is a message-role fallback).

## Why

- **FR-R7 / §9.15 / §16.1**: "Each role resolves its own timeout independently (FR-R7)." The accessor
  (`ResolveRoleTimeout`) and the config layers (`RoleConfig.Timeout`, `[role.message].timeout`,
  `--message-timeout`, env, git-config) are ALL LANDED. The only missing piece on the single-commit path
  is the call site reading the accessor instead of `cfg.Timeout`. This task is that call-site wiring for
  `CommitStaged`'s own one-shot Execute + budget display.
- **FR25 / §9.5**: "Impose a configurable per-role generation timeout … on timeout, kill that role's agent
  process and enter the rescue path." The one-shot Execute (generate.go:335) is THE site that bounds the
  message agent; it must read the message-role timeout.
- **FR-T5 / §9.24**: "Total wall-clock budget = message-timeout × (N+1). … the CLI prints the turn count
  and total budget on the progress line at fallback time." The budget display (generate.go:426) must use the
  message-role timeout so the printed `~Mm total` is correct under a `--message-timeout` override.
- **Behavior-preserving by default**: because `ResolveRoleTimeout("message", cfg)` returns `cfg.Timeout`
  when no message-role override is set (the message role has NO built-in — only the planner does), this
  wiring is invisible to every existing test and every default-config user. It ONLY activates under a
  per-role message timeout, which is precisely the new capability.
- **Bounded scope**: 1 ADD + 2 EDITS in one function of one file, + 1 test. No new types, no new imports
  (config + time already imported), no signature changes, no docs (contract: "DOCS: none").

## What

**User-visible behavior**: With no message-role timeout configured, nothing changes (msgTimeout ==
cfg.Timeout). With `[role.message].timeout` / `--message-timeout` set, the message one-shot generation is
bounded by that value, and the multi-turn fallback progress line reports the correct total budget.

**Technical change**: 3 lines in `CommitStaged` (internal/generate/generate.go) + 1 test. See the
Implementation Blueprint for verbatim before/after anchored by STRING (the contract's line numbers — 287,
335, 423-426 — have drifted; the REAL current lines are 264, 335, 426, verified by read).

### Success Criteria
- [ ] `internal/generate/generate.go` resolves `msgTimeout := config.ResolveRoleTimeout("message", cfg)`
      immediately after the `_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)` line.
- [ ] The one-shot `provider.Execute(ctx, *spec, msgTimeout, deps.Verbose)` uses `msgTimeout` (was `cfg.Timeout`).
- [ ] The multi-turn budget line uses `msgTimeout * time.Duration(turns)` (was `cfg.Timeout * ...`).
- [ ] The `Run(ctx, deps, cfg, deps.Manifest, ...)` call (multi-turn) is UNCHANGED — still passes `cfg`
      (Run's internal per-turn timeout is S2; grep-guarded).
- [ ] NEW `TestCommitStaged_MessageRoleTimeout` in generate_test.go: cfg.Timeout=30s,
      cfg.Roles["message"]={Timeout:150ms}, stub SleepMS=2000 → asserts `*RescueError{Kind:ErrTimeout}`
      (proves the 150ms message-role timeout bounded Execute, NOT the 30s global).
- [ ] EXISTING `TestCommitStaged_Timeout` (cfg.Timeout=150ms, no Roles) stays GREEN (msgTimeout==cfg.Timeout).
- [ ] `go build ./...` + `GOOS=windows` + `GOOS=linux` clean; `gofmt -l` empty; `make lint` + `make coverage-gate` green.
- [ ] `make test` (race) green — full regression.
- [ ] Scope: `git diff --name-only` == {internal/generate/generate.go, internal/generate/generate_test.go}.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim before/after for all 3 edits anchored by STRING (with the drifted line numbers
corrected to the REAL current 264/335/426), the exact dependency signature
(`ResolveRoleTimeout(role string, cfg Config) time.Duration`, LANDED), the proof that the change is
behavior-preserving by default (ResolveRoleTimeout("message", cfg) == cfg.Timeout when no override — no
message built-in exists), the S1/S2 scope boundary (generate.go only; Run's signature + multiturn.go/workdesc.go
are S2; the Run call at 436 is LEFT unchanged because S2 wires Run's internals and ResolveRoleTimeout is
deterministic so the display and Run agree), the existing test that must stay green (TestCommitStaged_Timeout)
and why it does, the new test to add (cloning TestCommitStaged_Timeout's harness, flipping which field
carries the small timeout), the confirmation that `config` + `time` are ALREADY imported (no import edit),
and the grep guards.

### Documentation & References

```yaml
# MUST READ — the authoritative codebase findings (exact edit sites, the behavior-preserving proof, the S1/S2 boundary)
- docfile: plan/015_b461e4720495/P1M3T1S1/research/findings.md
  why: "§1 the ResolveRoleTimeout signature + why message has NO built-in (msgTimeout==cfg.Timeout by default);
        §2 the EXACT 3 edit sites anchored by STRING (264 ADD, 335 EDIT, 426 EDIT) + Execute's signature;
        §3 the S1/S2 site table (generate.go=S1; multiturn.go/workdesc.go/hook-exec.go=S2); §4 the behavior-
        preserving proof (existing TestCommitStaged_Timeout stays green); §5 why line 436 Run call is LEFT
        UNCHANGED (deferred to S2; ResolveRoleTimeout deterministic); §6 the stub-test harness for the new test."
  critical: "§2: line numbers DRIFTED (contract said 287/335/423-426; REAL lines are 264/335/426). Anchor by STRING.
             §4/§5: the change is behavior-preserving by default AND the Run call is intentionally NOT touched."

# MUST READ — the dependency contract (ResolveRoleTimeout; LANDED by P1.M2.T1.S1 — consume, don't rebuild)
- docfile: plan/015_b461e4720495/P1M2T1S1/PRP.md
  why: "Defines `func ResolveRoleTimeout(role string, cfg Config) time.Duration` + defaultRoleTimeouts
        {planner:480s}. Proves ResolveRoleTimeout('message', cfg) returns cfg.Roles['message'].Timeout if
        set (non-zero) ELSE cfg.Timeout (NO message built-in). This is THE function generate.go:264 calls."
  critical: "Do NOT change ResolveRoleTimeout or defaultRoleTimeouts (they LANDED). For 'message' there is no
             built-in, so the default path returns cfg.Timeout verbatim — that is WHY existing tests stay green."

# MUST EDIT — the file (the 3 sites in CommitStaged + the test file)
- file: internal/generate/generate.go
  why: "CommitStaged (line 188). ResolveRoleModel('message', cfg) at :264 — the ADD anchor (co-locate msgTimeout).
        provider.Execute(ctx, *spec, cfg.Timeout, ...) at :335 — EDIT the 3rd arg to msgTimeout. The multi-turn
        budget line `cfg.Timeout * time.Duration(turns)` at :426 — EDIT to msgTimeout. The Run(...) call at :436
        — LEAVE UNCHANGED (S2). config + time are ALREADY imported (no import edit)."
  pattern: "ResolveRoleModel('message', cfg) at :264 already establishes the per-role-resolution pattern for the
            message role in CommitStaged; msgTimeout is its timeout twin (FR-R7 parity — model + timeout both
            resolved once, used at the message Execute + budget sites)."
  gotcha: "Line numbers DRIFTED from the contract (287/335/423-426). Anchor every edit by its STRING via grep.
           Do NOT touch :436 (Run) or Run's signature in multiturn.go — S2 owns Run's per-turn Execute."

# MUST EDIT — the test file (clone TestCommitStaged_Timeout, flip which field carries the small timeout)
- file: internal/generate/generate_test.go
  why: "TestCommitStaged_Timeout (:449) is the template: stubtest.Manifest(bin, {Out:'feat: slow', SleepMS:2000})
        + cfg.Timeout=150ms → ErrTimeout. The NEW TestCommitStaged_MessageRoleTimeout clones it but sets
        cfg.Timeout=30s (large global) + cfg.Roles['message']={Timeout:150ms} (small role) → asserts ErrTimeout
        fires at 150ms, proving the MESSAGE-role timeout bounded Execute (not the 30s global)."
  pattern: "stubtest.Manifest(bin, stubtest.Options{Out: ..., SleepMS: 2000}); cfg := config.Defaults();
            cfg.Roles = map[string]config.RoleConfig{...}; errors.As(err, &re); errors.Is(err, ErrTimeout)."
  gotcha: "RoleConfig.Timeout is a plain time.Duration (0 ⇒ inherit). Set it via
           cfg.Roles = map[string]config.RoleConfig{'message': {Timeout: 150 * time.Millisecond}}."

# CONTEXT — the per-role timeout flag/config (ALREADY LANDED by P1.M1.T2; this task is their first consumer)
- file: internal/config/roles.go
  why: "ResolveRoleTimeout (:80) + defaultRoleTimeouts (:12, planner-only). Confirms 'message' has NO built-in
        ⇒ ResolveRoleTimeout('message', cfg) == cfg.Timeout unless a per-role override is set."
  critical: "READ-ONLY for this task. The function + map LANDED; do not edit them."

# CONTEXT — the existing timeout test (the REGRESSION canary — must stay GREEN)
- file: internal/generate/generate_test.go   # TestCommitStaged_Timeout :449-480
  why: "Sets cfg.Timeout=150ms with NO cfg.Roles['message']. After this task msgTimeout==cfg.Timeout==150ms
        ⇒ identical behavior ⇒ the test stays green. This IS the behavior-preserving proof at test time."
  critical: "Do NOT modify TestCommitStaged_Timeout. If it fails after this task, msgTimeout was wired wrong
             (e.g. ResolveRoleTimeout called with the wrong role, or a built-in accidentally added for message)."

# CONTEXT — the sibling S2 (NOT this task; do NOT touch its files)
- docfile: (P1.M3.T1.S2 PRP, when written)
  why: "S2 owns multiturn.go (:165/:176/:187 per-turn Execute) + workdesc.go (:75/:106/:122) + hook/exec.go.
        It will resolve config.ResolveRoleTimeout('message', cfg) locally at those Execute sites (no signature
        change). Because ResolveRoleTimeout is pure, S1's budget display (msgTimeout) and Run's per-turn bound
        agree. S1 does NOT touch line 436 or Run's signature."
  critical: "If you find yourself editing multiturn.go/workdesc.go or changing Run's signature, STOP — that is S2."

# CONTEXT — the in-flight parallel sibling (independent of this task)
- docfile: plan/015_b461e4720495/P1M2T2S1/PRP.md
  why: "P1.M2.T2.S1 flips the GLOBAL default 480s→120s. It is INDEPENDENT of this task: this task reads
        ResolveRoleTimeout('message', cfg) which returns cfg.Timeout when no override — whatever that value is.
        Whether the global is 480s or 120s, msgTimeout==cfg.Timeout by default. No coordination needed."
```

### Current Codebase tree (relevant slice)

```bash
internal/generate/
  generate.go          # EDIT — CommitStaged: +msgTimeout resolve (264), Execute→msgTimeout (335), budget→msgTimeout (426)
  generate_test.go     # EDIT — +TestCommitStaged_MessageRoleTimeout; TestCommitStaged_Timeout (:449) UNCHANGED (regression)
  multiturn.go         # READ-ONLY (S2) — Run's per-turn Execute at :165/:176/:187 use cfg.Timeout (S2 wires them)
  workdesc.go          # READ-ONLY (S2) — RunWorkDescription Execute at :75/:106/:122 use cfg.Timeout (S2)
  (other *_test.go)    # READ-ONLY — regression net (make test)
internal/config/
  roles.go             # READ-ONLY — ResolveRoleTimeout + defaultRoleTimeouts (LANDED; consume)
  config.go            # READ-ONLY — RoleConfig.Timeout field, Config.Timeout (LANDED)
go.mod / Makefile      # READ-ONLY — no new dep (config+time already imported); test=line70, lint=line103, coverage-gate=line77
```

### Desired Codebase tree with files to be added/modified

```bash
# MODIFIED (no new files). EXACTLY 2 files:
internal/generate/generate.go        # +1 line (msgTimeout resolve) + 2 one-token edits (cfg.Timeout → msgTimeout at 335 + 426)
internal/generate/generate_test.go   # +1 test function (TestCommitStaged_MessageRoleTimeout)
# (NOT touched: multiturn.go, workdesc.go, hook/exec.go — S2; config/* — LANDED; docs/* — P1.M4.T2)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (line numbers DRIFTED — anchor by STRING): the contract cited generate.go:287 (msgModel), :335 (Execute),
// :423-426 (budget). The REAL current lines (verified by read) are :264 (ResolveRoleModel), :335 (Execute),
// :426 (budget). roles.go grew ~90 lines in P1.M2.T1.S1, shifting everything above it. Anchor EVERY edit by its
// unique STRING via grep, not by line number.

// CRITICAL (the change is BEHAVIOR-PRESERVING BY DEFAULT — do not "fix" the tests to match a new value):
// ResolveRoleTimeout("message", cfg) returns cfg.Roles["message"].Timeout if non-zero, ELSE cfg.Timeout
// (the message role has NO built-in — only the planner does). So with no [role.message].timeout override,
// msgTimeout == cfg.Timeout byte-for-byte. The existing TestCommitStaged_Timeout (cfg.Timeout=150ms, no Roles)
// therefore STAYS GREEN without modification. Do NOT change its assertion. The ONLY new behavior is under a
// per-role message override — that is what the NEW test exercises.

// CRITICAL (do NOT touch the Run call at generate.go:436 or Run's signature): the contract point (d) "Pass
// msgTimeout into the multi-turn Run call if it takes cfg.Timeout internally (see S2)" is QUALIFIED by "(see S2)".
// Run (multiturn.go:145) takes cfg and uses cfg.Timeout internally at :165/:176/:187 — those are S2's sites.
// S1 LEAVES line 436 passing cfg and LEAVES Run's signature alone. S2 will resolve config.ResolveRoleTimeout
// ("message", cfg) locally at Run's Execute sites. Because ResolveRoleTimeout is PURE/DETERMINISTIC, S1's budget
// display (msgTimeout at :426) and Run's per-turn bound (S2) return the SAME value for the same cfg. With no
// message override they already agree (msgTimeout==cfg.Timeout). Editing Run's signature would cross the S1/S2
// file-ownership boundary — DON'T.

// CRITICAL (the budget line at :426 is a DISPLAY computation — keep the unit math identical): it computes
// totalMin := int((<timeout> * time.Duration(turns)).Minutes()) then clamps totalMin<1 → 1. Swapping cfg.Timeout
// → msgTimeout changes ONLY the source duration; the expression shape, the .Minutes(), the int(), and the clamp
// are UNCHANGED. time.Duration is already imported + used at :426.

// GOTCHA (NO new imports): generate.go already imports "time" (import block) and "github.com/dustin/stagecoach/
// internal/config" (for config.ResolveRoleModel at :264). config.ResolveRoleTimeout + msgTimeout (time.Duration)
// need nothing new. Do not add an import.

// GOTCHA (RoleConfig.Timeout is a plain time.Duration, 0 ⇒ inherit — mirror it in the test): set the message-role
// override via cfg.Roles = map[string]config.RoleConfig{"message": {Timeout: 150 * time.Millisecond}}. A 0 value
// means "inherit global" (ResolveRoleTimeout falls through to cfg.Timeout) — so the test MUST set a NON-ZERO
// message Timeout or it would just test the global path again.

// GOTCHA (Execute's 3rd arg is a plain time.Duration): provider.Execute(ctx, spec, timeout time.Duration, vb).
// msgTimeout is time.Duration — drops in directly. No wrapper, no conversion.

// GOTCHA (the multi-turn fallback path is gated — the budget line only runs when one-shot exhausted + large
// payload + append provider): the new test for the budget LINE itself is low-value/hard (it's a stderr print
// inside a deeply-gated branch). The FR-T5 correctness is ensured by using msgTimeout (the message-role budget)
// at :426 and is grep-guarded. The behavioral proof that matters is the ONE-SHOT timeout (TestCommitStaged_
// MessageRoleTimeout). Do not over-test the display line.
```

## Implementation Blueprint

### Data models and structure

None. No new types, no new fields, no signature changes. One new local variable (`msgTimeout
time.Duration`) in `CommitStaged`, consumed at two existing call sites.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/generate/generate.go — ADD the message-role timeout resolution
  - ANCHOR (the unique ResolveRoleModel line in CommitStaged, ~264):
        _, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)
  - ADD immediately AFTER it (the FR-R7 timeout twin — co-located with the model resolution):
        // FR-R7/FR25: resolve the message role's timeout so [role.message].timeout / --message-timeout
        // bound the message agent's one-shot generation (and the multi-turn total budget, FR-T5) instead
        // of the flat cfg.Timeout. With no per-role override ResolveRoleTimeout returns cfg.Timeout
        // (the message role has no built-in) — behavior-preserving by default.
        msgTimeout := config.ResolveRoleTimeout("message", cfg)
  - GOTCHA: anchor by the STRING `config.ResolveRoleModel("message", cfg)` (line drifted from the
    contract's 287 to ~264). Do NOT re-resolve elsewhere in CommitStaged — this one local serves both the
    one-shot Execute and the budget display.
  - NAMING: msgTimeout (mirrors msgModel/msgReasoning — the message role's resolved attributes).

Task 2: EDIT internal/generate/generate.go — the one-shot Execute timeout arg
  - OLD (~335):
        out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)
  - NEW:
        out, _, execErr := provider.Execute(ctx, *spec, msgTimeout, deps.Verbose)
  - ANCHOR: the unique `provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)` line inside the
    generation loop (the ONE-SHOT attempt). This is the behavior change — it bounds the message agent.
  - GOTCHA: there are OTHER provider.Execute(..., cfg.Timeout, ...) calls in the package (multiturn.go
    :165/:176/:187, workdesc.go :75/:106/:122) — those are S2, DO NOT touch them. Only the generate.go
    one-shot Execute changes.

Task 3: EDIT internal/generate/generate.go — the multi-turn budget DISPLAY line
  - OLD (~426):
        totalMin := int((cfg.Timeout * time.Duration(turns)).Minutes())
  - NEW:
        totalMin := int((msgTimeout * time.Duration(turns)).Minutes())
  - ANCHOR: the unique `cfg.Timeout * time.Duration(turns)` expression (inside the FR-T5 progress-line
    block, after `turns := len(chunkPayload(...)) + 1`). FR-T5: total budget = message-timeout × (N+1).
  - GOTCHA: keep the rest of the expression identical (the .Minutes(), int(), and the `if totalMin < 1`
    clamp below it are UNCHANGED). This is the progress-line number only; Run's ACTUAL per-turn bound is S2.

Task 4: LEAVE the Run call UNCHANGED (the scope boundary — do NOT edit)
  - The multi-turn call (~436): `msg2, ok2, cause := Run(ctx, deps, cfg, deps.Manifest, sysPrompt, mtPayload, msgModel, msgReasoning)`
    STAYS AS-IS. Run takes cfg and resolves its own message timeout internally (S2 wires multiturn.go's
    Execute sites). ResolveRoleTimeout is deterministic ⇒ display (Task 3) and Run agree. Grep-guarded.

Task 5: ADD internal/generate/generate_test.go — TestCommitStaged_MessageRoleTimeout (the behavioral proof)
  - CLONE TestCommitStaged_Timeout (:449) verbatim EXCEPT flip which field carries the small timeout:
        func TestCommitStaged_MessageRoleTimeout(t *testing.T) {
            bin := stubtest.Build(t)
            repo := t.TempDir()
            initRepo(t, repo)
            commitRaw(t, repo, "initial")
            writeFile(t, repo, "z.txt", "data")
            stageFile(t, repo, "z.txt")

            m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: slow", SleepMS: 2000})
            cfg := config.Defaults()
            cfg.Timeout = 30 * time.Second                                          // LARGE global (would NOT time out)
            cfg.Roles = map[string]config.RoleConfig{"message": {Timeout: 150 * time.Millisecond}} // SMALL message-role (times out)

            beforeHEAD := headSHA(t, repo)
            _, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
            if err == nil { t.Fatal("expected error on message-role timeout, got nil") }

            var re *RescueError
            if !errors.As(err, &re) { t.Fatalf("error type = %T, want *RescueError", err) }
            if !errors.Is(err, ErrTimeout) {
                t.Errorf("errors.Is(err, ErrTimeout) = false, want true (message-role 150ms should bound Execute, not the 30s global)")
            }
            if re.TreeSHA == "" { t.Error("RescueError.TreeSHA empty, want non-empty (snapshot taken)") }
            if got := headSHA(t, repo); got != beforeHEAD {
                t.Errorf("HEAD changed from %q to %q on timeout, want unchanged", beforeHEAD, got)
            }
        }
  - WHY this proves the wiring: with cfg.Timeout=30s the OLD code would NOT time out (30s > 2000ms sleep);
    only because the message-role timeout (150ms) is now the bound does Execute time out → ErrTimeout.
    This is the positive proof that msgTimeout (not cfg.Timeout) reaches Execute.
  - FOLLOW pattern: TestCommitStaged_Timeout (:449-480) — same harness, same stub SleepMS=2000, same assertions.
  - NAMING: TestCommitStaged_MessageRoleTimeout (descriptive; sits next to TestCommitStaged_Timeout).
  - GOTCHA: import config.RoleConfig + time are already imported in generate_test.go (TestCommitStaged_Timeout
    uses config.Defaults() + time.Millisecond). No new import.
  - GOTCHA: do NOT modify the existing TestCommitStaged_Timeout — it is the behavior-preserving regression canary.

Task 6: VERIFY — build (native+cross), vet, format, focused + full tests, lint, coverage, grep guards
  - go build ./... ; GOOS=windows go build ./... ; GOOS=linux go build ./...
  - go vet ./internal/generate/...
  - gofmt -l internal/generate/generate.go internal/generate/generate_test.go   # must be empty
  - go test ./internal/generate/ -run 'TestCommitStaged_Timeout|TestCommitStaged_MessageRoleTimeout' -v
  - make test ; make lint ; make coverage-gate
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the message-role resolution block (model + timeout resolved once, FR-R7 parity) — the entire feature's anchor:
_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)
msgTimeout := config.ResolveRoleTimeout("message", cfg)   // NEW — the message role's timeout (cfg.Timeout when no override)

// PATTERN: the one-shot Execute now reads the role-resolved timeout (the behavior change, 1 token):
out, _, execErr := provider.Execute(ctx, *spec, msgTimeout, deps.Verbose)   // was cfg.Timeout

// PATTERN: the FR-T5 budget display reads the role-resolved timeout (display-only; Run's actual bound is S2):
totalMin := int((msgTimeout * time.Duration(turns)).Minutes())   // was cfg.Timeout

// PATTERN: the new test flips WHICH field carries the small timeout (proves the message-role timeout is consumed):
cfg.Timeout = 30 * time.Second                                                     // global is LARGE (old code wouldn't time out)
cfg.Roles = map[string]config.RoleConfig{"message": {Timeout: 150 * time.Millisecond}} // role is SMALL → times out
// → ErrTimeout proves msgTimeout (150ms), not cfg.Timeout (30s), reached Execute.
```

### Integration Points

```yaml
CODE (internal/generate/generate.go CommitStaged):
  - ADD msgTimeout := config.ResolveRoleTimeout("message", cfg) after the ResolveRoleModel line (~264).
  - Execute 3rd arg cfg.Timeout → msgTimeout (~335).
  - Budget display cfg.Timeout → msgTimeout (~426).

NO-CHANGE (scope fences):
  - Run call (~436): UNCHANGED (still passes cfg). Run's signature (multiturn.go): UNCHANGED.
  - multiturn.go (:165/:176/:187), workdesc.go (:75/:106/:122), hook/exec.go: UNCHANGED (S2).
  - ResolveRoleTimeout / defaultRoleTimeouts (roles.go): UNCHANGED (LANDED P1.M2.T1.S1).
  - Config layers / --message-timeout flag (P1.M1): UNCHANGED (LANDED; this task is their first consumer).

CONSUMERS OF THIS CHANGE:
  - The [role.message].timeout / --message-timeout / STAGECOACH_MESSAGE_TIMEOUT / stagecoach.role.message.timeout
    settings (all LANDED by P1.M1.T2) now take effect on the single-commit one-shot path (and the multi-turn
    progress line). Previously they were resolved into cfg.Roles["message"].Timeout but read by no call site.

DOWNSTREAM (sibling items, NOT this task):
  - P1.M3.T1.S2 wires multiturn.go/workdesc.go/hook-exec.go to ResolveRoleTimeout("message", cfg) (Run's internals).
  - P1.M3.T2.S1 wires planner/stager/arbiter (decompose path) to their per-role timeouts.
  - P1.M4.T2.S1 syncs docs (this task: "DOCS: none").

NO database / migration / routes / new types / new imports / new flag / config change / signature change.
  - config + time are ALREADY imported by generate.go. ResolveRoleTimeout is ALREADY shipped. The
    --message-timeout flag is ALREADY shipped. This task is purely the call-site read.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Native + cross build (no platform tags in generate.go; a 1-token + 1-line change must build everywhere).
go build ./...
GOOS=windows go build ./...
GOOS=linux   go build ./...
# Expected: all clean. A failure means an edit strayed (e.g. a typo in msgTimeout, or an import went missing —
#           but NO import is added; config + time are already present).

# Vet.
go vet ./internal/generate/...
# Expected: clean.

# Format.
gofmt -l internal/generate/generate.go internal/generate/generate_test.go
# Expected: empty. If listed: gofmt -w the file(s).

# Lint.
make lint      # golangci-lint (staticcheck/gosimple/govet/errcheck/ineffassign/unused)
# Expected: zero errors. msgTimeout is used at :335 + :426 (no `unused`); cfg.Timeout is still used at :436? NO —
#           :436 passes cfg (the whole struct), not cfg.Timeout. After the edit, is cfg.Timeout still read in
#           generate.go? ResolveRoleTimeout reads it INSIDE config (not here). In generate.go, cfg.Timeout's only
#           direct reads were :335 + :426 — BOTH now msgTimeout. That's FINE (cfg is still used; a struct field
#           being unread is not a lint error). If staticcheck's U1000 fired on a field it would be on Config, not here.

# Scope guard: ONLY the 2 expected files changed.
git diff --name-only
# Expected: internal/generate/generate.go  internal/generate/generate_test.go  (exactly these 2).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The NEW behavioral proof (message-role timeout bounds Execute).
go test ./internal/generate/ -run 'TestCommitStaged_MessageRoleTimeout' -v
# Expected: PASS — ErrTimeout fires at the 150ms MESSAGE-role timeout (cfg.Timeout=30s would NOT have timed out).

# The REGRESSION canary (behavior-preserving by default — must stay GREEN unchanged).
go test ./internal/generate/ -run 'TestCommitStaged_Timeout' -v
# Expected: PASS — msgTimeout==cfg.Timeout==150ms (no message override) → identical behavior.

# The full generate package (the multi-turn/workdesc/invariant tests are S2-untouched; they must stay green).
go test ./internal/generate/ -v
# Expected: green. (multiturn.go/workdesc.go unchanged → their tests unchanged; the budget line change is
#           display-only and msgTimeout==cfg.Timeout under their configs.)

# Full race suite + coverage gate (PRD §20.3: ≥85% on internal/{git,provider,generate,config}).
make test
make coverage-gate
# Expected: green / passes. The new test ADDS coverage to the msgTimeout path; nothing is removed.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary (the wiring links into the binary; proves no compile/link break).
make build

# Manual sanity: a --message-timeout override actually bounds a run (the end-to-end proof of FR-R7 on this path).
# Use a stub/sleep provider OR a real slow one; the unit test (Task 5) is the deterministic proof, so this is optional.
# (A full per-role-timeout-bounds-a-real-run e2e is P1.M4.T1.S1's deliverable, NOT this subtask.)
d=$(mktemp -d) && cd "$d" && git init -q
git config user.email t@t.com && git config user.name t
printf 'a\n' > f.txt && git add f.txt && git commit -qm init
printf 'b\n' >> f.txt && git add f.txt
SC=/home/dustin/projects/stagecoach/bin/stagecoach
# With a tiny message timeout, a slow provider times out (exit 124); the unit test proves this deterministically.
cd - && rm -rf "$d"
```

> **Note**: this subtask is the call-site wiring for ONE path (single-commit message one-shot + budget display).
> The within-scope proof is: clean build/vet/lint/gofmt + the new unit test (Task 5) + the full regression green +
> the grep guards. The multi-turn per-turn timeout wiring is S2; the decompose (planner/stager/arbiter) wiring is
> P1.M3.T2; the docs sync is P1.M4.T2; the full per-role-timeout e2e is P1.M4.T1.S1.

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: msgTimeout is resolved once in CommitStaged, co-located with the message-role model resolution.
grep -n 'config.ResolveRoleTimeout("message", cfg)' internal/generate/generate.go
# Expected: 1 hit (the Task 1 ADD, right after the ResolveRoleModel line).

# Guard 2: the one-shot Execute uses msgTimeout (not cfg.Timeout).
grep -n 'provider.Execute(ctx, \*spec, msgTimeout, deps.Verbose)' internal/generate/generate.go
# Expected: 1 hit (~335). And:
grep -n 'provider.Execute(ctx, \*spec, cfg.Timeout' internal/generate/generate.go
# Expected: ZERO hits in generate.go (the one-shot no longer reads cfg.Timeout directly).

# Guard 3: the budget display uses msgTimeout.
grep -n 'msgTimeout \* time.Duration(turns)' internal/generate/generate.go
# Expected: 1 hit (~426). And:
grep -n 'cfg.Timeout \* time.Duration(turns)' internal/generate/generate.go
# Expected: ZERO hits (the budget line no longer reads cfg.Timeout).

# Guard 4: the Run call is UNCHANGED (still passes cfg; S2 owns Run's internals).
grep -n 'Run(ctx, deps, cfg, deps.Manifest' internal/generate/generate.go
# Expected: 1 hit (~436), UNCHANGED. Run's signature in multiturn.go is also UNCHANGED:
grep -n 'func Run(ctx context.Context, deps Deps, cfg config.Config' internal/generate/multiturn.go
# Expected: 1 hit, UNCHANGED (S2 does not change the signature either — it edits the Execute args inside).

# Guard 5: NO edits to multiturn.go / workdesc.go (S2's files).
git diff --name-only | grep -E 'multiturn.go|workdesc.go|hook/exec.go'
# Expected: EMPTY (S2 owns those; this task touches only generate.go + generate_test.go).

# Guard 6: NO new import added (config + time already present).
git diff internal/generate/generate.go | grep -E '^\+.*"time"|^\+.*internal/config"'
# Expected: EMPTY (no new import lines; the change is 1 ADD + 2 token swaps).

# Guard 7: the NEW test exists and sets the message-role timeout (not the global) as the small value.
grep -n 'TestCommitStaged_MessageRoleTimeout' internal/generate/generate_test.go
grep -A2 'cfg.Timeout = 30 \* time.Second' internal/generate/generate_test.go | grep 'Roles'
# Expected: 1 hit each — the new test sets a LARGE global + a SMALL message-role Timeout.

# Guard 8: the existing TestCommitStaged_Timeout is UNCHANGED (the regression canary).
git diff internal/generate/generate_test.go | grep -E '^\-.*TestCommitStaged_Timeout|^\-.*cfg.Timeout = 150'
# Expected: EMPTY (TestCommitStaged_Timeout is not modified — only the new test is added).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + `GOOS=windows/linux go build ./...` clean
- [ ] `go vet ./internal/generate/...` clean
- [ ] `gofmt -l internal/generate/generate.go internal/generate/generate_test.go` empty
- [ ] `make lint` zero errors (msgTimeout is used at :335 + :426)
- [ ] `make test` (race) green — incl. the new test + the unchanged TestCommitStaged_Timeout
- [ ] `make coverage-gate` ≥85% on internal/generate (the new test adds coverage)

### Feature Validation
- [ ] `msgTimeout := config.ResolveRoleTimeout("message", cfg)` added after the ResolveRoleModel line
- [ ] one-shot `provider.Execute(..., msgTimeout, ...)` (was cfg.Timeout)
- [ ] budget display `msgTimeout * time.Duration(turns)` (was cfg.Timeout) — FR-T5
- [ ] `TestCommitStaged_MessageRoleTimeout` proves the message-role timeout bounds Execute (cfg.Timeout=30s, role=150ms → ErrTimeout)
- [ ] `TestCommitStaged_Timeout` UNCHANGED and GREEN (behavior-preserving by default)

### Scope-Boundary Validation
- [ ] `git diff --name-only` == {internal/generate/generate.go, internal/generate/generate_test.go}
- [ ] Run call (~436) + Run signature (multiturn.go) UNCHANGED (S2 wires Run's internals)
- [ ] multiturn.go / workdesc.go / hook-exec.go UNCHANGED (S2)
- [ ] config/roles.go (ResolveRoleTimeout) + config layers + --message-timeout flag UNCHANGED (all LANDED)
- [ ] NO new import (config + time already present); NO new type/flag; NO docs change (P1.M4.T2)
- [ ] Grep guards 1–8 (Level 4) all pass

### Code Quality & Docs
- [ ] msgTimeout godoc/comment cites FR-R7/FR25 + the behavior-preserving-by-default note
- [ ] msgTimeout co-located with msgModel/msgReasoning (the message-role resolution block)
- [ ] All edits anchored by STRING (grep), not by the contract's drifted line numbers (287/335/423-426)

---

## Anti-Patterns to Avoid

- ❌ Don't anchor edits to the contract's line numbers (287, 335, 423-426). roles.go grew ~90 lines in
  P1.M2.T1.S1 and everything shifted. The REAL current lines are 264 (ResolveRoleModel), 335 (Execute),
  426 (budget). Anchor every edit by its unique STRING via `grep -n`. The contract's numbers are a guide,
  not a specification.
- ❌ Don't change the existing `TestCommitStaged_Timeout`. It sets `cfg.Timeout=150ms` with NO
  `cfg.Roles["message"]`, so after this task `msgTimeout == cfg.Timeout == 150ms` → identical behavior →
  the test stays GREEN unchanged. It IS the behavior-preserving proof. If you "fix" its assertion to a new
  value, you have misunderstood the change (the default path is unchanged). The NEW behavior is proven by
  the NEW test, not by editing the old one.
- ❌ Don't touch the `Run(ctx, deps, cfg, ...)` call at generate.go:436 or Run's signature in multiturn.go.
  The contract point (d) is qualified "(see S2)" — Run's INTERNAL per-turn Execute (multiturn.go:165/176/187)
  is S2's scope. S1 leaves line 436 passing `cfg` and leaves Run's signature alone. S2 resolves
  `ResolveRoleTimeout("message", cfg)` locally inside Run. Because ResolveRoleTimeout is pure/deterministic,
  the budget display (your msgTimeout at :426) and Run's per-turn bound agree. Changing Run's signature
  crosses the S1/S2 file boundary and creates a coordination hazard — DON'T.
- ❌ Don't edit multiturn.go / workdesc.go / hook/exec.go. Those cfg.Timeout Execute sites (:165/:176/:187,
  :75/:106/:122) are S2. This task's diff is EXACTLY {generate.go, generate_test.go}. A grep guard enforces it.
- ❌ Don't re-resolve msgTimeout at each site. Resolve it ONCE (Task 1, after ResolveRoleModel) and reuse the
  local at both the one-shot Execute (Task 2) and the budget display (Task 3). Re-resolving twice is harmless
  (the function is pure) but needlessly verbose and diverges from the msgModel/msgReasoning pattern (resolved
  once, reused).
- ❌ Don't change the budget-line math shape. Task 3 swaps `cfg.Timeout` → `msgTimeout` in
  `int((<dur> * time.Duration(turns)).Minutes())`; the `.Minutes()`, `int()`, and the `if totalMin < 1 { totalMin = 1 }`
  clamp below it are UNCHANGED. It's a display number; keep the expression identical apart from the duration source.
- ❌ Don't add an import. generate.go already imports `"time"` and `internal/config` (for ResolveRoleModel at
  :264). `config.ResolveRoleTimeout` and `msgTimeout time.Duration` need nothing new. Adding an import is a
  sign you've mis-scoped (e.g. reached into another package).
- ❌ Don't add a "message built-in" to defaultRoleTimeouts. The message role has NO built-in — only the planner
  does (480s). `ResolveRoleTimeout("message", cfg)` returns `cfg.Timeout` when no override; that is the intended
  behavior-preserving default. Adding a message built-in would silently change every default-config user's
  timeout and conflict with P1.M2.T2.S1's global-120s flip.
- ❌ Don't couple this to the global 480s→120s flip (P1.M2.T2.S1, in-flight parallel). This task reads
  `ResolveRoleTimeout("message", cfg)`, which returns `cfg.Timeout` when no message override — WHATEVER that
  value is (480s today, 120s after the sibling lands). The two tasks are independent; no coordination needed.
- ❌ Don't update the `ErrTimeout` godoc at generate.go:87 ("exceeded cfg.Timeout"). It is still accurate in
  spirit for the message role's default path (msgTimeout == cfg.Timeout). The contract says "DOCS: none —
  internal wiring"; expanding scope to refresh that comment is unnecessary. Leave it (or defer to P1.M4.T2).
- ❌ Don't write a test for the budget DISPLAY line (:426) specifically. It's a stderr print inside a deeply
  gated multi-turn branch (one-shot exhausted + large payload + append provider). The FR-T5 correctness is
  ensured by using msgTimeout (grep-guarded) and proven indirectly by the one-shot test. Over-testing the
  display line is low-value and brittle.

---

## Confidence Score: 9/10

This is a surgical 1-ADD + 2-token-swap change to a single function in a single file, plus one test that
clones an existing, proven test harness. Every integration point is verified against the real code: the exact
dependency signature (`ResolveRoleTimeout(role string, cfg Config) time.Duration`, LANDED), the proof that the
change is behavior-preserving by default (`ResolveRoleTimeout("message", cfg) == cfg.Timeout` when no override
— no message built-in exists), the three edit sites anchored by STRING (with the drifted line numbers
corrected 287/335/423-426 → 264/335/426), the confirmation that `config` + `time` are already imported (no
import edit), the existing test that stays green and why, the new test that proves the wiring (flipping which
field carries the small timeout), and the S1/S2 scope boundary (Run call + multiturn.go/workdesc.go are S2;
ResolveRoleTimeout is deterministic so the budget display and Run's per-turn bound agree). The -1 from 10/10
reflects the one subtlety the implementer must honor: the Run call at :436 is INTENTIONALLY left unchanged
(contract "(see S2)") — an implementer who "helpfully" threads msgTimeout into Run would cross the S1/S2
boundary and create a signature-coordination hazard with the parallel sibling. The grep guards (esp. 4 + 5)
catch that over-reach.
