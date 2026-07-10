name: "P1.M2.T2.S2 — Wire watchdog arming in default_action.go, gated by no_parent_watchdog (FR-K1/K2/K6/K7)"
description: >
  The CONSUMER wiring for the parent-death watchdog. Modify EXACTLY ONE existing file — internal/cmd/default_action.go —
  to arm `watchdog.Arm(ctx, 1*time.Second)` immediately after the run lock is acquired (`defer locker.Release()` site),
  gated by `!cfg.NoParentWatchdog` (the FR-K6 opt-out). This depends on two parallel siblings landing as specified:
  `watchdog.Arm(ctx, interval)` from P1.M2.T2.S1 and `Config.NoParentWatchdog bool` from P1.M2.T1.S1. The ctx passed
  (`cmd.Context()`) is ALREADY the signal-aware ctx produced by `signal.Install` in main.go → threaded through
  `cmd.Execute` → `rootCmd.SetContext` → `cmd.Context()`, so the poll goroutine dies with the process and, on parent
  death, `watchdog` calls `signal.Trigger(SIGTERM)` which reuses the SAME rescue + `OnRescueExit`(=lock.ReleaseCurrent)
  exit path a terminal SIGTERM takes — NO separate teardown, NO new `internal/lock` import in default_action.go. One
  arming covers BOTH the single-commit and the decompose paths (runDecompose runs under the same lock). Windows is a
  no-op inside the watchdog (FR-K7); the 1s poll cadence is FR-K2. THE CHANGE = 2 import lines + 1 gated arming block
  with a Mode-A comment citing FR-K1/K6. NO new files, NO new tests (contract: "mock nothing — test via the e2e harness",
  P1.M4.T1.S1), NO CLI flag, NO config change, NO docs change (P1.M4.T2 owns the docs sync).

---

## Goal

**Feature Goal**: Make a stagecoach run that holds the per-repo run lock ALSO arm the parent-death watchdog (unless the
user opted out via `no_parent_watchdog`), so that when the launcher (lazygit TUI, IDE window, detaching terminal) dies
without sending SIGINT/SIGTERM (§18.5's "closed without killing it" gap), the watchdog detects the reparent, routes
through `signal.Trigger(SIGTERM)`, and the lock file is released before exit instead of being orphaned indefinitely.

**Deliverable**: A 3-edit modification to `internal/cmd/default_action.go` (and ONLY that file):
1. Add `"time"` to the stdlib import group.
2. Add `"github.com/dustin/stagecoach/internal/watchdog"` to the stagecoach import group.
3. Immediately after `defer locker.Release()` (the run-lock acquire site, ~line 79), add a Mode-A-commented block:
   `if !cfg.NoParentWatchdog { watchdog.Arm(ctx, 1*time.Second) }`.

**Success Definition**:
- `go build ./...`, `GOOS=windows go build ./...`, `GOOS=linux go build ./...` all clean.
- `gofmt -l internal/cmd/default_action.go` empty; `go vet ./internal/cmd/...` clean; `make lint` clean.
- `make test` (race) stays GREEN — the existing ~30 `TestRunDefault_*` / routing tests are the regression net; the
  arming is inert under `Execute(context.Background())` (stable ppid + nil `signal.Active()` ⇒ never fires).
- Grep guards pass: exactly ONE production `watchdog.Arm` caller in `cmd/`; it reads `cfg.NoParentWatchdog`; it passes
  `ctx` (the `cmd.Context()` from line 37); it sits AFTER `defer locker.Release()`; NO new `internal/lock` import.
- NO behavioral change is observable from unit tests (the watchdog only fires on a real parent-pid CHANGE, which the
  e2e harness — P1.M4.T1.S1 — produces by launching real stagecoach subprocesses). The orphaned-lock e2e scenarios are
  P1.M4.T1.S1; this task only LANDS the wiring that makes them possible.

## User Persona (if applicable)

**Target User**: A developer who launches stagecoach from a short-lived or fragile parent — closing the lazygit TUI,
quitting an IDE window, or a detaching terminal — the orphaned-run scenarios in §9.27 / §18.5.

**Use Case**: The launcher exits without sending SIGINT/SIGTERM (the §18.5 bug case). The run now arms the watchdog at
lock-acquire; the watchdog detects the reparent and tears the run down cleanly (rescue message if mid-generation, lock
released) instead of orphaning the lock file and blocking every future run.

**User Journey**: `stagecoach` launched by lazygit → user quits lazygit → kernel reparents stagecoach → watchdog poll
sees the ppid change within ~1s → `signal.Trigger(SIGTERM)` → `handle()` forwards to the child group, cancels the ctx,
runs rescue-or-exit, and `OnRescueExit`=lock.ReleaseCurrent removes the lock file before `os.Exit`. Next run is unblocked.
(For intentional detach — `nohup`/`setsid`/`systemd-run` — the user sets `no_parent_watchdog` via
`STAGECOACH_NO_PARENT_WATCHDOG` / `stagecoach.noParentWatchdog` / `[generation] no_parent_watchdog`, P1.M2.T1.S1, and the
arming is skipped.)

**Pain Points Addressed**: FR-K1/K2 — the §18.5 "launcher closed without killing it" lock-orphan gap; FR-K6 — the escape
hatch for intentional detach.

## Why

- **FR-K1/K2/K6 / §9.27**: the watchdog PACKAGE (P1.M2.T2.S1) and the config GATE (P1.M2.T1.S1) are inert until a
  consumer actually calls `watchdog.Arm` after lock acquire, gated by the opt-out. This subtask is that consumer — the
  one wiring that turns the feature on for the default action (the only path that acquires the write lock).
- **Reuses the single rescue path**: by passing the signal-aware `ctx` and letting the watchdog call `signal.Trigger`,
  we get forward-to-child-group + cancel ctx + rescue/exit + `OnRescueExit`(lock release) for FREE — no duplicated
  teardown, no new `internal/lock` import, no rescue-message printing in the CLI layer.
- **Bounded, zero-conflict scope**: one file, three edits, no new files/tests/deps. The parallel siblings (watchdog pkg,
  config field) and the signal export (P1.M1.T2.S1) are treated as contracts; this task only adds the call site.

## What

**User-visible behavior**: Once the parallel siblings land, a `stagecoach` run (default action) that acquires the lock
also arms the parent-death watchdog. If the launcher dies, the run tears down via rescue + lock release within ~1s. If
`no_parent_watchdog` is set, the run does NOT self-teardown on launcher exit (intentional detach is respected). Windows
runs arm the no-op watchdog (FR-K7) — harmless.

**Technical change**: 3 edits to `internal/cmd/default_action.go`. See the Implementation Blueprint for verbatim
before/after + exact anchor text.

### Success Criteria
- [ ] `internal/cmd/default_action.go` imports `"time"` and `"github.com/dustin/stagecoach/internal/watchdog"`.
- [ ] Immediately after `defer locker.Release()`, an `if !cfg.NoParentWatchdog { watchdog.Arm(ctx, 1*time.Second) }`
      block exists, preceded by a Mode-A comment explaining the FR-K1/K6 gating and that it rides the signal ctx + the
      `OnRescueExit` lock-release seam.
- [ ] The block passes `ctx` (= `cmd.Context()` from line 37, the signal-aware ctx) — NOT a fresh `context.Background()`.
- [ ] The interval is exactly `1*time.Second` (FR-K2 default cadence).
- [ ] NO second arming is added in `runDecompose` (one arming covers both paths — the comment must say so).
- [ ] NO new `internal/lock` import in default_action.go (the seam is `signal.Trigger` → `OnRescueExit`).
- [ ] NO new file, NO new test, NO CLI flag, NO config change, NO docs change.
- [ ] `go build ./...` + `GOOS=windows` + `GOOS=linux` clean; `gofmt -l` empty; `go vet ./internal/cmd/...` clean.
- [ ] `make test` (race) green; `make lint` clean.
- [ ] Grep guards (§Validation Loop Level 4) all pass.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — verbatim before/after for all 3 edits with the EXACT anchor text (the import block + the lock-acquire block),
the proof that `ctx` is already signal-aware (main.go → Execute → SetContext → cmd.Context, with the stale inline comment
flagged), the exact signatures of the two parallel-sibling contracts consumed (`watchdog.Arm(ctx, interval)` and
`cfg.NoParentWatchdog`), the explanation of why NO new `internal/lock` import is needed (the `OnRescueExit`=lock.ReleaseCurrent
seam, wired in main.go), the rationale for adding NO unit test (contract: e2e only; the arming is inert under
`Execute(context.Background())`), and 6 grep guards that prove correct placement.

### Documentation & References

```yaml
# MUST READ — the authoritative consolidated findings (line numbers, signatures, the stale-comment proof, grep guards)
- docfile: plan/014_37208f58ffa2/P1M2T2S2/research/findings.md
  why: "§1 proves the ctx is ALREADY signal-aware (main.go→Execute→SetContext→cmd.Context) + flags the STALE inline
        comment at default_action.go:38; §2 gives the exact insertion site + line numbers; §3 the exact import edits;
        §4 the OnRescueExit seam (NO internal/lock import); §5 the no-test rationale; §7 the 6 grep guards."
  critical: "§1 + §2: the ONLY file to edit is internal/cmd/default_action.go; the ONLY site is between
             `defer locker.Release()` and the `// ---- §9.4 auto-stage-all…` comment; `ctx` and `cfg` are ALREADY in
             scope — do not redeclare either."

# MUST READ — the watchdog package API this task consumes (TREAT AS A CONTRACT; lands in parallel as P1.M2.T2.S1)
- docfile: plan/014_37208f58ffa2/P1M2T2S1/PRP.md
  why: "Defines `func Arm(ctx context.Context, interval time.Duration)` (best-effort, no error return; nil-safe when
        signal.Active()==nil; Windows no-op; the poll goroutine exits on ctx cancel). Confirms the watchdog's ONLY
        effect is signal.Trigger(SIGTERM) — it does NOT import internal/lock or print rescue messages."
  critical: "Arm has NO error return — do NOT wrap it in `if err :=`. interval<=0 internally defaults to 1s, but pass
             1*time.Second explicitly (the contract value, FR-K2). There is NO need to call watchdog.Stop() on the CLI
             path — the goroutine dies with the process (ctx = signal-aware ctx; Trigger → os.Exit)."

# MUST READ — the config gate field this task reads (TREAT AS A CONTRACT; lands in parallel as P1.M2.T1.S1)
- docfile: plan/014_37208f58ffa2/P1M2T1S1/PRP.md
  why: "Adds `NoParentWatchdog bool` to Config (default false). Confirms the field is resolved via the 7-layer
        precedence (env STAGECOACH_NO_PARENT_WATCHDOG / git stagecoach.noParentWatchdog / [generation]
        no_parent_watchdog; NO CLI flag). This task is its FIRST production reader."
  critical: "`cfg` in runDefault is *config.Config (cfg := Config() at line 41), so `cfg.NoParentWatchdog` is the correct
             access form. Do NOT add the field here — it is the sibling's job; this task only READS it."

# MUST READ — the file being edited (the lock-acquire site + the import block + the ctx line)
- file: internal/cmd/default_action.go
  why: "runDefault: `ctx := cmd.Context()` (line 37) — the signal-aware ctx. lock.Acquire + `defer locker.Release()`
        (lines 71–79) — the insertion anchor. The import block (lines 3–21) — the import edit anchor. `cfg := Config()`
        (line 41) — *config.Config in scope."
  pattern: "The lock comment at lines 67–70 already states one acquire+defer covers BOTH the single-commit and decompose
            paths; the watchdog arming inherits this — place it ONCE, in runDefault, never in runDecompose."
  gotcha: "The inline comment on line 37 says 'P1.M4.T2 swaps it for a signal-aware ctx later' — that is STALE; main.go
           already passes the signal.Install ctx. Do not 'fix' main.go (out of scope); just use ctx as-is (it IS
           signal-aware). Optionally refresh the stale comment while you're here (low-risk, Mode-A doc)."

# CONTEXT — the ctx source + the OnRescueExit seam (READ-ONLY; proves ctx is signal-aware + lock release is free)
- file: cmd/stagecoach/main.go
  why: "signal.Install(context.Background(), Options{… OnRescueExit: lock.ReleaseCurrent …}) → ctx → cmd.Execute(ctx).
        Proves the watchdog's ctx is signal-aware AND that OnRescueExit=lock.ReleaseCurrent releases the lock before
        os.Exit on BOTH rescue(3) and pre-snapshot(143) paths — so the consumer needs NO internal/lock import."
  critical: "Do NOT edit main.go. It already wires everything the watchdog needs."

# CONTEXT — signal.Trigger (the rescue path the watchdog rides; READ-ONLY)
- file: internal/signal/signal.go
  why: "Trigger (lines 156–164): nil-safe (active==nil ⇒ no-op) + stopped-guarded (no-op after RestoreDefault).
        handle (lines 173–203): forward-to-group → cancel ctx → rescue(3)/exit(129/130/143) + OnRescueExit on BOTH
        branches. Confirms ONE rescue path, no duplicated teardown."
  gotcha: "Trigger is nil-safe, so the arming is harmless in unit tests (signal.Active()==nil there). Never call os.Exit
           or lock.Release* from the CLI arming site — Trigger + OnRescueExit already do it."

# CONTEXT — the FR-K1/K2/K6/K7 design (the arming point + the opt-out)
- docfile: plan/014_37208f58ffa2/architecture/watchdog_config.md
  why: "'Arming point: default_action.go post-Acquire' shows the exact call + gating; 'FR-K6' documents the opt-out."
  critical: "Confirms the arming is gated by `!cfg.NoParentWatchdog` and placed after lock.Acquire succeeds."
```

### Current Codebase tree (relevant slice)

```bash
cmd/stagecoach/main.go          # READ-ONLY — signal.Install(ctx, {OnRescueExit: lock.ReleaseCurrent}) → cmd.Execute(ctx)
internal/cmd/
  root.go                       # READ-ONLY — Execute(ctx) does rootCmd.SetContext(ctx) (lines 315–320)
  default_action.go             # EDIT (this task) — +2 imports, +1 arming block after `defer locker.Release()`
  default_action_test.go        # READ-ONLY — regression net (Execute(context.Background())); no new test added
internal/signal/signal.go       # READ-ONLY — Trigger(nil-safe)+handle(); OnRescueExit seam (already wired in main.go)
internal/lock/lock.go           # READ-ONLY — ReleaseCurrent (line 211) = the OnRescueExit fn; NOT imported by this edit
# internal/watchdog/            # P1.M2.T2.S1 (parallel) — provides watchdog.Arm(ctx, interval). NOT created here.
# internal/config/ NoParentWatchdog field  # P1.M2.T1.S1 (parallel) — provides cfg.NoParentWatchdog. NOT added here.
go.mod                          # READ-ONLY — NO new dep (stdlib time + the already-planned internal/watchdog import)
Makefile                        # test=line 70 (-race); lint=line 103; build=line 52; coverage-gate=line 77 (NOT cmd)
```

### Desired Codebase tree with files to be added/modified

```bash
# MODIFIED (the ONLY file this task touches):
internal/cmd/default_action.go   # +import "time"; +import internal/watchdog; +gated watchdog.Arm block after lock acquire
# (NO new files. NO new tests. NO other modifications.)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (ctx is ALREADY signal-aware — do not redeclare or re-derive it): runDefault line 37 does `ctx := cmd.Context()`.
// That ctx is the signal.Install ctx from main.go (main.go: signal.Install(ctx) → cmd.Execute(ctx) → rootCmd.SetContext(ctx)
// → cmd.Context()). The inline comment claiming "P1.M4.T2 swaps it for a signal-aware ctx later" is STALE. Use this ctx
// verbatim — the watchdog's poll goroutine dies when the process exits, and on parent death Trigger→handle→os.Exit kills it.

// CRITICAL (do NOT add a second arming in runDecompose): the lock is acquired ONCE in runDefault (lines 71–79) and
// `defer locker.Release()` covers BOTH the single-commit path and the decompose path (runDecompose is called below, under
// the same lock — see the lock comment at lines 67–70). The watchdog arming inherits this coverage. Arming a SECOND time in
// runDecompose would spawn a duplicate goroutine and is WRONG. State this in the arming-site comment.

// CRITICAL (do NOT import internal/lock for the rescue path): the watchdog's ONLY effect is signal.Trigger(SIGTERM), which
// reuses handle() → OnRescueExit (= lock.ReleaseCurrent, wired in main.go) to release the lock before os.Exit. default_action.go
// ALREADY imports internal/lock for lock.Acquire/HeldError (pre-existing) — do NOT add lock-release calls for the watchdog.
// The grep guard `grep -c 'internal/lock' default_action.go` must stay at 1 (the pre-existing import).

// CRITICAL (watchdog.Arm has NO error return): `func Arm(ctx context.Context, interval time.Duration)` returns nothing.
// Do NOT write `if err := watchdog.Arm(...); err != nil`. Best-effort by design (the prctl syscall + poll are best-effort;
// the reliable path is the getppid poll). Just call it.

// CRITICAL (interval is a VALUE, pass 1*time.Second explicitly): the contract pins the default poll cadence to 1s (FR-K2).
// watchdog.Arm internally defaults interval<=0 to 1s, but the contract value is `1*time.Second` — pass it explicitly so the
// cadence is documented at the call site and grep-findable.

// GOTCHA (add NO unit test): the item contract says "mock nothing — test via the e2e harness (P1.M4.T1.S1)". The arming is
// inert in unit tests anyway: default_action_test.go calls Execute(context.Background()) (stable ppid ⇒ the poll never fires;
// signal.Active()==nil ⇒ Trigger is a no-op). A gated "armed/not-armed" unit test would require mocking the watchdog, which
// the contract forbids. The behavioral proof is a real-subprocess e2e (P1.M4.T1.S1). This task's proof = build/lint/gofmt +
// full regression suite green + grep guards.

// GOTCHA (harmless goroutine accumulation in the cmd test binary is EXPECTED and SAFE): each TestRunDefault_* that reaches
// the lock arms a 1s-poll goroutine on context.Background() (the test's ctx, never canceled). They never fire (stable ppid)
// and die at process exit; no -race issue (no shared mutable state); Go has no built-in leak detector. make test stays green.

// GOTCHA (import placement — let gofmt finalize, but place correctly to stay gofmt-clean): add "time" right after "strings"
// in the stdlib group, and internal/watchdog after internal/ui (before pkg/stagecoach) in the stagecoach group.
```

## Implementation Blueprint

### Data models and structure

None. This task adds no types, no fields, no new packages. It consumes two already-specified symbols
(`watchdog.Arm`, `cfg.NoParentWatchdog`) at one call site.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/cmd/default_action.go — add the "time" import
  - OLD (the tail of the stdlib import group):
        \t"os"
        \t"strings"

        \t"github.com/spf13/cobra"
  - NEW:
        \t"os"
        \t"strings"
        \t"time"

        \t"github.com/spf13/cobra"
  - ANCHOR: the exact two-line `\t"os"\n\t"strings"` block immediately before the blank line + cobra import.
  - GOTCHA: place "time" AFTER "strings" (alphabetical). gofmt will keep the block sorted.

Task 2: EDIT internal/cmd/default_action.go — add the internal/watchdog import
  - OLD (the tail of the stagecoach import group):
        \t"github.com/dustin/stagecoach/internal/ui"
        \t"github.com/dustin/stagecoach/pkg/stagecoach"
  - NEW:
        \t"github.com/dustin/stagecoach/internal/ui"
        \t"github.com/dustin/stagecoach/internal/watchdog"
        \t"github.com/dustin/stagecoach/pkg/stagecoach"
  - ANCHOR: the exact `\t"github.com/dustin/stagecoach/internal/ui"` line followed by the pkg/stagecoach line.
  - GOTCHA: internal/watchdog sorts after internal/ui and before pkg/stagecoach in the contiguous stagecoach block.
  - NOTE: this import is what makes the watchdog.Arm symbol available; it depends on P1.M2.T2.S1 having created the package.

Task 3: EDIT internal/cmd/default_action.go — the gated arming block (THE core edit)
  - OLD (the lock-acquire site):
        \treturn exitcode.New(exitcode.Error, fmt.Errorf("acquire run lock: %w", lockErr))
        }
        \tdefer locker.Release()

        \t// ---- §9.4 auto-stage-all state machine (FR16–FR20) ----
  - NEW:
        \treturn exitcode.New(exitcode.Error, fmt.Errorf("acquire run lock: %w", lockErr))
        }
        \tdefer locker.Release()

        \t// §9.27 FR-K1/K6 — parent-death watchdog. Now that THIS process owns the run lock, arm the watchdog so that
        \t// when the launcher dies without sending a signal (closing the lazygit TUI, quitting an IDE, a detaching
        \t// terminal — §18.5's "closed without killing it" case), it reclaims the lock instead of orphaning it. Gated
        \t// by the FR-K6 opt-out (NoParentWatchdog) for intentional detach (nohup/setsid/systemd-run). The watchdog
        \t// shares this ctx (cmd.Context → main.go's signal.Install ctx), so its poll goroutine dies with the process;
        \t// on a parent-pid change it calls signal.Trigger(SIGTERM), reusing the SAME rescue + OnRescueExit
        \t// (=lock.ReleaseCurrent) exit path a terminal SIGTERM takes — no separate teardown, no internal/lock import.
        \t// One arming covers BOTH the single-commit path and runDecompose (runDecompose runs under this same lock),
        \t// so do NOT re-arm inside runDecompose. Windows is a no-op inside the watchdog (FR-K7); 1s cadence is FR-K2.
        \tif !cfg.NoParentWatchdog {
        \t\twatchdog.Arm(ctx, 1*time.Second)
        \t}

        \t// ---- §9.4 auto-stage-all state machine (FR16–FR20) ----
  - ANCHOR: the unique `defer locker.Release()` line followed by the blank line + the §9.4 comment.
  - GOTCHA: `ctx` and `cfg` are ALREADY in scope (lines 38 and 46) — do not redeclare. Pass `ctx`, NOT context.Background().
  - GOTCHA: `watchdog.Arm` returns NOTHING — no error handling wrapper.
  - FOLLOW pattern: the lock comment at lines 67–70 (the "one acquire covers both paths" reasoning) for the arming comment's tone.
  - NAMING: `watchdog.Arm`, `cfg.NoParentWatchdog`, `ctx`, `1*time.Second` — all exactly as the contracts specify.
  - PLACEMENT: immediately after `defer locker.Release()`, before the §9.4 auto-stage comment (so the watchdog is armed
    for EVERY write path that holds the lock, including the early `hasStaged`/AddAll/decompose branches below).

Task 4 (OPTIONAL, low-risk Mode-A doc polish): refresh the STALE ctx comment at line 37
  - OLD:
        \tctx := cmd.Context() // S1's Execute set this; P1.M4.T2 swaps it for a signal-aware ctx later.
  - NEW:
        \tctx := cmd.Context() // the signal-aware ctx (main.go's signal.Install → cmd.Execute → rootCmd.SetContext).
  - This is OPTIONAL; it does not change behavior. It corrects the stale note now that the signal ctx is live and is the
    very ctx the watchdog depends on. If you prefer a minimal diff, skip this — but it improves the Mode-A doc accuracy.
  - GOTCHA: do NOT also touch the "P1.M4.T2" reference elsewhere; this is a one-line comment refresh only.

Task 5: VERIFY — build (native+cross), vet, format, full regression, lint, grep guards
  - go build ./... ; GOOS=windows go build ./... ; GOOS=linux go build ./...
  - go vet ./internal/cmd/...
  - gofmt -l internal/cmd/default_action.go   # must be empty
  - go test ./internal/cmd/ -v                 # regression (no new tests; all pre-existing must stay green)
  - make test ; make lint ; make build
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the gated arming (the entire feature, in 3 lines). Gated by the parallel-sibling config field; passes the
// already-signal-aware ctx; best-effort Arm with the contract's 1s cadence.
if !cfg.NoParentWatchdog {
    watchdog.Arm(ctx, 1*time.Second)
}

// PATTERN: why no teardown/Stop is needed on the CLI path. ctx = the signal.Install ctx from main.go. When the process
// exits normally, the goroutine dies with it. When the watchdog fires (parent death), it calls signal.Trigger(SIGTERM) →
// handle() → h.cancel() (cancels this ctx → the watchdog's internal armCtx cancels → poll goroutine exits) → OnRescueExit
// (lock.ReleaseCurrent) → os.Exit. There is no path where the poll goroutine outlives a CLI run. (watchdog.Stop() exists
// only for library use of pkg/stagecoach without signal.Install — not the CLI's concern.)

// PATTERN: why no internal/lock import/call. The rescue path's lock release is the OnRescueExit seam (lock.ReleaseCurrent),
// wired in main.go and invoked by signal.handle() on BOTH the rescue(3) and pre-snapshot(129/130/143) branches. The
// watchdog reaches it via signal.Trigger → handle. The CLI arming site does NOT touch the lock for teardown.
```

### Integration Points

```yaml
IMPORTS (internal/cmd/default_action.go):
  - ADD "time" (stdlib group)
  - ADD "github.com/dustin/stagecoach/internal/watchdog" (stagecoach group)
  - (internal/lock is ALREADY imported pre-existing for lock.Acquire/HeldError — do NOT remove or duplicate it)

CALL SITE (internal/cmd/default_action.go runDefault):
  - ANCHOR: immediately after `defer locker.Release()` (~line 79), before the `// ---- §9.4 auto-stage-all…` comment.
  - GATE: `if !cfg.NoParentWatchdog {`  (cfg is *config.Config from line 41; the field is P1.M2.T1.S1)
  - CALL:  `watchdog.Arm(ctx, 1*time.Second)`  (ctx is cmd.Context() from line 37; watchdog.Arm is P1.M2.T2.S1)

NO database / migration / routes / new types / new flag / new file / new test / config change / docs change.
  - The feature's external docs (README, docs/) sync is P1.M4.T2.S1.
  - The feature's e2e scenarios (real-subprocess orphan reclaim) are P1.M4.T1.S1.
  - The feature's config field is P1.M2.T1.S1 (parallel sibling).
  - The feature's watchdog package is P1.M2.T2.S1 (parallel sibling).
  - This task is SOLELY the consumer wiring — the one call site that turns the feature on for the default action.

SCOPE FENCES:
  - Modifies ONLY internal/cmd/default_action.go (2 imports + 1 arming block; +1 optional comment refresh).
  - Does NOT create internal/watchdog/* (P1.M2.T2.S1), add Config.NoParentWatchdog (P1.M2.T1.S1), edit internal/signal/*
    (Trigger already exported by P1.M1.T2.S1), edit cmd/stagecoach/main.go, edit any test file, or add a CLI flag.
  - Adds NO third-party dependency (go.mod unchanged).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Native + cross build (the new import + call must compile on every platform; the watchdog's Windows file is a no-op).
go build ./...
GOOS=linux   go build ./...
GOOS=windows go build ./...
# Expected: all clean. If GOOS=windows fails, the arming somehow referenced a Unix-only symbol — it must not (Arm is
#           the cross-platform entry; arm_windows.go is the no-op).

# Vet.
go vet ./internal/cmd/...
# Expected: clean.

# Format — the edited file must be gofmt-clean (import placement + the arming block indentation).
gofmt -l internal/cmd/default_action.go
# Expected: empty. If listed: gofmt -w internal/cmd/default_action.go.

# Lint.
make lint      # golangci-lint (staticcheck/gosimple/govet/errcheck/ineffassign/unused)
# Expected: zero errors. `unused` could fire only if watchdog.Arm were imported-but-uncalled — it IS called, so clean.

# Scope guard: ONLY internal/cmd/default_action.go changed (or + the optional comment refresh on the same file).
git status --porcelain
# Expected: exactly 1 modified file: internal/cmd/default_action.go. ZERO new files. ZERO changes elsewhere.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Regression: the cmd package's existing tests are the net. The arming is inert under Execute(context.Background()).
go test ./internal/cmd/ -v
# Expected: ALL pre-existing TestRunDefault_* / TestRouting_* / TestShouldDecompose / TestHandleDecomposeError tests PASS.
#           (The watchdog poll never fires: stable ppid + signal.Active()==nil ⇒ signal.Trigger is a no-op.)

# Full race suite (the arming spawns a poll goroutine; -race must stay clean — no shared mutable state).
make test
# Expected: green (race detector). See Known Gotchas for the harmless goroutine-accumulation note.

# NOTE: NO new test is added (contract: "mock nothing — test via the e2e harness", P1.M4.T1.S1). make coverage-gate
# (line 77) gates ONLY internal/{git,provider,generate,config} — NOT internal/cmd — so the no-test decision does not
# threaten the coverage gate.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary (the new import links into the binary; proves no import cycle / link error).
make build

# Manual sanity: a default-action run that acquires the lock now arms the watchdog (no observable behavior change in a
# normal run — the watchdog only fires on a real parent-pid CHANGE). Use --dry-run so no commit lands; it still reaches
# the lock-acquire + arming site.
d=$(mktemp -d) && cd "$d" && git init -q
git config user.email t@t.com && git config user.name t
printf 'a\n' > f.txt && git add f.txt && git commit -qm init
printf 'b\n' >> f.txt && git add f.txt
SC=/home/dustin/projects/stagecoach/bin/stagecoach
"$SC" --dry-run 2>&1 | head -3   # runs to completion (exit 0); the watchdog was armed and died harmlessly at process exit
# Confirm the opt-out resolves and the run still completes (no behavior change either way at the unit level):
STAGECOACH_NO_PARENT_WATCHDOG=1 "$SC" --dry-run 2>&1 | head -1
cd - && rm -rf "$d"

# Expected: both runs complete normally. There is NO observable difference from the unit/integration level because the
# watchdog only acts on a real parent death — that scenario is the e2e harness (P1.M4.T1.S1), which launches real
# stagecoach subprocesses, kills the launcher, and asserts the lock is released + the process exits via rescue.
```

> **Note**: this subtask is pure consumer wiring with no unit-testable behavior of its own (mocking is forbidden by the
> contract). The within-scope proof is: clean build/vet/lint/gofmt + the full regression suite green + the grep guards.
> The end-to-end "launcher dies → watchdog fires → lock released + rescue exit" scenario is P1.M4.T1.S1 (it needs this
> wiring PLUS the parallel siblings PLUS real subprocesses). This task LANDS the wiring that makes that e2e possible.

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: exactly ONE production watchdog.Arm caller in cmd/, gated by cfg.NoParentWatchdog.
grep -rn 'watchdog.Arm' --include='*.go' internal/ cmd/ pkg/
# Expected: default_action.go (1 hit) + internal/watchdog/*_test.go (the P1.M2.T2.S1 tests). cmd/ has exactly 1.

# Guard 2: the gate reads the parallel-sibling field (not a flag).
grep -n 'cfg.NoParentWatchdog' internal/cmd/default_action.go
# Expect: 1 hit — the `if !cfg.NoParentWatchdog {` line.

# Guard 3: the ctx passed is cmd.Context() (the signal-aware ctx), NOT a fresh context.
grep -n 'watchdog.Arm(ctx' internal/cmd/default_action.go
# Expect: 1 hit using `ctx` (declared at line 37 as `ctx := cmd.Context()`).

# Guard 4: the arming is AFTER the lock is held (after `defer locker.Release()`).
grep -n -A3 'defer locker.Release()' internal/cmd/default_action.go
# Expect: the gated arming block (`if !cfg.NoParentWatchdog { watchdog.Arm(ctx, 1*time.Second) }`) appears within the
#         next few lines.

# Guard 5: imports added: time + internal/watchdog.
grep -n '"time"' internal/cmd/default_action.go
grep -n 'internal/watchdog"' internal/cmd/default_action.go
# Expect: 1 hit each.

# Guard 6: NO extra internal/lock import (the rescue path rides the OnRescueExit seam, not a direct lock call).
grep -c 'internal/lock"' internal/cmd/default_action.go
# Expect: 1 (the PRE-EXISTING import for lock.Acquire/HeldError). NOT 2.

# Guard 7: NO second arming in runDecompose (one arming covers both paths).
grep -n 'watchdog.Arm' internal/cmd/default_action.go
# Expect: exactly 1 hit total (in runDefault). Confirm runDecompose has none.

# Guard 8: the 1s cadence is explicit (FR-K2).
grep -n '1\*time.Second' internal/cmd/default_action.go
# Expect: 1 hit inside the arming block.

# Guard 9: scope — only one file changed.
git status --porcelain
# Expect: 1 file (internal/cmd/default_action.go). No new files, no other modifications.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + `GOOS=linux go build ./...` + `GOOS=windows go build ./...` clean
- [ ] `go vet ./internal/cmd/...` clean
- [ ] `gofmt -l internal/cmd/default_action.go` empty
- [ ] `make lint` zero errors (watchdog.Arm is imported AND called ⇒ no `unused`)
- [ ] `make test` (race) green — the full existing cmd regression suite passes (arming is inert under `context.Background()`)

### Feature Validation
- [ ] `default_action.go` imports `"time"` and `"github.com/dustin/stagecoach/internal/watchdog"`
- [ ] Immediately after `defer locker.Release()`, the block `if !cfg.NoParentWatchdog { watchdog.Arm(ctx, 1*time.Second) }`
      exists with a Mode-A comment citing FR-K1/K6 and noting it rides the signal ctx + OnRescueExit seam + one-arming-covers-both-paths
- [ ] The block passes `ctx` (= `cmd.Context()`, the signal-aware ctx) — NOT `context.Background()`
- [ ] The interval is exactly `1*time.Second` (FR-K2)
- [ ] NO second arming in `runDecompose`
- [ ] NO new `internal/lock` import / lock-release call for the watchdog path (the seam is signal.Trigger → OnRescueExit)
- [ ] NO new unit test (contract: e2e only, P1.M4.T1.S1)

### Scope-Boundary Validation
- [ ] `git status` shows ONLY `internal/cmd/default_action.go` modified (1 file; + the optional 1-line comment refresh on the same file)
- [ ] NO edit to `internal/watchdog/*` (P1.M2.T2.S1), `internal/config/*` (P1.M2.T1.S1), `internal/signal/*` (P1.M1.T2.S1),
      `internal/lock/*`, `cmd/stagecoach/main.go`, or any test file
- [ ] NO new CLI flag, NO new file, NO new third-party dependency (go.mod unchanged)
- [ ] Grep guards 1–9 (Level 4) all pass

### Code Quality & Docs
- [ ] The arming-site comment cites §9.27 FR-K1/K6, explains the gating, the signal-ctx reuse, the OnRescueExit lock-release seam,
      the one-arming-covers-both-paths property, the Windows no-op (FR-K7), and the 1s cadence (FR-K2) — PRD Mode A
- [ ] Imports placed gofmt-cleanly (`time` after `strings`; `internal/watchdog` after `internal/ui`)
- [ ] `watchdog.Arm` called bare (no error wrapper — it returns nothing)

---

## Anti-Patterns to Avoid

- ❌ Don't pass `context.Background()` (or a freshly-derived ctx) to `watchdog.Arm`. Use the `ctx` already in scope at
  `runDefault` line 37 (`ctx := cmd.Context()`) — that IS the signal-aware ctx from main.go's `signal.Install`. A fresh
  ctx would mean the poll goroutine never dies on rescue/cancel and the watchdog's `signal.Trigger` wouldn't be tied to
  the run's lifecycle.
- ❌ Don't redeclare `ctx` or `cfg`. Both are already in scope in `runDefault` (`ctx := cmd.Context()` line 37;
  `cfg := Config()` line 41). Just reference them.
- ❌ Don't add a second `watchdog.Arm` call in `runDecompose`. The lock is acquired ONCE in `runDefault`; `runDecompose`
  runs under that same lock (see the lock comment at lines 67–70). One arming covers both the single-commit and the
  decompose paths. A second arming spawns a duplicate goroutine.
- ❌ Don't wrap `watchdog.Arm(...)` in `if err := …`. It returns NOTHING (best-effort by design — the prctl syscall and
  the getppid poll are both best-effort; the reliable detector is the poll). Calling it bare is correct.
- ❌ Don't import `internal/lock` for the rescue path or call `locker.Release()`/`lock.ReleaseCurrent()` from the arming
  site. The watchdog calls `signal.Trigger(SIGTERM)` → `handle()` → `OnRescueExit` (= `lock.ReleaseCurrent`, wired in
  main.go) → releases the lock before `os.Exit` skips the deferred `locker.Release()`. That's the SAME path a terminal
  SIGTERM takes. Re-teardown here is redundant and wrong.
- ❌ Don't call `os.Exit`, `signal.Trigger`, or print a rescue message from the CLI arming site. The watchdog owns all of
  that internally (it is the watchdog's entire job). The consumer's ONLY action is `watchdog.Arm(ctx, 1*time.Second)`.
- ❌ Don't add a unit test for the gate ("armed when false, not armed when true"). The item contract says "mock nothing —
  test via the e2e harness (P1.M4.T1.S1)". Such a unit test would require mocking the watchdog package, which is
  forbidden. The arming is inert in unit tests anyway (stable ppid + nil signal.Active()).
- ❌ Don't edit `cmd/stagecoach/main.go`, `internal/signal/*`, `internal/lock/*`, `internal/config/*`, or
  `internal/watchdog/*`. Those are the parallel siblings' / prior tasks' scope. This task edits ONE file:
  `internal/cmd/default_action.go`.
- ❌ Don't add a `--no-parent-watchdog` CLI flag or a `loadFlags` entry. FR-K6 has NO flag (env + git-config + file only);
  the field is the P1.M2.T1.S1 sibling's job, and this task only READS `cfg.NoParentWatchdog`.
- ❌ Don't change the interval from `1*time.Second`. The contract pins the default poll cadence (FR-K2). Even though
  `watchdog.Arm` internally defaults a non-positive interval to 1s, pass `1*time.Second` explicitly so the cadence is
  documented and grep-findable at the call site.
- ❌ Don't "fix" the stale inline comment at line 37 by editing main.go or root.go. The ctx IS already signal-aware
  (main.go wires it). At most, optionally refresh the one-line comment in default_action.go (Task 4) — a doc-only,
  in-scope change. main.go and root.go are READ-ONLY for this task.

---

## Confidence Score: 9/10

This is a surgical 3-edit change to a single existing file, with every integration point verified against the real code:
the exact import block and lock-acquire site (verbatim, with line numbers), the proof that `ctx` is ALREADY the
signal-aware ctx (main.go → `cmd.Execute` → `rootCmd.SetContext` → `cmd.Context()`), the stale-comment flag, the two
parallel-sibling contracts consumed (`watchdog.Arm(ctx, interval)` and `cfg.NoParentWatchdog`), the reason NO new
`internal/lock` import is needed (the `OnRescueExit`=lock.ReleaseCurrent seam wired in main.go), the reason NO unit test
is added (contract: e2e only; the arming is inert under `Execute(context.Background())`), the harmless goroutine
behavior in the cmd test binary (analyzed and safe), and 9 grep guards. The change is small enough to be fully specified
verbatim. The -1 from 10/10 reflects the inherent dependency on the two parallel siblings landing exactly as their PRPs
specify (treated as contracts here, but they are in-flight); if either's public surface drifts (e.g. `Arm` gains an error
return, or the field is renamed), the consumer's exact text would need a one-line adjustment — which the grep guards and
the "Arm returns nothing" gotcha make trivial to catch and fix.
