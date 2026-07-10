name: "P1.M1.T1.S1 — Platform signal set helper (caughtSignals) + SIGHUP exit code (FR-K3)"
description: >
  Extend the signal handler's caught set from {SIGINT, SIGTERM} to {SIGINT, SIGTERM, SIGHUP} on
  Unix (Windows unchanged — FR-K7) via a build-tagged `caughtSignals()` helper, and add the
  conventional 128+signum SIGHUP exit code (129) to `exitCodeForSignal`. handle()/run()/
  RestoreDefault are signal-agnostic and need NO changes. This is the foundational signal change
  that lets a terminal hangup route through rescue + lock-release instead of a raw terminate.

---

## Goal

**Feature Goal**: Make Stagecoach's signal handler catch `SIGHUP` on Unix (alongside `SIGINT`/`SIGTERM`)
so that when the controlling terminal closes, the run routes through the existing rescue/exit path
(forward to child group → cancel ctx → rescue-or-exit → release lock file) instead of dying under
Go's default disposition and orphaning the lock file. Windows is unchanged (FR-K7: no SIGHUP, no
init-reparenting analog).

**Deliverable**: A build-tagged `caughtSignals() []os.Signal` helper (Unix returns SIGINT+SIGTERM+SIGHUP;
Windows returns SIGINT+SIGTERM), `signal.Notify` switched to use it, the `syscall.SIGHUP → 129` case
added to Unix's `exitCodeForSignal`, the now-unused `syscall` import removed from `signal.go`,
build-tagged unit tests proving the signal set + exit code on both platforms, and refreshed internal
doc comments. No public API is added in this subtask.

**Success Definition**:
- `go build ./...` AND `GOOS=windows go build ./...` AND `GOOS=linux go build ./...` all compile clean.
- On Unix, a SIGHUP routed through `handle()` with no snapshot exits **129**; with a snapshot armed it exits **3** (rescue) and forwards SIGHUP to the child group (unit tests prove both).
- On Unix, `caughtSignals()` contains `syscall.SIGHUP`; on Windows it returns exactly `{Interrupt, SIGTERM}` (build-tagged tests prove both).
- `RestoreDefault` still disarms SIGHUP for the update-ref window (no change needed — `signal.Stop` covers the whole Notify set).
- `go test ./internal/signal/...` passes on the host platform; `make test` and `make lint` pass.

## Why

- **FR-K3 / §18.4**: The current handler catches only `SIGINT`/`SIGTERM` (signal.go:103). When the
  controlling terminal closes (closing the lazygit TUI, quitting an IDE, a detaching terminal), the
  kernel delivers `SIGHUP` to the process group. Under Go's default disposition the process simply
  terminates, **skipping the deferred lock-file release** — the lock file is orphaned for the next
  run's reaper, and an in-flight snapshot's rescue recipe never prints. Catching SIGHUP and routing
  it through the rescue path fixes the terminal-hangup case (§9.27 FR-K3).
- This is the foundational piece of P1.M1 (Signal Infrastructure): the parent-death watchdog
  (FR-K1, P1.M2) and its `signal.Trigger` export (P1.M1.T2) build on top of a handler that already
  treats SIGHUP as a first-class rescue signal. Getting the platform signal set + exit code right here
  unblocks those siblings without changing any handler logic.

## What

**User-visible behavior**: Closing the terminal/IDE that launched stagecoach mid-generation no longer
silently orphans the run + lock file on Unix. Stagecoach prints the rescue recipe (if a snapshot was
armed) and exits cleanly with the lock file removed. On Windows nothing changes (no SIGHUP concept).

**Technical change (small, surgical):**
1. `signal_unix.go` (`//go:build !windows`): add `caughtSignals()` returning `{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}`; add `case syscall.SIGHUP: return 129` to `exitCodeForSignal`.
2. `signal_windows.go` (`//go:build windows`): add `caughtSignals()` returning `{os.Interrupt, syscall.SIGTERM}` (NO SIGHUP).
3. `signal.go`: replace line 103 `signal.Notify(h.ch, os.Interrupt, syscall.SIGTERM)` → `signal.Notify(h.ch, caughtSignals()...)`; **remove the now-unused `syscall` import**; refresh doc comments.
4. NEW `signal_unix_test.go` (`//go:build !windows`): SIGHUP exit code + caughtSignals content tests.
5. NEW `signal_windows_test.go` (`//go:build windows`): caughtSignals length/content test.
6. Internal doc comments only (no README/docs/ change — that is P1.M4.T2).

### Success Criteria
- [ ] `signal_unix.go::caughtSignals()` returns `{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}`
- [ ] `signal_windows.go::caughtSignals()` returns `{os.Interrupt, syscall.SIGTERM}` (no SIGHUP)
- [ ] `signal.go:103` uses `signal.Notify(h.ch, caughtSignals()...)` and `syscall` import is removed
- [ ] `exitCodeForSignal(syscall.SIGHUP) == 129` (Unix); pre-snapshot SIGHUP via `handle()` exits 129
- [ ] Post-snapshot SIGHUP via `handle()` exits 3 (rescue) and forwards SIGHUP to child group
- [ ] `GOOS=windows go build ./...` clean; `GOOS=linux go build ./...` clean; `go build ./...` clean
- [ ] `make test` + `make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — every file, line, build tag, the unused-import gotcha, the Windows-no-SIGHUP gotcha, the exact
test to mirror, and the scope boundaries against sibling subtasks are enumerated below (all verified by grep).

### Documentation & References

```yaml
- file: internal/signal/signal.go
  why: "The handler. Install() at line 76; signal.Notify at line 103 (THE line to change); handle() at 122-155 (signal-agnostic, NO change); RestoreDefault at 207-215 (signal.Stop disarms all caught signals, NO change)."
  pattern: "signal.Notify(h.ch, os.Interrupt, syscall.SIGTERM) at line 103 → signal.Notify(h.ch, caughtSignals()...)"
  gotcha: "signal.go imports syscall (line 24) and uses it ONLY at line 103. After the swap, REMOVE the syscall import or go build fails: 'imported and not used: syscall'. os and os/signal STAY (used elsewhere)."

- file: internal/signal/signal_unix.go
  why: "The //go:build !windows twin. exitCodeForSignal (func at line 21, switch 22-28) maps SIGINT→130, SIGTERM→143, default→1. Add caughtSignals() + the SIGHUP case here."
  pattern: "Mirror the existing exitCodeForSignal switch style (case os.Interrupt, syscall.SIGINT: return 130). caughtSignals() mirrors KillProcessGroup's placement (build-tagged helper)."
  gotcha: "syscall.SIGHUP EXISTS on Unix — safe to reference here. SIGHUP=signal 1 → 128+1=129."

- file: internal/signal/signal_windows.go
  why: "The //go:build windows twin. Add caughtSignals() here WITHOUT SIGHUP (FR-K7). exitCodeForSignal unchanged."
  gotcha: "syscall.SIGHUP DOES NOT EXIST in Go's Windows syscall package — referencing it here breaks the Windows build. caughtSignals() returns ONLY {os.Interrupt, syscall.SIGTERM}."

- file: internal/signal/signal_test.go
  why: "The test pattern to mirror. UN-TAGGED (compiles on all platforms). TestHandler_Exit143SIGTERM is the exact template for a SIGHUP exit test. installTestHandler(t, opts) helper + active.Store(nil) cleanup."
  pattern: "install handler with Exit func(code int){...} recorder + Out bytes.Buffer; call h.handle(syscall.SIGTERM); assert exitCode==143."
  gotcha: "signal_test.go is UN-TAGGED and compiles on Windows — you CANNOT put a syscall.SIGHUP reference here. SIGHUP tests MUST go in a NEW //go:build !windows file (signal_unix_test.go)."

- file: internal/signal/signal_integration_test.go
  why: "Precedent for a build-tagged test file in this package (its line 1 is //go:build !windows). Mirror this exact build-tag style for the new signal_unix_test.go / signal_windows_test.go."

- docfile: plan/014_37208f58ffa2/architecture/signal_extension.md
  why: "The full FR-K3 extension design (caughtSignals() code, the SIGHUP exit code, and the proof that handle()/run()/RestoreDefault need no changes)."
  section: "FR-K3 extension (SIGHUP)"

- docfile: plan/014_37208f58ffa2/P1M1T1S1/research/verification_deltas.md
  why: "The 7 deltas vs. the task description — ESPECIALLY the unused-syscall-import gotcha (Delta 1) and the SIGHUP-test-must-be-build-tagged gotcha (Delta 2). READ THIS before editing."
```

### Current Codebase tree (relevant slice)

```bash
internal/signal/
  signal.go                 # handler: Install()→signal.Notify:103; handle():122-155; RestoreDefault():207-215 (imports syscall, used ONLY at :103)
  signal_unix.go            # //go:build !windows — KillProcessGroup + exitCodeForSignal (SIGINT→130, SIGTERM→143, default→1)
  signal_windows.go         # //go:build windows  — KillProcessGroup + exitCodeForSignal (no SIGHUP)
  signal_test.go            # UN-TAGGED tests — TestHandler_Exit143SIGTERM etc. (uses SIGINT/SIGTERM only; compiles cross-platform)
  signal_integration_test.go# //go:build !windows — build-tagged test precedent
# NEW files (this subtask):
#   signal_unix_test.go     # //go:build !windows — SIGHUP exit code + caughtSignals content
#   signal_windows_test.go  # //go:build windows  — caughtSignals content (len 2)
cmd/stagecoach/main.go:59           # signal.Install(...) — the single Install callsite (no change needed)
internal/provider/executor.go:68    # signal.RegisterChild — no change
internal/generate/generate.go:526   # signal.RestoreDefault — no change
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (unused import): signal.go uses syscall in EXACTLY ONE place — line 103.
//   Replacing that line with caughtSignals()... makes the syscall import unused → go build FAILS.
//   REMOVE "syscall" from signal.go's import block. (grep "syscall\." internal/signal/signal.go == line 103 only.)

// CRITICAL (Windows build): syscall.SIGHUP does not exist on Windows. Any .go file that is NOT
//   //go:build !windows and references syscall.SIGHUP breaks `GOOS=windows go build`. The new
//   caughtSignals() that mentions SIGHUP MUST live in signal_unix.go (!windows). The SIGHUP test
//   MUST live in a NEW signal_unix_test.go (!windows). Never reference SIGHUP in signal.go /
//   signal_test.go / signal_windows.go.

// CRITICAL (test file build tags): signal_test.go is UN-TAGGED (compiles on Windows). It currently
//   only uses syscall.SIGINT/SIGTERM (both exist on Windows). Do NOT add a SIGHUP test there —
//   it would break the Windows test build. Put SIGHUP tests in signal_unix_test.go (!windows).

// exit-code convention: 128 + signum. SIGHUP=1 → 129 (mirrors SIGINT=2→130, SIGTERM=15→143).
//   §15.4's exit table (0/1/2/3/124) does NOT list these — they are signal-abort codes documented
//   inline in exitCodeForSignal, not in the CLI exit-code table. Keep them out of §15.4.

// RestoreDefault disarms SIGHUP FOR FREE: signal.Stop(h.ch) (signal.go:212) stops ALL signals in
//   the Notify set. Once caughtSignals() includes SIGHUP, the update-ref window is covered with
//   zero changes to handle()/run()/RestoreDefault. Do not touch those functions.
```

## Implementation Blueprint

### Data models and structure

No new types. One new unexported helper `caughtSignals() []os.Signal` per platform (build-tagged),
and one new `case` in the Unix `exitCodeForSignal` switch. The `Handler`/`Options` structs are
unchanged — SIGHUP flows through the existing signal-agnostic `handle()`.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/signal/signal_unix.go — add caughtSignals() + SIGHUP exit case
  - ADD helper (mirror KillProcessGroup's build-tagged-helper placement):
        // caughtSignals returns the signals this platform's handler intercepts (FR-K3). Unix adds
        // SIGHUP so a controlling-terminal hangup routes through rescue instead of a raw terminate.
        func caughtSignals() []os.Signal {
            return []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}
        }
  - EDIT exitCodeForSignal switch — add the SIGHUP case before default:
        case syscall.SIGHUP:
            return 129 // 128 + 1
  - UPDATE the exitCodeForSignal doc comment to mention SIGHUP→129 alongside SIGINT→130/SIGTERM→143.
  - NAMING: caughtSignals (unexported; only signal.go calls it). Matches the unexported-helper convention (KillProcessGroup/exitCodeForSignal).
  - DEPENDENCIES: none (this file is self-contained).

Task 2: MODIFY internal/signal/signal_windows.go — add caughtSignals() WITHOUT SIGHUP
  - ADD helper:
        // caughtSignals returns the signals this platform's handler intercepts (FR-K7). Windows has
        // no SIGHUP concept; the parent-death watchdog is Unix-only, so only SIGINT/SIGTERM are caught.
        func caughtSignals() []os.Signal {
            return []os.Signal{os.Interrupt, syscall.SIGTERM}
        }
  - DO NOT add a SIGHUP case to Windows' exitCodeForSignal (SIGHUP can't be referenced here; the branch is unreachable anyway).
  - DEPENDENCIES: Task 1 defines the shared signature (same name/signature in both build-tagged files — exactly like KillProcessGroup).

Task 3: MODIFY internal/signal/signal.go — use caughtSignals(); REMOVE unused syscall import; refresh docs
  - EDIT line 103:
        signal.Notify(h.ch, os.Interrupt, syscall.SIGTERM)   →   signal.Notify(h.ch, caughtSignals()...)
  - CRITICAL: REMOVE `"syscall"` from the import block (lines 17-24). It is now unused (line 103 was its only reference). Keep context/fmt/io/os/os/signal/sync/sync/atomic.
  - EDIT package doc comment (lines 1-2): "SIGINT/SIGTERM safety net" → mention SIGHUP (Unix), e.g. "...SIGINT/SIGTERM/(Unix)SIGHUP safety net (PRD §18.4 / §9.27 FR-K3)...".
  - REFRESH stale SIGINT/SIGTERM-only comments for accuracy: line 60 (chan comment), line 77 (Install doc), line 102 (Windows no-op note → reword: the caught set is platform-specific via caughtSignals()), line 212 (signal.Stop comment → "all caught signals").
  - DO NOT touch handle() (122-155), run() (111-119), RestoreDefault() (207-215), or any wrapper — they are signal-agnostic.
  - DEPENDENCIES: Tasks 1+2 (caughtSignals must exist in both build-tagged files or one platform won't compile).

Task 4: CREATE internal/signal/signal_unix_test.go (//go:build !windows) — SIGHUP tests
  - FILE HEADER: line 1 `//go:build !windows`, blank line, `package signal` (mirror signal_integration_test.go).
  - ADD direct table test of exitCodeForSignal covering SIGHUP→129 (and keep/regress SIGINT→130, SIGTERM→143):
        func TestExitCodeForSignal_Unix(t *testing.T) { cases := ...; for ... { if got := exitCodeForSignal(tc.sig); got != tc.want {...} } }
  - ADD caughtSignals content test:
        func TestCaughtSignals_UnixIncludesSIGHUP(t *testing.T) { sigs := caughtSignals(); /* assert contains syscall.SIGHUP, os.Interrupt, syscall.SIGTERM */ }
  - ADD handle()-level test mirroring TestHandler_Exit143SIGTERM:
        func TestHandler_Exit129SIGHUP(t *testing.T) { ... h.handle(syscall.SIGHUP); assert exitCode==129 (no snapshot) ... }
  - ADD a post-snapshot SIGHUP test asserting exit 3 + SIGHUP forwarded to the Kill recorder (mirror TestHandler_RescueOnSignalWithSnapshot but pass syscall.SIGHUP and assert killedSig==syscall.SIGHUP).
  - USE the existing installTestHandler(t, opts) helper from signal_test.go (same package — accessible). Import bytes/context/os/syscall/testing as needed.
  - DEPENDENCIES: Tasks 1+3.

Task 5: CREATE internal/signal/signal_windows_test.go (//go:build windows) — caughtSignals content
  - FILE HEADER: line 1 `//go:build windows`, blank line, `package signal`.
  - ADD:
        func TestCaughtSignals_WindowsExcludesSIGHUP(t *testing.T) {
            sigs := caughtSignals()
            // assert len(sigs)==2, contains os.Interrupt and syscall.SIGTERM.
            // Do NOT reference syscall.SIGHUP anywhere (it does not exist on Windows).
        }
  - NOTE: this test only runs under `GOOS=windows go test` (or Windows CI). It guards against accidentally adding SIGHUP to the Windows set later.
  - DEPENDENCIES: Task 2.
```

### Implementation Patterns & Key Details

```go
// PATTERN: the build-tagged platform helper (signal_unix.go + signal_windows.go)
// Two files, same unexported signature, divergent bodies — exactly like the existing
// KillProcessGroup pair. signal.go calls the generic name; the build tag selects the body.
func caughtSignals() []os.Signal {
    return []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP} // Unix
    // Windows twin omits SIGHUP (FR-K7): {os.Interrupt, syscall.SIGTERM}
}

// PATTERN: variadic spread into signal.Notify (signal.go:103)
signal.Notify(h.ch, caughtSignals()...)   // ... spreads the slice into Notify's ...os.Signal

// PATTERN: exit-code case mirrors the existing entries (signal_unix.go exitCodeForSignal)
case syscall.SIGHUP:
    return 129 // 128 + 1 (SIGHUP is signal 1); mirrors SIGINT→130 (128+2), SIGTERM→143 (128+15)

// PATTERN: build-tagged test file header (mirror signal_integration_test.go line 1)
//go:build !windows

package signal
```

### Integration Points

```yaml
NO database / config / routes / new public API changes. Pure internal signal-package change.

SIGNAL CAUGHT SET:
  - Unix:    {SIGINT, SIGTERM, SIGHUP}   (signal_unix.go::caughtSignals)
  - Windows: {SIGINT, SIGTERM}           (signal_windows.go::caughtSignals)
  - Wired at: signal.go:103 signal.Notify(h.ch, caughtSignals()...)

EXIT CODES (signal-abort, 128+signum; NOT added to §15.4 table):
  - SIGHUP → 129 (NEW, Unix exitCodeForSignal)
  - SIGINT → 130 (unchanged)
  - SIGTERM → 143 (unchanged)
  - post-snapshot any signal → 3 (unchanged; handle() hardcodes Exit(3))

DOWNSTREAM (this subtask ENABLES but does NOT build):
  - P1.M1.T2.S1 signal.Trigger(sig) export — will reuse the now-SIGHUP-aware handle() path.
  - P1.M2 watchdog — will call Trigger on parent death; SIGHUP is already a first-class rescue signal after this subtask.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Native build (host = linux/darwin)
go build ./...

# CRITICAL — cross-platform compile (the core risk of this change):
GOOS=linux   go build ./...
GOOS=windows go build ./...
GOOS=darwin  go build ./...
# Expected: all clean. If GOOS=windows fails, you referenced syscall.SIGHUP in a non-!windows file,
# OR you left the unused syscall import in a Windows-only path. If native fails, you forgot to
# remove the now-unused "syscall" import from signal.go.

# Vet (native + cross-platform — catches unused imports, shadowed vars)
go vet ./...
GOOS=windows go vet ./internal/signal/...

# Format
gofmt -l internal/signal/
# Expected: empty. If listed: gofmt -w internal/signal/

# Lint (project uses golangci-lint)
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Signal package (host platform)
go test ./internal/signal/... -v
# Expected: all pass, incl. new TestExitCodeForSignal_Unix / TestCaughtSignals_UnixIncludesSIGHUP /
#           TestHandler_Exit129SIGHUP (on Unix host).

# Cross-platform test COMPILE check (the Windows test file must at least compile):
GOOS=windows go test -c -o /dev/null ./internal/signal/
# Expected: compiles (the Windows test binary builds; SIGHUP is never referenced there).

# Whole suite (race detector — the project standard)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary
make build

# Manual SIGHUP smoke (Unix only) — proves the handler now catches SIGHUP end-to-end:
#   1. Run stagecoach against a repo with a slow/stub provider in one terminal: `stagecoach ...`
#   2. From another terminal send SIGHUP: `kill -HUP <stagecoach-pid>`
#   3. EXPECT: rescue recipe printed (if snapshot armed) OR clean exit; lock file removed; exit code
#      `echo $?` == 129 (pre-snapshot) or 3 (post-snapshot). Pre-fix this would have been a raw
#      terminate with an orphaned lock file.
#
# (The scripted e2e version — driving a launcher that delivers SIGHUP on close — is the deliverable
#  of P1.M4.T1.S1. Here the unit tests in Task 4 are the within-scope proof; a one-off manual check
#  is recommended but not a hard gate for this subtask.)

# Confirm RestoreDefault still disarms SIGHUP for the update-ref window (covered by existing
# TestHandler_RestoreDefaultStopsForward — re-run it; it must still pass unchanged).
go test ./internal/signal/... -run RestoreDefault -v
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: prove NO non-!windows file references syscall.SIGHUP (Windows build safety)
grep -rn "syscall.SIGHUP\|SIGHUP" --include="*.go" internal/signal/ | grep -v "_unix"
# Expected: every SIGHUP reference is in a *_unix.go file (signal_unix.go, signal_unix_test.go).
# signal.go / signal_test.go / signal_windows*.go must have ZERO syscall.SIGHUP references.

# Grep guard: prove signal.go no longer imports an unused syscall
grep -n '"syscall"' internal/signal/signal.go && echo "FAIL: syscall still imported in signal.go" || echo "OK: syscall import removed"
# (signal.go MAY still mention SIGHUP in COMMENTS — that's fine; only the import + syscall.X code matters.)

# Scope-boundary guard: this subtask added NO new exported symbol (Trigger is T2's job)
grep -n "func Trigger" internal/signal/*.go
# Expected: empty (signal.Trigger lands in P1.M1.T2.S1, NOT here).

# Confirm Windows exitCodeForSignal was NOT given a SIGHUP case (impossible there)
grep -n "SIGHUP" internal/signal/signal_windows.go
# Expected: empty.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `GOOS=windows go build ./...` clean; `GOOS=linux go build ./...` clean; `GOOS=darwin go build ./...` clean
- [ ] `go vet ./...` clean; `GOOS=windows go vet ./internal/signal/...` clean
- [ ] `gofmt -l internal/signal/` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) all pass, incl. new SIGHUP tests

### Feature Validation
- [ ] `caughtSignals()` on Unix contains `syscall.SIGHUP` (TestCaughtSignals_UnixIncludesSIGHUP)
- [ ] `caughtSignals()` on Windows returns exactly `{Interrupt, SIGTERM}`, no SIGHUP (TestCaughtSignals_WindowsExcludesSIGHUP)
- [ ] `exitCodeForSignal(syscall.SIGHUP) == 129` (TestExitCodeForSignal_Unix)
- [ ] Pre-snapshot `h.handle(syscall.SIGHUP)` exits 129 (TestHandler_Exit129SIGHUP)
- [ ] Post-snapshot `h.handle(syscall.SIGHUP)` exits 3 + forwards SIGHUP to child group
- [ ] `signal.go:103` uses `caughtSignals()...` and `syscall` import removed
- [ ] Existing RestoreDefault/handle tests still pass unchanged

### Scope-Boundary Validation
- [ ] NO `signal.Trigger` added (that's P1.M1.T2.S1)
- [ ] NO watchdog / no_parent_watchdog config / lock-status changes (P1.M2/P1.M3)
- [ ] NO README.md / docs/ changes (P1.M4.T2) — only internal code comments updated
- [ ] handle()/run()/RestoreDefault()/wrappers UNCHANGED

### Code Quality & Docs
- [ ] caughtSignals() follows the existing build-tagged-helper convention (KillProcessGroup pair)
- [ ] Doc comments refreshed (signal.go package doc, exitCodeForSignal doc, signal.go line 60/77/102/212)
- [ ] New test files use the exact build-tag header style of signal_integration_test.go

---

## Anti-Patterns to Avoid

- ❌ Don't leave the `syscall` import in signal.go after swapping line 103 — it becomes unused and `go build` fails. Remove it. (This is the #1 one-pass failure mode.)
- ❌ Don't reference `syscall.SIGHUP` in any file that is NOT `//go:build !windows` (signal.go, signal_test.go, signal_windows*.go). It breaks the Windows build.
- ❌ Don't add the SIGHUP test to signal_test.go — that file is un-tagged and compiles on Windows where SIGHUP doesn't exist. Use a new `//go:build !windows` signal_unix_test.go.
- ❌ Don't touch handle()/run()/RestoreDefault()/RegisterChild/SetSnapshot — they are signal-agnostic; SIGHUP flows through them unchanged. signal.Stop in RestoreDefault disarms SIGHUP for free.
- ❌ Don't add `signal.Trigger` (that's P1.M1.T2.S1), the watchdog (P1.M2), or lock-status (P1.M3) here.
- ❌ Don't add 129 to the §15.4 exit-code table or change README/docs — external docs are P1.M4.T2; this subtask updates only internal code comments.
- ❌ Don't add a SIGHUP case to Windows' exitCodeForSignal — unreachable and uncompilable there.
- ❌ Don't skip the cross-compile (`GOOS=windows go build`) — native-only testing would miss a Windows breakage that CI (§20.4 matrix) will catch.

---

## Confidence Score: 9/10

One-pass success is very high: the change is small and surgical, every site is enumerated with
verified line numbers, and the architecture doc already worked out the exact `caughtSignals()`
design plus the proof that no handler logic changes. The -1 is for the two compile gotchas that
the task description omitted but this PRP foregrounds: (1) the now-unused `syscall` import in
signal.go (hard build blocker if missed), and (2) the requirement that SIGHUP tests live in a
build-tagged `signal_unix_test.go` (a SIGHUP test in the un-tagged signal_test.go silently breaks
`GOOS=windows`). Both are called out as CRITICAL in the gotchas and guarded by Level-4 grep checks.
