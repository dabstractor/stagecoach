name: "P1.M3.T1.S1 — Add per-query timeout + WaitDelay to cmdRunner.Run (BUG-004, replicate osRunner's 3s deadline)"
description: >
  Bug fix (BUG-004, Major): the PRODUCTION tier-(b) package-manager query runner `cmdRunner.Run`
  (internal/cmd/upgrade_run.go:152) has NEITHER the 3s `context.WithTimeout` NOR `cmd.WaitDelay` that the
  reference `osRunner.Run` (internal/upgrade/detect.go:121) was correctly built with. osRunner is UNEXPORTED,
  so package cmd wrote a timeout-less twin. The stale comment (upgrade_run.go:141-142) falsely claims "the
  command's ctx already bounds the whole upgrade" — but main.go:62 builds the root ctx via
  `signal.Install(context.Background(), …)` (signal-cancelable, NO deadline), so a hung PM query (brew
  refreshing its DB, NFS-mounted home, broken PM) hangs `stagecoach upgrade` indefinitely, recoverable only
  via Ctrl-C; and the missing WaitDelay means a forked grandchild holding the stdout pipe blocks past any
  cancel. FR-U2(b)/external_deps §7 ("these queries must not hang") is violated. FIX = Option A (the
  contract's PREFERRED): replicate osRunner's two missing pieces in cmdRunner.Run — (1) `ctx, cancel :=
  context.WithTimeout(ctx, upgrade.DefaultQueryTimeout); defer cancel()` at the top of Run; (2)
  `cmd.WaitDelay = upgrade.DefaultQueryTimeout` after the cmd creation — and EXPORT the shared constant
  `defaultQueryTimeout` → `DefaultQueryTimeout` in detect.go (3 edits there; ZERO test references ⇒ safe) so
  the two runners CANNOT drift again (drift is the root cause of this bug). The existing error-handling block
  (ctx.Err() FIRST ⇒ skip; ExitError ⇒ skip; start-failure ⇒ err) is UNCHANGED — it already mirrors osRunner;
  the timeout simply makes the ctx.Err() branch REACHABLE on a hang. [Mode A] rewrite the stale comment
  (remove the false "no per-query timeout / ctx bounds the upgrade" claim; document the 3s deadline +
  WaitDelay + that the root ctx has no deadline). NO new import in upgrade_run.go
  (upgrade.DefaultQueryTimeout is a time.Duration; assigning/passing it needs no `time` import in package cmd).
  NOT in scope: BUG-007/008 (P1.M3.T2/T3), the docs sync (P1.M3.T4), osRunner's logic, Option B (exporting
  osRunner — "changes the seam architecture"). The parallel P1.M2.T3.S1 (decompose docs) is unrelated (zero
  overlap). Optional regression test (a hanging `sleep` ⇒ returns within ~3.5s with DeadlineExceeded) noted
  but not required by the contract.

---

## Goal

**Feature Goal**: Bound every production package-manager detection query in `stagecoach upgrade` to 3 seconds
(per-query) so a hung PM (brew refreshing its DB, a broken PM, an NFS-mounted home) is recovered-with-skip
instead of hanging the whole upgrade indefinitely. This closes BUG-004 by giving `cmdRunner.Run` the same
per-query deadline + WaitDelay its reference twin `osRunner.Run` already has, and by sharing the timeout
constant so the two cannot drift apart again.

**Deliverable** (3 edits in `internal/upgrade/detect.go` + 2 code edits + 1 comment rewrite in
`internal/cmd/upgrade_run.go`; no new files):
1. **detect.go** — export `defaultQueryTimeout` → `DefaultQueryTimeout` (the const + its godoc + the 2 in-package references).
2. **upgrade_run.go cmdRunner.Run** — add `context.WithTimeout(ctx, upgrade.DefaultQueryTimeout)` + `defer cancel()` at the top; add `cmd.WaitDelay = upgrade.DefaultQueryTimeout` after the cmd creation.
3. **upgrade_run.go cmdRunner comment** — [Mode A] rewrite the stale "There is NO per-query timeout here … the command's ctx already bounds the whole upgrade" claim to document the 3s deadline + WaitDelay + the root-ctx-has-no-deadline fact.

**Success Definition**:
- `cmdRunner.Run` derives a per-query timeout context (`context.WithTimeout(ctx, upgrade.DefaultQueryTimeout)`)
  and sets `cmd.WaitDelay = upgrade.DefaultQueryTimeout` on every invocation.
- A hung PM query (a command that does not exit within 3s) returns within ~3s + WaitDelay with
  `errors.Is(err, context.DeadlineExceeded)` (the existing `ctx.Err()`-first branch now fires) ⇒ the probe
  skips, the cascade continues, `stagecoach upgrade` does not hang.
- The existing error contract is UNCHANGED: exit 0 ⇒ (stdout, 0, nil); non-zero exit ⇒ (stdout, code, nil)
  ["not installed" skip]; start failure / deadline ⇒ ("", 0, err) [skip].
- `upgrade.DefaultQueryTimeout` is the single shared 3s constant both `osRunner` and `cmdRunner` read (no drift).
- The stale comment no longer claims "no per-query timeout" or "ctx bounds the upgrade."
- `go build ./...` + cross-build clean; `go vet` clean; `gofmt -l` empty; `make test` + `make lint` green.
- Scope: `git status` == {internal/cmd/upgrade_run.go, internal/upgrade/detect.go} (only).

## User Persona (if applicable)

**Target User**: A `stagecoach upgrade` user whose system has a slow/hung package manager (brew mid-DB-refresh,
a broken PM, an NFS home that stalls forked helpers). Also the maintainer who needs the upgrade detection
cascade to be robust against environmental hangs.

**Use Case**: `stagecoach upgrade` probes `brew list stagecoach` / `scoop prefix` / `choco list` / etc. to
detect the install channel. One of those PMs hangs (brew is refreshing its DB). Pre-fix: the upgrade hangs
forever (only Ctrl-C recovers). Post-fix: the 3s per-query deadline fires, the probe is treated as "PM hung
⇒ skip", and the cascade moves on (or resolves via path heuristics / the override).

**User Journey**: user runs `stagecoach upgrade` → prodDetect → cmdRunner.Run(`brew list …`) → brew hangs →
3s deadline fires → ctx cancelled → child killed → WaitDelay force-closes the stdout pipe after a bounded
grace → Run returns (partial-stdout, 0, context.DeadlineExceeded) → the brew probe skips → detection falls
through to the next tier (path heuristics / override / direct) → upgrade proceeds (or reports the channel it
found) instead of hanging.

**Pain Points Addressed**: BUG-004 / FR-U2(b) / external_deps §7 — "these queries must not hang." Pre-fix a
single hung PM query makes the whole upgrade unrecoverable without Ctrl-C; the missing WaitDelay means even a
signal-cancel can leave a forked grandchild holding the stdout pipe.

## Why

- **BUG-004 (Major) / FR-U2(b) / external_deps §7**: the production detection cascade must not hang on a
  single PM query. `osRunner.Run` (the reference, detect.go:121) was built correctly with the 3s
  `context.WithTimeout` + `cmd.WaitDelay`; `cmdRunner.Run` (the package-cmd twin, upgrade_run.go:152) was
  written WITHOUT them because osRunner is unexported and the author mistakenly believed the inherited ctx
  bounded the run (it does not — main.go:62's root ctx is signal-cancelable with NO deadline). This task
  replicates the proven pattern.
- **Shared constant (root-cause fix)**: the bug IS drift — two runners with the same contract diverged, and
  one lost the timeout. Exporting `defaultQueryTimeout` → `DefaultQueryTimeout` and having both runners read
  it makes that drift structurally impossible to recur.
- **Minimal, behavior-preserving for the happy path**: the happy path (PM exits 0 quickly) and the "not
  installed" path (non-zero exit) are byte-identical — the timeout + WaitDelay only engage on a hang. The
  error-handling block is already correct (it already checks `ctx.Err()` first); the timeout just makes that
  branch reachable. No new type, no seam change, no Option-B architectural churn.

## What

**User-visible behavior**: `stagecoach upgrade` no longer hangs on a slow/hung package-manager query — each
probe is bounded to ~3s, and a hung probe is skipped (the cascade continues). Happy-path behavior is unchanged.

**Technical change**: export the shared 3s constant + add the two missing lines to cmdRunner.Run + rewrite
the stale comment. See the Implementation Blueprint for verbatim before/after.

### Success Criteria
- [ ] `internal/upgrade/detect.go` exports `DefaultQueryTimeout = 3 * time.Second` (was unexported `defaultQueryTimeout`);
      the osRunner.timeout field comment + the line-124 reference use the new name.
- [ ] `internal/cmd/upgrade_run.go` `cmdRunner.Run` derives `ctx, cancel := context.WithTimeout(ctx,
      upgrade.DefaultQueryTimeout); defer cancel()` at the top and sets `cmd.WaitDelay = upgrade.DefaultQueryTimeout`.
- [ ] The existing error-handling block (ctx.Err() first; ExitError ⇒ skip; start-failure ⇒ err) is UNCHANGED.
- [ ] The stale comment (141-142) is rewritten: no "no per-query timeout" / "ctx bounds the upgrade" claim;
      documents the 3s deadline + WaitDelay + the root-ctx-has-no-deadline fact.
- [ ] `go build ./...` + cross-build clean; `go vet ./internal/cmd/... ./internal/upgrade/...` clean;
      `gofmt -l` empty on both files; `make test` + `make lint` green.
- [ ] Scope: `git status` == {internal/cmd/upgrade_run.go, internal/upgrade/detect.go} (only).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim before/after for both runners (the buggy cmdRunner.Run + the reference osRunner.Run),
the exact 3 detect.go edits to export the constant (with the verified fact that it has ZERO test/external
references), the exact 2 lines to add to cmdRunner.Run (and WHERE — top of Run for WithTimeout, after cmd
creation for WaitDelay), the proof that the error-handling block is already correct (so it stays unchanged),
the proof that NO new import is needed in upgrade_run.go (assigning a `time.Duration` constant needs no `time`
import in package cmd), the stale-comment text to remove + the accurate replacement text, the rejected
alternatives (Option B = export osRunner; hardcode `3*time.Second`), and the grep guards.

### Documentation & References

```yaml
# MUST READ — the authoritative findings (verbatim both runners + the export-is-safe proof + scope fences)
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M3T1S1/research/findings.md
  why: "§1 the bug (cmdRunner.Run missing WithTimeout+WaitDelay; the stale FALSE comment; main.go:62 root ctx
        has NO deadline) + the reference (osRunner.Run, verbatim); §2 the fix (Option A — the 2 missing pieces
        + the shared-constant rationale + NO new import proof); §3 the exact detect.go rename (3 edits); §4 scope
        fences; §5 the optional test; §6 validation."
  critical: "§1: the existing error-handling block is ALREADY correct (ctx.Err() first) — DO NOT change it; the
             timeout just makes it reachable. §2: NO new import in upgrade_run.go (upgrade.DefaultQueryTimeout is
             a time.Duration). §2/§3: EXPORT the constant (the contract's 'Preferred shared constant'); hardcoding
             is the inferior fallback (reintroduces drift)."

# MUST EDIT — the buggy file (cmdRunner.Run + its stale comment)
- file: internal/cmd/upgrade_run.go   # cmdRunner.Run :152; the stale comment :136-142
  why: "THE bug location. cmdRunner.Run (line 152) needs the WithTimeout + WaitDelay added; the type comment
        (lines 136-142, esp. 141-142 'There is NO per-query timeout here… the command's ctx already bounds the
        whole upgrade') must be rewritten — that claim is FALSE (main.go:62's root ctx has no deadline)."
  pattern: "Mirror osRunner.Run's structure: WithTimeout at the top (defer cancel); WaitDelay after cmd creation;
            the error block (ctx.Err() first) stays. Reference upgrade.DefaultQueryTimeout (the exported constant)."
  gotcha: "Do NOT add \"time\" to the import block — upgrade.DefaultQueryTimeout is a time.Duration; assigning it
           to cmd.WaitDelay / passing to context.WithTimeout needs no time import in package cmd. (Hardcoding
           3*time.Second WOULD require the import — another reason to prefer the shared constant.)"

# MUST EDIT — export the shared constant (detect.go; the reference runner's home)
- file: internal/upgrade/detect.go   # defaultQueryTimeout :112; osRunner.timeout comment :106; usage :124
  why: "Export defaultQueryTimeout → DefaultQueryTimeout so cmdRunner can reference upgrade.DefaultQueryTimeout
        and the two runners cannot drift. Verified SAFE: defaultQueryTimeout is referenced ONLY in detect.go
        (lines 106/109/112/124) — ZERO test references, ZERO external callers."
  pattern: "Rename the const (capital D); update its godoc to note it is shared with cmd.cmdRunner; update the
            2 in-package references (the field comment at :106 '0 ⇒ 3s default (defaultQueryTimeout)' and the
            usage at :124 'timeout = defaultQueryTimeout')."
  gotcha: "Do NOT change osRunner's LOGIC — only export the constant + update the references. osRunner.Run's body
           stays byte-identical (it already reads defaultQueryTimeout, now DefaultQueryTimeout)."

# MUST READ — the reference implementation (osRunner.Run, the pattern to replicate)
- file: internal/upgrade/detect.go   # osRunner.Run :121-160; defaultQueryTimeout :109-112
  why: "The CORRECT twin. Lines 126 (WithTimeout) + 140 (WaitDelay) + 147 (ctx.Err() first) are exactly what
        cmdRunner.Run must gain. Copy the structure, not the osRunner-specific `r.timeout` field (cmdRunner is
        a struct{} with no fields — use the constant directly)."
  critical: "osRunner is UNEXPORTED (type osRunner :105) — package cmd cannot reuse it. That is WHY cmdRunner
             exists. Keep cmdRunner; just give it the timeout. (Option B — exporting osRunner — is rejected:
             'changes the seam architecture'.)"

# CONTEXT — the root ctx provenance (proves the stale comment is false)
- file: cmd/stagecoach/main.go   # :62 signal.Install(context.Background(), ...)
  why: "Confirms the root ctx is signal-cancelable with NO deadline. This is WHY the per-query WithTimeout is
        load-bearing — without it, a hung PM query has no bound (only Ctrl-C recovers)."
  critical: "READ-ONLY. Do NOT edit main.go. The stale comment in upgrade_run.go claimed this ctx 'bounds the
             whole upgrade' — it does not; the comment rewrite documents the truth."

# CONTEXT — the bug spec (the selected_prd §Issue 3 / BUG-004)
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/architecture/bugfix_subsystems.md
  section: "## BUG-004 (MAJOR): Upgrade cmdRunner has no per-query timeout or WaitDelay"
  why: "The authoritative bug statement: location (upgrade_run.go:152), the false comment, main.go:62 root ctx,
        the missing WaitDelay grandchild hazard, FR-U2(b)/external_deps §7."
  critical: "Confirms the fix is 'export upgrade.osRunner (or replicate its 3s WithTimeout + cmd.WaitDelay)' —
             this task does the REPLICATE path with a shared constant (Option A, preferred)."

# CONTEXT — the parallel sibling (NO overlap; READ-ONLY)
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M2T3S1/PRP.md
  why: "P1.M2.T3.S1 is a docs-only audit of README.md/docs/how-it-works.md for the DECOMPOSE fast-path (BUG-003).
        Zero file overlap with upgrade_run.go/detect.go; no conflict. Confirms this task owns the upgrade-subsystem
        timeout fix exclusively."
  critical: "Do NOT touch README.md or docs/how-it-works.md here — those are the parallel sibling's scope."
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/
  upgrade_run.go          # EDIT — cmdRunner.Run (+WithTimeout +WaitDelay) + the stale comment rewrite
internal/upgrade/
  detect.go               # EDIT — export defaultQueryTimeout → DefaultQueryTimeout (const + godoc + 2 refs); osRunner.Run UNCHANGED
cmd/stagecoach/
  main.go                 # READ-ONLY — :62 root ctx provenance (signal.Install, no deadline)
# go.mod / Makefile — READ-ONLY (no new dep; Go 1.20+ already required for cmd.WaitDelay)
```

### Desired Codebase tree with files to be modified

```bash
internal/cmd/upgrade_run.go     # MODIFIED — cmdRunner.Run gains WithTimeout + WaitDelay; stale comment rewritten
internal/upgrade/detect.go      # MODIFIED — defaultQueryTimeout → DefaultQueryTimeout (exported; 3 edits); osRunner logic UNCHANGED
# (NO new files. NO other modifications.)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the error-handling block is ALREADY correct — DO NOT change it): cmdRunner.Run already does
//   if cerr := ctx.Err(); cerr != nil { return buf.String(), 0, cerr }   // BEFORE the ExitError check.
// This is the RIGHT order (a deadline-kill surfaces as *exec.ExitError, so ctx.Err() must be checked first).
// Pre-fix it was dead code for hangs (no timeout ⇒ ctx.Err() nil unless Ctrl-C). Adding the WithTimeout makes
// it REACHABLE. Do not reorder or rewrite the block — just add the timeout above it.

// CRITICAL (export the constant; do NOT hardcode 3*time.Second): the bug IS drift (two runners diverged).
// Exporting defaultQueryTimeout → DefaultQueryTimeout and having both read it makes drift impossible. Hardcoding
// 3*time.Second in cmdRunner (a) reintroduces the drift risk and (b) requires adding "time" to upgrade_run.go's
// import block. The shared constant needs NO new import (upgrade.DefaultQueryTimeout is a time.Duration; assigning
// it to cmd.WaitDelay / passing to context.WithTimeout does not require package cmd to import "time").

// CRITICAL (WaitDelay goes AFTER cmd creation, WithTimeout at the TOP of Run): the ordering is
//   ctx, cancel := context.WithTimeout(ctx, upgrade.DefaultQueryTimeout); defer cancel()   // first
//   cmd := exec.CommandContext(ctx, name, args...)                                          // uses the derived ctx
//   cmd.WaitDelay = upgrade.DefaultQueryTimeout                                             // after creation
// exec.CommandContext must receive the TIMEOUT-derived ctx (not the raw one), so WithTimeout precedes it.

// CRITICAL (the stale comment is FALSE — rewrite it, don't preserve it): upgrade_run.go:141-142 says "There is
// NO per-query timeout here (unlike osRunner's 3s) because the command's ctx already bounds the whole upgrade;
// a hung PM surfaces as ctx.Err()." EVERY clause is wrong post-fix: there IS a per-query timeout; the ctx does
// NOT bound the upgrade (main.go:62 root ctx has no deadline); a hung PM surfaces as ctx.Err() ONLY because the
// per-query timeout fires. Rewrite to document the 3s deadline + WaitDelay + the root-ctx-has-no-deadline fact.

// GOTCHA (Go 1.20+ for cmd.WaitDelay): cmd.WaitDelay was added in Go 1.20. The repo's go.mod already requires
// Go 1.20+ (osRunner uses WaitDelay today). No go.mod change.

// GOTCHA (do NOT export osRunner — Option B is rejected): exporting osRunner and using it in prodDetect "changes
// the seam architecture" (cmdRunner is the package-cmd test-seam twin P1.M4.T3 overrides via upgradeDetect).
// Keep cmdRunner; give it the timeout. Only the CONSTANT is exported.

// GOTCHA (do NOT touch osRunner.Run's logic): the export is a RENAME of the constant + reference updates.
// osRunner.Run's body (the WithTimeout/WaitDelay/error-handling) stays byte-identical — it already reads the
// (now-renamed) constant. Do not "improve" it.

// GOTCHA (scope — NOT BUG-007/008/docs): Linuxbrew Cellar heuristic (BUG-007) is P1.M3.T2; escaping c.Repo
// (BUG-008) is P1.M3.T3; the docs sync is P1.M3.T4. This task is ONLY the cmdRunner timeout + the constant export.
```

## Implementation Blueprint

### Data models and structure

None. No new types, no struct fields (cmdRunner stays `struct{}`), no interface change. One constant is
renamed (exported); two lines are added to one method; one comment is rewritten.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/upgrade/detect.go — EXPORT the shared constant (do this FIRST so cmdRunner can reference it)
  - (a) const rename (line 112):
      OLD:  const defaultQueryTimeout = 3 * time.Second
      NEW:  const DefaultQueryTimeout = 3 * time.Second
      + update its godoc (lines 109-111) to note it is SHARED with cmd.cmdRunner (e.g. append: "Exported so
        internal/cmd/upgrade_run.go's cmdRunner shares the same bound as osRunner — the two production runners
        must not drift (BUG-004).").
  - (b) osRunner.timeout field comment (line 106):
      OLD:  timeout time.Duration // 0 ⇒ 3s default (defaultQueryTimeout).
      NEW:  timeout time.Duration // 0 ⇒ 3s default (DefaultQueryTimeout).
  - (c) usage in osRunner.Run (line 124):
      OLD:  timeout = defaultQueryTimeout
      NEW:  timeout = DefaultQueryTimeout
  - ANCHOR each by its unique string (the const decl, the field comment, the assignment). 3 small edits.
  - VERIFY: grep -rn 'defaultQueryTimeout' . ⇒ ZERO hits (all renamed); grep -rn 'DefaultQueryTimeout' . ⇒ the 3 detect.go sites + the new cmdRunner reference (Task 2).

Task 2: EDIT internal/cmd/upgrade_run.go cmdRunner.Run — add the per-query timeout + WaitDelay
  - OLD (the body, lines 152-171):
        func (cmdRunner) Run(ctx context.Context, name string, args ...string) (string, int, error) {
            var buf bytes.Buffer
            cmd := exec.CommandContext(ctx, name, args...)
            cmd.Stdout = &buf
            if err := cmd.Run(); err != nil {
                ...ctx.Err() first; ExitError ⇒ skip; start-failure ⇒ err...
            }
            return buf.String(), 0, nil
        }
  - NEW (add 2 lines — WithTimeout at the top, WaitDelay after cmd creation; the error block UNCHANGED):
        func (cmdRunner) Run(ctx context.Context, name string, args ...string) (string, int, error) {
            // BUG-004: bound each PM query to the shared 3s deadline so a hung PM (brew refreshing its DB, NFS
            // home, broken PM) cannot stall the cascade. The root ctx (main.go signal.Install) is signal-cancelable
            // with NO deadline, so this per-query bound is load-bearing (FR-U2(b)/external_deps §7). Mirrors
            // upgrade.osRunner.Run; the shared upgrade.DefaultQueryTimeout keeps the two runners from drifting.
            ctx, cancel := context.WithTimeout(ctx, upgrade.DefaultQueryTimeout)
            defer cancel()

            var buf bytes.Buffer
            cmd := exec.CommandContext(ctx, name, args...)
            cmd.Stdout = &buf
            // WaitDelay bounds how long Run waits for the process (and any forked grandchild still holding the
            // stdout pipe) to release its I/O AFTER the deadline fires. Without it a hung PM that forked a helper
            // keeps the pipe open past the cancel. Same window as the timeout (mirrors osRunner).
            cmd.WaitDelay = upgrade.DefaultQueryTimeout

            if err := cmd.Run(); err != nil {
                // ctx deadline / cancellation kills the child with a signal that looks like an ExitError —
                // check ctx.Err() first so a timeout is treated as "PM hung" (err), not "not installed".
                if cerr := ctx.Err(); cerr != nil {
                    return buf.String(), 0, cerr
                }
                var ee *exec.ExitError
                if errors.As(err, &ee) {
                    return buf.String(), ee.ExitCode(), nil // non-zero exit ⇒ skip, NOT err
                }
                return "", 0, err // start failure (binary absent) ⇒ err
            }
            return buf.String(), 0, nil
        }
  - ANCHOR: the unique `var buf bytes.Buffer\n\tcmd := exec.CommandContext(ctx, name, args...)` block.
  - PRESERVE: the entire error-handling block (ctx.Err() first; ExitError ⇒ skip; start-failure ⇒ err) UNCHANGED.
  - NO new import (upgrade.DefaultQueryTimeout is a time.Duration; context + os/exec already imported).

Task 3: EDIT internal/cmd/upgrade_run.go — rewrite the STALE cmdRunner type comment (Mode A)
  - OLD (the false claim, lines ~140-142):
        // ⇒ err. There is NO per-query timeout here (unlike osRunner's 3s) because the command's ctx already
        // bounds the whole upgrade; a hung PM surfaces as ctx.Err().
  - NEW:
        // ⇒ err. Each Run wraps the command in a per-query 3s deadline (upgrade.DefaultQueryTimeout) +
        // cmd.WaitDelay, mirroring upgrade.osRunner (the root ctx from main.go's signal.Install is
        // signal-cancelable with NO deadline, so the per-query bound is load-bearing — FR-U2(b)/external_deps
        // §7; BUG-004). The shared constant keeps the two runners from drifting.
  - ANCHOR: the unique "There is NO per-query timeout here (unlike osRunner's 3s)" string.
  - This IS the [Mode A] docs work (in-code comment; no user-facing doc surface change).

Task 4: VERIFY — build (native+cross), vet, format, existing suites, lint, grep guards
  - go build ./... ; GOOS=windows go build ./... ; GOOS=linux go build ./... ; GOOS=darwin go build ./...
  - go vet ./internal/cmd/... ./internal/upgrade/...
  - gofmt -l internal/cmd/upgrade_run.go internal/upgrade/detect.go   # empty
  - go test ./internal/upgrade/...        # osRunner/detect tests stay green (additive rename)
  - go test ./internal/cmd/...            # cmd tests stay green (cmdRunner contract unchanged on the happy path)
  - make test ; make lint
  - grep guards (see Validation Loop Level 4)

Task 5 (OPTIONAL): regression test — a hanging command returns within ~3.5s with DeadlineExceeded
  - IF you add a test (the contract lists no test; this is a value-add), place it in internal/cmd (package cmd,
    internal — can access the unexported cmdRunner), Unix-only:
        //go:build !windows
        package cmd
        import ("context";"errors";"testing";"time")
        func TestCmdRunner_PerQueryTimeoutBounded(t *testing.T) {
            start := time.Now()
            _, _, err := (cmdRunner{}).Run(context.Background(), "sleep", "30")
            elapsed := time.Since(start)
            if !errors.Is(err, context.DeadlineExceeded) {
                t.Errorf("hung query err = %v, want DeadlineExceeded (BUG-004)", err)
            }
            if elapsed >= 30*time.Second { t.Errorf("hung query ran %v, want bounded by ~3s+WaitDelay", elapsed) }
            if elapsed < 2500*time.Millisecond || elapsed > 4500*time.Millisecond {
                t.Logf("note: elapsed = %v (expected ~3s+WaitDelay; CI-variance window 2.5–4.5s)", elapsed)
            }
        }
  - NOTE: this is a ~3s test (slow but deterministic). Unix-only (sleep). Windows CI skips it. OPTIONAL — the
    PRIMARY validation is build/vet/gofmt + the existing suites green + grep guards. If you add it, confirm
    `go test ./internal/cmd/ -run TestCmdRunner_PerQueryTimeoutBounded -v` passes.
```

### Implementation Patterns & Key Details

```go
// PATTERN: the per-query timeout + WaitDelay (mirror osRunner.Run exactly; the shared constant prevents drift).
ctx, cancel := context.WithTimeout(ctx, upgrade.DefaultQueryTimeout)
defer cancel()
cmd := exec.CommandContext(ctx, name, args...)
cmd.Stdout = &buf
cmd.WaitDelay = upgrade.DefaultQueryTimeout
// ... unchanged error block: ctx.Err() first (deadline ⇒ skip), ExitError ⇒ skip, start-failure ⇒ err.

// PATTERN: the shared exported constant (detect.go) — the DRY fix for the drift that caused BUG-004.
const DefaultQueryTimeout = 3 * time.Second   // exported; read by BOTH osRunner.Run AND cmd.cmdRunner.Run.
```

### Integration Points

```yaml
UPGRADE PACKAGE (internal/upgrade/detect.go):
  - RENAME defaultQueryTimeout → DefaultQueryTimeout (exported); update godoc + 2 in-package references.
  - osRunner.Run logic UNCHANGED (it already reads the constant; now the exported name).

CMD PACKAGE (internal/cmd/upgrade_run.go):
  - cmdRunner.Run: +context.WithTimeout(upgrade.DefaultQueryTimeout) +cmd.WaitDelay = upgrade.DefaultQueryTimeout.
  - cmdRunner type comment: rewritten (Mode A — remove the false "no per-query timeout / ctx bounds" claim).

NO database / migration / routes / new types / new imports / new files / interface changes / config changes.
  - Go 1.20+ (cmd.WaitDelay) already required by osRunner today — no go.mod change.
  - cmdRunner stays struct{} (no timeout FIELD — it uses the constant directly; only osRunner has the injectable
    field, for its tests). prodDetect's `Exec: cmdRunner{}` is unchanged.

SCOPE FENCES: NO osRunner logic change (only the const rename); NO Option B (exporting osRunner); NO BUG-007
  (Linuxbrew — P1.M3.T2); NO BUG-008 (escape c.Repo — P1.M3.T3); NO docs sync (P1.M3.T4); NO main.go edit;
  NO README/docs/how-it-works edit (the parallel P1.M2.T3.S1 sibling owns decompose docs — zero overlap).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build + vet (the rename + the 2 added lines must compile; cmd.WaitDelay needs Go 1.20+, already required).
go build ./...
GOOS=windows go build ./...
GOOS=linux   go build ./...
GOOS=darwin  go build ./...
# Expected: clean. A failure usually means a missed defaultQueryTimeout reference (Task 1) or a typo in the const name.

# Vet.
go vet ./internal/cmd/... ./internal/upgrade/...
# Expected: clean.

# Format.
gofmt -l internal/cmd/upgrade_run.go internal/upgrade/detect.go
# Expected: empty. If listed: gofmt -w the file(s).

# Lint.
make lint      # golangci-lint
# Expected: zero errors. The exported DefaultQueryTimeout is referenced (cmdRunner) ⇒ not `unused`.

# Scope guard: only the 2 expected files changed.
git status --short
# Expected: M internal/cmd/upgrade_run.go  M internal/upgrade/detect.go  (only).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The upgrade package — osRunner/detect tests stay GREEN (the rename is additive; osRunner logic unchanged).
go test ./internal/upgrade/... -v
# Expected: green. (defaultQueryTimeout had ZERO test references — verified — so the rename breaks nothing.)

# The cmd package — cmdRunner's happy-path contract (exit 0 / non-zero / start-failure) is UNCHANGED; the
# timeout only engages on a hang, which no existing test triggers.
go test ./internal/cmd/... -v
# Expected: green. (If you added the optional Task 5 test, it runs here as a ~3s bounded-hang assertion.)

# Full race suite.
make test
# Expected: green. The change is additive (a timeout that only fires on a hang); no shared mutable state added.
```

### Level 3: Integration Testing (System Validation)

```bash
# There is no clean integration/e2e for this without a real hung PM (brew mid-refresh is not reproducible on
# demand). The optional Task 5 unit test (a `sleep 30` ⇒ ~3s return) is the deterministic proxy. A real e2e
# (stagecoach upgrade against a host with a wedged PM) is manual-only.

# Sanity: the package still builds into the binary (no downstream compile break).
go build ./...

# Optional manual smoke (NOT CI): on a host where `brew` is mid-DB-refresh (or point STAGECOACH at a fake slow
# PM), `stagecoach upgrade` should return within ~3s per probe (skip the hung PM) instead of hanging. The unit
# test (Task 5) is the committed proof; this is a confidence check only.
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: the constant is exported + referenced by BOTH runners (the shared-constant / no-drift contract).
grep -n 'const DefaultQueryTimeout' internal/upgrade/detect.go           # the export (1 hit)
grep -n 'DefaultQueryTimeout' internal/cmd/upgrade_run.go                # cmdRunner's reference (≥2: WithTimeout + WaitDelay)
grep -n 'DefaultQueryTimeout' internal/upgrade/detect.go                 # osRunner's reference (≥2: field comment + usage)

# Guard 2: the unexported defaultQueryTimeout is GONE (no stragglers).
grep -rn 'defaultQueryTimeout' --include='*.go' .
# Expected: ZERO hits (all renamed to DefaultQueryTimeout).

# Guard 3: cmdRunner.Run has BOTH the WithTimeout AND the WaitDelay (the two missing pieces).
grep -n 'context.WithTimeout(ctx, upgrade.DefaultQueryTimeout)' internal/cmd/upgrade_run.go
grep -n 'cmd.WaitDelay = upgrade.DefaultQueryTimeout' internal/cmd/upgrade_run.go
# Expected: 1 hit each.

# Guard 4: the stale comment is GONE (no "no per-query timeout" / "ctx already bounds" claim).
grep -n 'There is NO per-query timeout\|ctx already bounds' internal/cmd/upgrade_run.go
# Expected: ZERO hits (the false claim was rewritten).

# Guard 5: the error-handling block is UNCHANGED (ctx.Err() first; ExitError ⇒ skip).
grep -n 'if cerr := ctx.Err(); cerr != nil' internal/cmd/upgrade_run.go
grep -n 'ee.ExitCode(), nil // non-zero exit ⇒ skip' internal/cmd/upgrade_run.go
# Expected: 1 hit each (the block was preserved, not rewritten).

# Guard 6: NO new import added to upgrade_run.go (the shared constant needs no "time" import).
git diff internal/cmd/upgrade_run.go | grep -E '^\+.*"time"'
# Expected: EMPTY (no new import line). If non-empty, you hardcoded 3*time.Second instead of using the constant — use the constant.

# Guard 7: osRunner.Run's LOGIC is byte-identical (only the const name changed).
git diff internal/upgrade/detect.go | grep -E '^\-.*timeout|^+.*timeout'
# Expected: ONLY the defaultQueryTimeout → DefaultQueryTimeout rename (the const decl, the field comment, the
#           usage). NO other osRunner.Run line changes.

# Guard 8: scope — only 2 files.
git status --porcelain
# Expected: M internal/cmd/upgrade_run.go  M internal/upgrade/detect.go  (only).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + cross-build (windows/linux/darwin) clean
- [ ] `go vet ./internal/cmd/... ./internal/upgrade/...` clean
- [ ] `gofmt -l internal/cmd/upgrade_run.go internal/upgrade/detect.go` empty
- [ ] `make lint` zero errors (DefaultQueryTimeout is referenced ⇒ not `unused`)
- [ ] `go test ./internal/upgrade/...` + `go test ./internal/cmd/...` green (existing suites; additive change)
- [ ] `make test` (race) green

### Feature Validation
- [ ] `cmdRunner.Run` derives `context.WithTimeout(ctx, upgrade.DefaultQueryTimeout)` + sets `cmd.WaitDelay`
- [ ] The error-handling block is UNCHANGED (ctx.Err() first; ExitError ⇒ skip; start-failure ⇒ err)
- [ ] `DefaultQueryTimeout` is exported in detect.go and referenced by BOTH osRunner and cmdRunner
- [ ] The stale comment is rewritten (no "no per-query timeout" / "ctx bounds the upgrade" claim)
- [ ] (Optional Task 5) the bounded-hang test passes (`sleep 30` ⇒ ~3s return + DeadlineExceeded)

### Scope-Boundary Validation
- [ ] `git status` == {internal/cmd/upgrade_run.go, internal/upgrade/detect.go} (only 2 files)
- [ ] NO osRunner.Run logic change (only the const rename + references)
- [ ] NO new import in upgrade_run.go; NO new file; NO new type/field/interface; NO go.mod change
- [ ] NO Option B (osRunner not exported as a type); NO BUG-007/008/docs-sync (P1.M3.T2/T3/T4)
- [ ] NO main.go / README / docs/how-it-works edit (parallel sibling P1.M2.T3.S1 owns decompose docs)
- [ ] Grep guards 1–8 (Level 4) all pass

### Code Quality & Docs
- [ ] cmdRunner.Run's added comment cites BUG-004 + FR-U2(b)/external_deps §7 + the root-ctx-has-no-deadline fact
- [ ] DefaultQueryTimeout's godoc notes it is shared with cmd.cmdRunner (the no-drift rationale)
- [ ] The fix mirrors osRunner.Run's structure exactly (the proven reference)

---

## Anti-Patterns to Avoid

- ❌ Don't change the error-handling block. It is ALREADY correct — `ctx.Err()` is checked BEFORE the
  `*exec.ExitError` match (the right order, since a deadline-kill surfaces as an ExitError). Pre-fix it was
  dead code for hangs only because there was no timeout. Adding the timeout makes it reachable; do not rewrite it.
- ❌ Don't hardcode `3 * time.Second` in cmdRunner. The bug IS drift (two runners diverged; cmdRunner lost the
  timeout). Export `DefaultQueryTimeout` and reference `upgrade.DefaultQueryTimeout` so the two CANNOT drift
  again. Hardcoding also forces adding `"time"` to upgrade_run.go's imports (the shared constant needs no import).
- ❌ Don't export `osRunner` (Option B). The contract explicitly rejects it: "changes the seam architecture."
  cmdRunner is the package-cmd test-seam twin (P1.M4.T3 overrides `upgradeDetect`); keep it, just give it the
  timeout. Only the CONSTANT is exported.
- ❌ Don't touch `osRunner.Run`'s logic. The detect.go edit is a const RENAME + reference updates — osRunner's
  body (WithTimeout/WaitDelay/error-handling) stays byte-identical. It already reads the constant; it just gets
  the exported name. "Improving" it is scope creep.
- ❌ Don't preserve the stale comment. The claim "There is NO per-query timeout here … the command's ctx already
  bounds the whole upgrade" is FALSE on every clause post-fix (there IS a timeout; the ctx does NOT bound the
  upgrade — main.go:62's root ctx has no deadline). Rewrite it (Task 3) — that IS the Mode-A docs work.
- ❌ Don't reverse the ordering (WaitDelay before WithTimeout, or WithTimeout after cmd creation). The
  timeout-derived ctx must be passed to `exec.CommandContext`, so `context.WithTimeout` goes at the TOP of Run
  (before cmd creation); `cmd.WaitDelay` goes AFTER cmd creation. Reversing either breaks the bound.
- ❌ Don't add a `timeout` field to `cmdRunner`. It is `struct{}` (the package-cmd twin); osRunner has the
  injectable field for ITS tests, but cmdRunner uses the constant directly. Adding a field changes the type and
  the `prodDetect` construction (`Exec: cmdRunner{}`) for no benefit — the contract pins the 3s constant.
- ❌ Don't conflate the root-ctx deadline with the per-query deadline. main.go:62's `signal.Install(context.Background(),
  …)` is signal-cancelable with NO deadline — that is WHY the per-query WithTimeout is load-bearing. The fix does
  NOT touch main.go (out of scope); it adds the per-query bound inside cmdRunner.Run.
- ❌ Don't edit main.go, README.md, docs/how-it-works.md, or any BUG-007/008 file. main.go is READ-ONLY (the root
  ctx is the provenance fact, not the fix site). The decompose docs (README/how-it-works) are the parallel
  sibling P1.M2.T3.S1's scope. BUG-007 (Linuxbrew) is P1.M3.T2; BUG-008 (escape c.Repo) is P1.M3.T3.
- ❌ Don't add a slow/unbounded test. If you add the optional regression test (Task 5), it must assert the hung
  query returns within ~3s+WaitDelay (bounded) — NOT actually wait 30s. Use `sleep 30` + an elapsed assertion;
  Unix-only (`//go:build !windows`) since `sleep` is absent on Windows. The contract lists no test, so this is a
  value-add, not a requirement.

---

## Confidence Score: 9/10

A surgical bug fix with a verbatim reference implementation (osRunner.Run — the exact two missing pieces are
spelled out), the verified fact that the constant rename is safe (defaultQueryTimeout has ZERO test/external
references — only 4 in-detect.go sites), the proof that the error-handling block is already correct (so it stays
unchanged — the timeout just makes ctx.Err() reachable), the proof that NO new import is needed (the shared
constant is a time.Duration), the stale-comment text to remove + the accurate replacement, the root-cause
rationale (drift ⇒ share the constant), and 8 grep guards. The -1 from 10/10 reflects the one judgment call the
contract leaves open: export-the-constant (preferred, DRY, no-drift) vs. hardcode-3s (minimal, no-detect.go-edit).
The PRP recommends export with a clear rationale and spells out both, but an implementer who hardcoded would
still produce a working fix (just one that reintroduces the drift risk and needs a `time` import) — the grep
guards (esp. guard 6) catch the import difference. No new dep, no new file, no logic change to the proven
osRunner, no seam-architecture change.