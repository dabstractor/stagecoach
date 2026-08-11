name: "P1.M4.T2.S1 — Document accepted Windows processAlive no-op (Option A: code comment + FR-K7 cite) (BUG-010)"
description: >
  A Mode-A DOCUMENTATION fix for BUG-010 (PRD §h3.9 / architecture §BUG-010). `internal/lock/lock_windows.go`
  `processAlive(pid, hostname)` unconditionally returns `true`, so `reapStaleLocks` never removes a stale lock
  file on Windows. The contract offers Option A (document the accepted behavior + cite FR-K7 — RECOMMENDED)
  or Option B (a real Windows liveness probe via OpenProcess+WaitForSingleObject — low-value, deferred). This
  PRP does Option A: REWRITE the `processAlive` doc comment to comprehensively + ACCURATELY document WHY it
  returns true. ONE FILE, comment-only (`internal/lock/lock_windows.go`). NO behavior change, NO new code,
  NO tests, NO user-facing docs. ⚠️ CRITICAL CORRECTION: the contract + the bugfix architecture + the
  prd_snapshot ALL claim "lock files are not created on the Windows commit path, so reaping is moot" — this is
  FACTUALLY FALSE. `lock.go:85` `os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)` has NO build gate, and
  `default_action.go:73` `lock.Acquire(repoDir)` runs on ALL platforms → the lock FILE IS created on Windows
  (only `flock` is the no-op). The strengthened comment MUST state the accurate reasoning (file is created but
  inert; deferred Release cleans normal exits; abnormal-exit litter is benign; §13.5 CAS is the correctness
  guarantee per FR-K7) — it MUST NOT propagate the false "files aren't created" claim. The FR-K7 cross-reference
  lives IN THE CODE COMMENT (cite "FR-K7" by name). The contract's "add a cross-reference to FR-K7 in
  spec/SPEC.md" is NOT executed autonomously: spec/SPEC.md is now a 67-line header that @-imports 7 split files
  (FR-K7 lives in spec/06-reliability.md §9.27 + §18.5, which ALREADY cover "watchdog is Unix-only + flock is a
  no-op + CAS is the guarantee"), AND AGENTS.md hard rule #2 forbids autonomous spec edits — if a spec gap is
  found, SURFACE it to the human (chat/issue), do not auto-edit. SCOPE: internal/lock/lock_windows.go ONLY.
  ZERO overlap with the parallel P1.M4.T1.S1 (BUG-009: lock.go/lock_test.go/lock_unix_test.go). Validates via
  `GOOS=windows go build ./internal/lock/` + `GOOS=windows go vet ./internal/lock/` (the file is
  `//go:build windows` — not compiled on a Linux dev host) + gofmt + grep guards.

---

## Goal

**Feature Goal**: Comprehensively + accurately document the accepted Windows `processAlive` no-op (always
returns `true` → `reapStaleLocks` never reaps on Windows) as the FR-K7-sanctioned behavior, so a future
maintainer understands WHY it is correct + conservative (not a bug or a TODO) — and is NOT misled by the
false "lock files aren't created on Windows" claim that the contract/architecture/prd_snapshot all repeat.

**Deliverable**: ONE edit — rewrite the `processAlive` doc comment in `internal/lock/lock_windows.go`
(comment-only; the `return true` body is UNCHANGED). The comment cites FR-K7 (PRD §9.27), states the accurate
file-creation/inert/CAS reasoning, satisfies the "never reap a live pid" invariant framing, and documents
Option B (a real Windows liveness probe) as a deferred low-value option.

**Success Definition**:
- `internal/lock/lock_windows.go` `processAlive` has a comprehensive doc comment that (a) cites **FR-K7** by
  name; (b) states `flock` is a no-op on Windows + the §13.5 CAS is the correctness guarantee; (c) ACCURATELY
  notes lock files ARE created on Windows (OpenFile O_CREATE is cross-platform) but are inert + cleaned by the
  deferred Release on normal exits, with abnormal-exit litter being benign; (d) states the "never reap a live
  pid" invariant is trivially satisfied (reap nothing = conservative); (e) documents Option B as a deferred
  low-value option, not a gap.
- The comment does NOT contain the false claim "lock files are not created" / "aren't created" (grep guard).
- `GOOS=windows go build ./internal/lock/` + `GOOS=windows go vet ./internal/lock/` clean (the file is
  `//go:build windows` — these are the real compile/vet gates; `go build ./...` on a Linux host skips it).
- `gofmt -l internal/lock/lock_windows.go` empty; `make test` + `make lint` green (comment-only; no behavior).
- `git status --porcelain` == `internal/lock/lock_windows.go` ONLY. NO edit to lock.go, lock_unix.go,
  orphan_*.go, any `*_test.go`, any `spec/*.md` file (AGENTS.md rule #2), PRD.md, tasks.json, or docs.

## User Persona (if applicable)

**Target User**: The Stagecoach maintainer (developer) reading `lock_windows.go` and wondering why
`processAlive` is a stub, or reviewing whether BUG-010 needs a real fix.
**Use Case**: A maintainer encounters the Windows lock path (a BUG-010 ticket, a Windows-litter report, a
code review) and reads the comment to decide: is this intentional? is it safe? should I implement Option B?
The comment answers all three (yes / yes / no — deferred) without needing to cross-reference the PRD.
**Pain Points Addressed**: the current comment is substantive but (a) doesn't cite FR-K7 (the requirement
that pins this as accepted), (b) implies "no reaping to do" (the accurate framing is "reaping is
non-functional, not unnecessary"), and (c) the surrounding architecture docs carry a FALSE "files aren't
created" claim that could mislead a maintainer into underestimating the litter behavior.

## Why

- **BUG-010 acceptance**: the architecture §BUG-010 explicitly says "This is documented behavior (FR-K7) ...
  Given the CAS guarantee, this is low priority." Option A (document) is the contract's RECOMMENDED fix. This
  item delivers it.
- **Correctness is not at stake**: on Windows the §13.5 CAS (update-ref compare-and-swap) is the SOLE
  correctness guarantee (FR-K7); a missing reaper cannot cause a wrong commit — only benign disk litter.
  Documenting this clearly closes BUG-010 without spending a platform-API dependency on a non-issue.
- **Stop the false claim from spreading**: the "lock files aren't created on Windows" line is repeated in 3
  planning artifacts. If it reached the code comment, a future maintainer might (wrongly) believe Windows
  produces zero lock files and miss a litter-accumulation report. The accurate comment is the anchor.

## What

A single comment rewrite in `internal/lock/lock_windows.go`. The `processAlive` function body (`return true`)
is UNCHANGED. No new code, no new deps, no tests, no spec/docs edits.

### Success Criteria
- [ ] `processAlive`'s doc comment cites **FR-K7** (PRD §9.27) as the requirement that sanctions the no-op.
- [ ] The comment states the §13.5 CAS is the correctness guarantee on Windows + `flock` is a no-op.
- [ ] The comment ACCURATELY states lock files ARE created on Windows (OpenFile O_CREATE is cross-platform;
      `default_action.go` calls `lock.Acquire` on all platforms) but are inert + cleaned by the deferred
      `Release` on normal exits; abnormal-exit litter (taskkill /F, crash) is benign + bounded.
- [ ] The comment states the "never reap a live pid" invariant (§18.5) is trivially satisfied (reap nothing).
- [ ] The comment documents Option B (OpenProcess + WaitForSingleObject / syscall.OpenProcess) as a deferred
      low-value option (litter-only; no correctness gain; adds a platform-API dep) — NOT a gap/TODO.
- [ ] The comment does NOT contain "not created" / "aren't created" (the false claim) — grep guard.
- [ ] `GOOS=windows go build ./internal/lock/` + `GOOS=windows go vet ./internal/lock/` clean.
- [ ] `gofmt -l internal/lock/lock_windows.go` empty; `make test` + `make lint` green.
- [ ] `git status --porcelain` == `internal/lock/lock_windows.go` ONLY.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the current comment (verbatim), the verbatim strengthened replacement, the FR-K7 wording, the
false-claim correction (with the code proof), the spec-is-split + AGENTS.md-rule-2 spec constraint, the
parallel-safety analysis, and the Windows-build-tag validation specifics.

### Documentation & References

```yaml
# MUST READ — the codebase-specific findings (the false-claim correction is the headline).
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M4T2S1/research/findings.md
  why: "§0 THE FALSE CLAIM — 'lock files aren't created on Windows' is WRONG (lock.go:85 OpenFile(O_CREATE)
        has no build gate; default_action.go:73 calls Acquire on all platforms). Do NOT propagate it; state
        the accurate 'file IS created but inert + CAS-guaranteed + deferred-Release-cleaned' reasoning. §1 the
        current comment; §2 FR-K7 wording; §3 the spec-is-split + AGENTS.md rule #2 (do NOT auto-edit spec);
        §4 lock_unix.go is accurate (out of scope); §5 parallel-safety; §6 Windows-build-tag validation; §7
        the verbatim strengthened comment."

# MUST READ — the file I EDIT (the current processAlive comment + the flock no-op above it).
- file: internal/lock/lock_windows.go
  why: "The `//go:build windows` file. processAlive (~line 19) returns true; its comment is what I rewrite.
        The flock() no-op comment above it (also cites §13.5 CAS) is the cross-reference for 'see flock above'.
        KEEP the flock/isWouldBlock comments + bodies UNCHANGED; rewrite ONLY the processAlive comment."
  pattern: "Comment cites section anchors (§13.5, §18.5) + cross-refs the twin (lock_unix.go) + names the
            caller (reapStaleLocks). The strengthened version ADDS FR-K7 + the accurate file-creation note."
  gotcha: "The file is //go:build windows — `go build ./...` on Linux DOES NOT compile it. Validate with
           GOOS=windows go build/vet. gofmt works cross-platform."

# MUST READ — the proof the file IS created on Windows (do not propagate the false claim).
- file: internal/lock/lock.go
  why: "Line 85: `os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)` — NO build gate → creates the lock file on
        EVERY platform (incl. Windows). Line 90: flock(int(f.Fd())) — the ONLY call that's a no-op on Windows.
        Line 129: reapStaleLocks (the caller of processAlive). The deferred Release (later in the file) removes
        the file on normal exits. This is the code proof that 'files aren't created' is false."
- file: internal/cmd/default_action.go
  why: "Line 73: `locker, lockErr := lock.Acquire(repoDir)` — NO build gate → the Windows commit path DOES
        reach Acquire → DOES create the lock file. Confirms the file is created on Windows."

# MUST READ — the cross-platform twin (accurate; out of scope, but the comment cross-refs it).
- file: internal/lock/lock_unix.go
  why: "processAlive uses syscall.Kill(pid, 0) (ESRCH→dead, EPERM→alive-different-user, foreign-host→true).
        Its comment already says 'lock_windows.go provides an always-true twin (flock is a no-op there → no
        reaping; the §13.5 CAS is the guarantee).' ACCURATE — leave it unchanged (tight scope). Optionally add
        a reciprocal 'FR-K7' cite, but NOT required."

# CONTEXT — FR-K7 wording (the requirement the comment cites).
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/prd_snapshot.md
  section: "§9.27 FR-K7 — 'the watchdog is Unix-only; on Windows flock is already a no-op and the §13.5 CAS
            is the guarantee (FR-K7).' (Also §18.5's 'Orphaned-but-alive' paragraph: 'the watchdog is Unix-only
            ... (FR-K6–K7).')"
  why: "The exact requirement text to cite. FR-K7 is the authority that the Windows no-op is ACCEPTED, not a bug."

# CONTEXT — BUG-010 as scoped by the architecture (note its false claim; correct it in the comment).
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/architecture/bugfix_subsystems.md
  section: "§BUG-010 (line ~161): 'processAlive unconditionally returns true ... Per FR-K7 ... Lock files are
            not created on the Windows commit path, so this is effectively non-functional rather than harmful.'
            NOTE the false 'not created' clause — the comment MUST NOT repeat it; state the accurate version."

# CONTEXT — the AGENTS.md hard rule that forbids autonomous spec edits.
- file: AGENTS.md
  why: "Hard rule #2: 'Never modify spec/SPEC.md outside of an interactive session ... If you find a spec gap
        or contradiction while working, surface it (open an issue / raise it in chat) and stop — do not edit
        spec/SPEC.md to fix it on your own.' This is WHY the FR-K7 cross-reference goes in the CODE COMMENT, not
        an autonomous spec edit. (spec/SPEC.md is also now a 67-line header @-importing 7 split files; FR-K7
        lives in spec/06-reliability.md, which ALREADY covers it.)"
```

### Current Codebase tree (relevant slice)

```bash
internal/lock/
  lock_windows.go   # EDIT — rewrite the processAlive doc comment (comment-only; body `return true` unchanged)
  lock_unix.go      # READ-ONLY — the accurate cross-platform twin (syscall.Kill); out of scope
  lock.go           # READ-ONLY — Acquire (OpenFile O_CREATE, line 85) + reapStaleLocks (line 129, the caller) + deferred Release
  lock_test.go      # READ-ONLY (P1.M4.T1.S1 parallel owns lock.go/lock_test.go/lock_unix_test.go — BUG-009)
  lock_unix_test.go # READ-ONLY (P1.M4.T1.S1 parallel)
  orphan_unix.go    # READ-ONLY (P1.M4.T3.S1 planned — BUG-011)
  orphan_windows.go # READ-ONLY (P1.M4.T3.S1 planned)
spec/SPEC.md        # READ-ONLY — 67-line header @-importing split files; AGENTS.md rule #2 forbids autonomous edits
spec/06-reliability.md  # READ-ONLY — FR-K7 (§9.27) + §18.5 reaping/watchdog prose already cover the substance
AGENTS.md           # READ-ONLY — hard rule #2 (the spec-edit gate)
```

### Desired Codebase tree with files to be added/modified

```bash
internal/lock/lock_windows.go   # EDIT — rewrite the processAlive doc comment ONLY (cite FR-K7; accurate reasoning;
                                #        document Option B as deferred). Body unchanged. NOTHING ELSE.
# No new files. No tests. No spec/* edits. No docs. No PRD/tasks.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the "lock files aren't created on Windows" claim is FALSE — do NOT propagate it): lock.go:85
// os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) has NO build gate, and default_action.go:73 calls
// lock.Acquire(repoDir) on ALL platforms. The lock FILE IS created on Windows. Only flock(fd)
// (lock_windows.go) is the no-op. The accurate comment states the file is created but inert + cleaned by the
// deferred Release on normal exits; abnormal-exit litter is benign + bounded. The contract, the bugfix
// architecture §BUG-010, AND prd_snapshot.md:125 ALL repeat the false claim — do NOT copy it into the comment.

// CRITICAL (//go:build windows — not compiled on a Linux dev host): `go build ./...` and `go vet ./...`
// SKIP lock_windows.go on Linux. The real compile/vet gate is `GOOS=windows go build ./internal/lock/` +
// `GOOS=windows go vet ./internal/lock/`. Run BOTH or the edit is unverified. (gofmt -l works cross-platform.)

// CRITICAL (AGENTS.md hard rule #2 — do NOT autonomously edit spec/*): the contract's "add a cross-reference
// to FR-K7 in spec/SPEC.md" is NOT an autonomous edit. spec/SPEC.md is a 67-line header @-importing 7 split
// files; FR-K7 lives in spec/06-reliability.md (§9.27 + §18.5 ALREADY cover "watchdog Unix-only + flock
// no-op + CAS guarantee"). The FR-K7 cross-reference goes IN THE CODE COMMENT (cite "FR-K7" by name). If you
// believe the spec should explicitly call out the Windows processAlive no-op, SURFACE it to the human
// (chat/issue) per AGENTS.md rule #2 — do NOT auto-edit any spec/*.md file.

// GOTCHA (comment-only — zero behavior change): processAlive's body stays `return true`. Do NOT "fix" it to
// a real probe (Option B) — that is a deferred low-value enhancement, explicitly out of scope for this item.
// The comment DOCUMENTS Option B as deferred; it does not implement it.

// GOTCHA (do NOT touch the flock/isWouldBlock comments above processAlive): they are accurate (flock no-op +
// §13.5 CAS + isWouldBlock false). Rewrite ONLY the processAlive comment. The strengthened comment
// cross-refs "see flock above" — keep that pointer valid by leaving flock's comment intact.

// GOTCHA (scope fence — parallel items own the other lock files): P1.M4.T1.S1 (BUG-009, parallel) edits
// lock.go + lock_test.go + lock_unix_test.go. P1.M4.T3.S1 (BUG-011, planned) edits orphan_unix.go. This item
// edits lock_windows.go ONLY — zero overlap. Do NOT touch any other lock file or test.
```

## Implementation Blueprint

### Data models and structure
None — comment-only. No types, no code, no deps.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/lock/lock_windows.go — rewrite the processAlive doc comment
  - LOCATE the processAlive func (~line 19). Its current comment is the 5-line "conservative no-op on Windows
    ... flock is a no-op ... §13.5 CAS ... never reap a live pid ... Cross-platform twin ... reapStaleLocks"
    block. REPLACE that comment block with the strengthened version BELOW. KEEP the func signature + body
    (`return true`) UNCHANGED. KEEP the flock() + isWouldBlock() comments/bodies above it UNCHANGED.
  - THE STRENGTHENED COMMENT (verbatim substance — wording may be lightly trimmed, but ALL 5 content points
    MUST be present: FR-K7 cite / CAS-guarantee + flock-no-op / ACCURATE file-creation + inert + deferred-
    Release + benign-litter / never-reap-a-live-pid-trivially-satisfied / Option-B-deferred):
      // processAlive is an intentional no-op on Windows: it always reports the pid as alive, so
      // reapStaleLocks never removes a lock file on Windows. This is ACCEPTED behavior per FR-K7
      // (PRD §9.27): the parent-death watchdog is Unix-only, and on Windows flock is already a no-op
      // (see flock above), so the §13.5 CAS (update-ref HEAD compare-and-swap) is the SOLE correctness
      // guarantee — a leftover lock file is inert disk litter, never a correctness or contention hazard.
      //
      // Why returning true (never reap) is correct + conservative:
      //   - flock is a no-op on Windows (no inode-bound-flock hazard), so unlinking a lock file is always
      //     SAFE here — but there is likewise no flock-based staleness to detect, so a real liveness probe
      //     adds no safety. The §13.5 CAS guarantees a killed/aborted run can never land a wrong commit,
      //     regardless of any leftover lock file.
      //   - the "never reap a live pid" safety invariant (PRD §18.5) is trivially satisfied: reap nothing
      //     ⇒ never reap a live pid. This is the conservative choice — it cannot cause a wrong reaping,
      //     only leave benign litter.
      //   - lock files ARE created on Windows (Acquire's os.OpenFile(O_CREATE) runs cross-platform), but
      //     they are inert: the deferred Release removes the file on every NORMAL exit, and only an
      //     ABNORMAL exit (taskkill /F, a crash) leaves a file behind. That litter is harmless (the CAS is
      //     the guarantee) and bounded (normal exits dominate); it is the accepted tradeoff of FR-K7's
      //     Unix-only-watchdog scope.
      //
      // A real Windows liveness probe (OpenProcess + WaitForSingleObject via golang.org/x/sys/windows, or
      // syscall.OpenProcess) would let reaping work here, but it is low-value: it would only clean benign
      // litter the CAS already renders harmless, adding a platform-API dependency for no correctness gain.
      // Documented here as a deferred option (FR-K7's accepted scope), not a gap.
      //
      // Cross-platform twin of lock_unix.go's processAlive (syscall.Kill(pid, 0)); used by reapStaleLocks.
  - NAMING/PLACEMENT: the comment stays immediately above `func processAlive(pid int, hostname string) bool`.
    No new identifiers. No import changes (comment-only — lock_windows.go imports nothing today).
  - GOTCHA: the comment MUST contain "FR-K7" + "§13.5" + "§18.5" + "os.OpenFile(O_CREATE)" (the accurate
    file-creation note) and MUST NOT contain "not created" / "aren't created" (the false claim). Grep guards
    enforce all of these.

Task 2: VERIFY — Windows build/vet + format + full suite/lint + grep guards
  - GOOS=windows go build ./internal/lock/    # the //go:build windows file compiles (NOT exercised by plain `go build ./...` on Linux)
  - GOOS=windows go vet ./internal/lock/      # vet the windows file
  - gofmt -l internal/lock/lock_windows.go    # empty
  - make test && make lint && make build      # comment-only; no behavior change; confirm nothing breaks
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN (the strengthened comment — 5 mandatory content points):
// 1. FR-K7 cite: "ACCEPTED behavior per FR-K7 (PRD §9.27)".
// 2. CAS guarantee + flock no-op: "the §13.5 CAS is the SOLE correctness guarantee; flock is already a no-op".
// 3. ACCURATE file note: "lock files ARE created on Windows (Acquire's os.OpenFile(O_CREATE) runs
//    cross-platform), but they are inert ... deferred Release removes the file on every NORMAL exit ...
//    ABNORMAL exit leaves a file behind ... harmless (the CAS) + bounded".  ← corrects the false claim.
// 4. Invariant: "the 'never reap a live pid' safety invariant (PRD §18.5) is trivially satisfied (reap nothing)".
// 5. Option B deferred: "A real Windows liveness probe (OpenProcess + WaitForSingleObject ... ) is low-value
//    ... Documented here as a deferred option, not a gap."

// PATTERN (cross-refs the comment must carry): "see flock above" (the no-op twin in the same file),
// "Cross-platform twin of lock_unix.go's processAlive (syscall.Kill(pid, 0))", "used by reapStaleLocks".
// These match the existing comment's cross-ref style — preserve them.
```

### Integration Points

```yaml
CODE (the single edit):
  - internal/lock/lock_windows.go processAlive doc comment — rewritten (body `return true` UNCHANGED).
NO database / migration / routes / config / exitcode / CLI / provider / commit-path change. Comment-only.
SPEC (DO NOT autonomously edit): the FR-K7 cross-reference is IN THE CODE COMMENT. spec/SPEC.md (67-line
  header) + spec/06-reliability.md ALREADY cover FR-K7 (§9.27 + §18.5). AGENTS.md hard rule #2 forbids
  autonomous spec edits; if a spec gap is believed, SURFACE it to the human (chat/issue), do NOT auto-edit.
SCOPE FENCES:
  - Touches ONLY: internal/lock/lock_windows.go.
  - Does NOT touch: lock.go, lock_unix.go, orphan_unix.go, orphan_windows.go, any *_test.go,
    any spec/*.md file, PRD.md, tasks.json, prd_snapshot.md, docs/*, AGENTS.md, go.mod.
  - Parallel-safe: P1.M4.T1.S1 (BUG-009) owns lock.go/lock_test.go/lock_unix_test.go; P1.M4.T3.S1 (BUG-011)
    owns orphan_*.go; P1.M4.T4.S1 owns README/docs — ALL different files, zero overlap.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# The file is //go:build windows — plain `go build ./...` on Linux SKIPS it. Compile it explicitly:
GOOS=windows go build ./internal/lock/
# Expected: clean. (A comment-only edit can't break the build, but this proves the file still parses under
#           the windows build tag — the real gate, since the dev host won't compile it otherwise.)

# Vet the windows file (same reason — must set GOOS).
GOOS=windows go vet ./internal/lock/
# Expected: clean.

# Format (cross-platform — gofmt parses regardless of build tags).
gofmt -l internal/lock/lock_windows.go
# Expected: empty. If listed: gofmt -w internal/lock/lock_windows.go.

# Full suite + lint + build (comment-only; confirm no collateral — the file isn't compiled on Linux, so
# `make test`/`make lint` won't even see the change, but run them to prove the package is healthy).
make test && make lint && make build
# Expected: all green.

# Scope guard: ONLY lock_windows.go.
git status --porcelain
# Expected: internal/lock/lock_windows.go ONLY.
```

### Level 2: Unit Tests (Component Validation)

```bash
# N/A — comment-only; no behavior change; no new tests. There is no lock_windows_test.go (only lock_unix_test.go).
# The Windows reaping no-op has no test today (the Unix twin is tested in lock_unix_test.go). This item adds
# none (a comment is not testable). Confirm the existing lock tests stay green:
go test ./internal/lock/ -race
# Expected: green (unchanged — the comment doesn't affect behavior).
```

### Level 3: Integration Testing (System Validation)

```bash
# N/A — comment-only. No command path is affected. (Optionally cross-compile the whole module for windows to
# prove nothing else broke under that GOOS — but the per-package GOOS=windows build in Level 1 is sufficient.)
GOOS=windows go build ./...
# Expected: clean.
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: the comment cites FR-K7 (the requirement that sanctions the no-op).
grep -n 'FR-K7' internal/lock/lock_windows.go
# Expected: ≥1 hit (in the processAlive comment).

# Guard 2: the comment cites the §13.5 CAS guarantee + §18.5 invariant.
grep -n '§13.5' internal/lock/lock_windows.go   # ≥1 (the CAS guarantee)
grep -n '§18.5' internal/lock/lock_windows.go   # ≥1 (the never-reap-a-live-pid invariant)

# Guard 3: the comment does NOT propagate the FALSE "files aren't created" claim.
grep -niE 'not created|aren.t created|isn.t created|never created' internal/lock/lock_windows.go && echo "FAIL: false 'files not created' claim present" || echo "OK: no false claim"

# Guard 4: the comment states the ACCURATE file-creation note (OpenFile O_CREATE is cross-platform).
grep -n 'os.OpenFile(O_CREATE)\|OpenFile(O_CREATE)\|created on Windows' internal/lock/lock_windows.go
# Expected: ≥1 hit (the accurate "files ARE created" note).

# Guard 5: the comment documents Option B as DEFERRED (not implemented, not a TODO/gap).
grep -niE 'OpenProcess|WaitForSingleObject|deferred option|low-value' internal/lock/lock_windows.go
# Expected: ≥1 hit (Option B named as deferred).

# Guard 6: the processAlive BODY is unchanged (still `return true` — comment-only).
grep -nA1 'func processAlive' internal/lock/lock_windows.go | grep 'return true'
# Expected: `return true` immediately follows the signature.

# Guard 7: scope — ONLY lock_windows.go.
git status --porcelain
# Expected: internal/lock/lock_windows.go ONLY.
git diff --name-only | grep -vE '^internal/lock/lock_windows\.go$' && echo "FAIL: out-of-scope file" || echo "OK: scope clean"

# Guard 8: NO spec/* file was edited (AGENTS.md rule #2).
git diff --name-only | grep -E '^spec/' && echo "FAIL: spec/ edited (AGENTS.md rule #2 violation)" || echo "OK: spec/ untouched"

# Guard 9: NO other lock file edited (parallel items own them).
git diff --name-only | grep -E '^internal/lock/(lock\.go|lock_unix\.go|orphan_|.*_test\.go)' && echo "FAIL: edited a parallel-owned lock file" || echo "OK: only lock_windows.go"
```

## Final Validation Checklist

### Technical Validation
- [ ] `GOOS=windows go build ./internal/lock/` clean (the `//go:build windows` file compiles)
- [ ] `GOOS=windows go vet ./internal/lock/` clean
- [ ] `gofmt -l internal/lock/lock_windows.go` empty
- [ ] `make test` + `make lint` + `make build` green (comment-only; no collateral)

### Feature Validation (the 5 content points)
- [ ] FR-K7 cited (grep guard 1)
- [ ] §13.5 CAS guarantee + §18.5 invariant cited (grep guard 2)
- [ ] NO false "files not created" claim (grep guard 3); ACCURATE file-creation note present (grep guard 4)
- [ ] Option B documented as deferred (grep guard 5)
- [ ] processAlive body unchanged (`return true`) (grep guard 6)

### Scope-Boundary Validation
- [ ] `git status` shows ONLY `internal/lock/lock_windows.go` (grep guards 7, 9)
- [ ] NO spec/* file edited — AGENTS.md rule #2 respected (grep guard 8)
- [ ] NO edit to lock.go/lock_unix.go/orphan_*.go/*_test.go (parallel items), PRD.md, tasks.json, docs, go.mod
- [ ] The FR-K7 cross-reference lives in the CODE COMMENT (not an autonomous spec edit); any spec gap surfaced to the human, not auto-fixed

### Code Quality & Docs
- [ ] Comment is self-contained (a maintainer understands WHY without cross-referencing the PRD)
- [ ] Comment cross-refs `flock above`, the lock_unix.go twin, and `reapStaleLocks` (the caller)
- [ ] Comment does NOT propagate the false "files aren't created on Windows" claim from the contract/architecture

---

## Anti-Patterns to Avoid

- ❌ Don't propagate the false "lock files are not created on the Windows commit path" claim. The contract,
  the bugfix architecture §BUG-010, AND prd_snapshot.md:125 ALL repeat it — it is wrong (lock.go:85 OpenFile
  O_CREATE has no build gate; default_action.go:73 calls Acquire on all platforms). State the accurate
  version (file IS created but inert + deferred-Release-cleaned; abnormal-exit litter is benign + bounded).
- ❌ Don't autonomously edit `spec/SPEC.md` or any `spec/*.md` file. AGENTS.md hard rule #2 forbids it; the spec
  is also split (FR-K7 is in spec/06-reliability.md, which ALREADY covers it). The FR-K7 cross-reference goes
  IN THE CODE COMMENT. If you believe the spec needs an explicit Windows-no-op callout, SURFACE it to the
  human (chat/issue) — do not auto-edit.
- ❌ Don't implement Option B (a real Windows liveness probe). It is a deferred low-value enhancement
  (litter-only; no correctness gain; adds a platform-API dep). The comment DOCUMENTS it as deferred; this item
  is Option A (documentation only). Changing the body to a real probe is out of scope and would add a
  dependency for no safety benefit.
- ❌ Don't validate with only `go build ./...` on a Linux host. lock_windows.go is `//go:build windows` — Linux
  skips it. Use `GOOS=windows go build ./internal/lock/` + `GOOS=windows go vet ./internal/lock/` (the real
  gates). gofmt works cross-platform.
- ❌ Don't touch the `flock()`/`isWouldBlock()` comments or bodies above processAlive. They are accurate (flock
  no-op + §13.5 CAS + isWouldBlock false). The strengthened comment cross-refs "see flock above" — keep that
  pointer valid by leaving flock's comment intact.
- ❌ Don't edit lock.go, lock_unix.go, orphan_*.go, or any *_test.go. P1.M4.T1.S1 (BUG-009, parallel) owns
  lock.go/lock_test.go/lock_unix_test.go; P1.M4.T3.S1 (BUG-011) owns orphan_*.go. This item is lock_windows.go
  ONLY — zero overlap is the parallel-safety guarantee.
- ❌ Don't add tests. A comment is not testable; the Windows reaping no-op has no test today (the Unix twin is
  tested in lock_unix_test.go). Adding a Windows test is out of scope (and would require a //go:build windows
  _test.go that doesn't exist). Comment-only.
- ❌ Don't change `processAlive`'s body. It stays `return true`. The deliverable is the COMMENT. Editing the
  body (even to "fix" it) is Option B, which is explicitly deferred.
- ❌ Don't drop the existing cross-references. The current comment cross-refs `flock above`, the lock_unix.go
  twin, and `reapStaleLocks`. Preserve all three — they orient the maintainer.