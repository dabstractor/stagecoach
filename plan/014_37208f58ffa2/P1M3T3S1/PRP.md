name: "P1.M3.T3.S1 — Refactor handleLockContention: lock path on its own line + orphaned-holder hint (FR-K5)"
description: >
  The FR-K5 / PRD §9.27 Busy-message reformat (default_action.go handleLockContention). TODAY the lock
  path is buried mid-sentence ("…re-run stagecoach after it finishes. Lock: <path>."). FR-K5 (PRD line
  568) requires it on its OWN line so a script/hurried user can copy it, AND — when the holder APPEARS
  orphaned (its launcher exited, the §9.27 "lock stays forever" case) — an extra hint telling the user
  they may safely `kill <pid>` / `rm <path>` (pointing at `stagecoach lock status`). This is a SURGICAL
  edit: (1) ADD one exported helper `lock.IsOrphaned(contents LockContents) bool` to internal/lock/lock.go
  (mirrors Status's alive→orphan logic; reuses the LANDED unexported processAlive + appearsOrphaned —
  NO edit to Status or the orphan_*.go twins); (2) ADD one package-level func-var SEAM
  `var orphanChecker = lock.IsOrphaned` to default_action.go (the codebase's proven testability idiom —
  see config_init_interactive.go:20 `interactiveStdinIsTTY`); (3) REWRITE ONLY the Busy `fmt.Fprintf`
  block (default_action.go:326-330) — main message (Lock removed) + blank line + `Lock: <path>` on its
  own line + conditional orphan hint. The no-op fast path (303-309), the Issue-4b fallback diagnostics
  (314-325), and the SILENT `exitcode.New(exitcode.Busy, nil)` return are PRESERVED UNCHANGED. Tests in
  lock_contention_test.go are UPDATED for the new format + 3 NEW seam-driven hint tests (present / absent
  / empty-pid-guarded); a NEW `lock.IsOrphaned` unit test joins the orphan tests in lock_unix_test.go.
  NO overlap with P1.M3.T2.S1 (lock.go subcommand — different file, parallel-safe) or P1.M4.T1.S1 (the
  real-orphan E2E — a genuine ppid==1 holder is flaky/OS-dependent, so the unit tests drive the hint via
  the seam, not a real orphan). [Mode A] godoc on IsOrphaned; README sync is P1.M4.T2.S1.

---

## Goal

**Feature Goal**: Make Stagecoach's §18.5 "another stagecoach run is in progress" (Busy) message satisfy
FR-K5 (PRD §9.27, line 568): the lock path is on its OWN line (copy-pasteable, not buried mid-sentence),
and when the holder APPEARS orphaned (its launcher exited without killing it — the "lock stays forever"
hazard), the message adds a hint that the user may safely `kill <pid>` or `rm <path>`, pointing at
`stagecoach lock status` (FR-K4) for confirmation. The no-op fast path (snapshot match → exit 0) and the
Issue-4b fallback diagnostics (`<unknown>` / `an unknown repo` substitutions) are byte-preserving.

**Deliverable**:
1. `internal/lock/lock.go` — ADD `func IsOrphaned(contents LockContents) bool` (exported; self-contained;
   reuses processAlive + appearsOrphaned; mirrors Status's alive→orphan order). Does NOT modify Status.
2. `internal/cmd/default_action.go` — ADD `var orphanChecker = lock.IsOrphaned` (testability seam) and
   REWRITE ONLY the Busy `fmt.Fprintf` block (lines 326-330): main message (Lock clause removed) → newline
   → blank line → `Lock: <path>` on its own line → conditional orphan hint. No-op fast path, fallback
   diagnostics, and the SILENT Busy return are UNCHANGED.
3. `internal/lock/lock_unix_test.go` — ADD a `lock.IsOrphaned` unit test (empty/malformed pid → false;
   self → appearsOrphaned(getpid())).
4. `internal/cmd/lock_contention_test.go` — UPDATE assertions for the new format (Lock on own line) and
   ADD 3 seam-driven tests (hint present / hint absent / hint empty-pid-guarded).

**Success Definition**:
- A Busy contention now prints (NOT orphaned):
  ```
  stagecoach: another stagecoach run is already in progress on <repo> (pid <N> on <host>).
  Your newly-staged changes will remain staged — re-run stagecoach after it finishes.

  Lock: <path>
  ```
  and exits 5 (Busy), SILENT (`exitcode.New(exitcode.Busy, nil)` — main does not double-print).
- When the holder's pid is non-empty AND `lock.IsOrphaned(heldErr.Contents)` is true, ONE additional
  line follows `Lock: <path>`:
  ```
  The holder's launcher appears to have exited — it may be orphaned and holding this lock uselessly. You may safely `kill <N>` or `rm <path>` to clear it. See `stagecoach lock status`.
  ```
  where `<N>` is the REAL holder pid (`heldErr.Contents.Pid`, guaranteed non-empty by the guard) and
  `<path>` is `heldErr.Path`.
- The no-op fast path (snapshot == contender WriteTree → `nothing to do…` exit 0) is unchanged.
- The Issue-4b fallbacks (`an unknown repo`, `<unknown>`) are unchanged; the message NEVER renders the
  broken `on  (pid  on )` pattern (the existing `strings.Contains(msg, "  ")` guard still passes — the
  blank line is `\n\n`, two NEWLINES, NOT two spaces).
- An EMPTY holder pid NEVER emits the hint (the `Pid != ""` guard is independent of the predicate).
- `go build ./...` (+ GOOS=linux/darwin/windows) clean; `go vet` clean; `gofmt -l` empty;
  `go test ./internal/cmd/ ./internal/lock/ -race` green; `make test` + `make lint` + `make build` clean.
- `git status --porcelain` shows ONLY the 4 files above (scope guard).

## User Persona (if applicable)

**Target User**: A developer whose `stagecoach` invocation hit the §18.5 Busy message ("another run is in
progress") — especially the §9.27 "the lock stays forever" case where the holder's launcher (lazygit TUI,
IDE, detaching terminal) closed without killing the child.

**Use Case**: The user wants to (a) copy the lock path directly (scripts, `rm`), and (b) be TOLD when the
holder looks orphaned so they can safely `kill`/`rm` instead of waiting on a run nobody will ever see.

**User Journey**: user runs `stagecoach` → Busy message prints `Lock: <path>` on its own line (copy-paste)
+ (if orphaned) the kill/rm hint → user runs `stagecoach lock status` to confirm → user `kill <N>` or
`rm <path>` → next run unblocked.

**Pain Points Addressed**: FR-K5 — the lock path buried mid-sentence (not scriptable) and the silent
"orphaned-but-alive" holder that holds the lock forever with no user-facing signal.

## Why

- **FR-K5 / §9.27 (PRD line 568)**: "The §18.5 contention message already ends with `Lock: <path>`; it is
  reformatted so the lock path is on its own line (not buried mid-sentence)… When the holder appears
  orphaned (FR-K4's test), the message additionally says the holder's launcher has exited and the user
  may safely `kill` the pid or `rm` the lock file." This item is exactly that reformat.
- **Consistency with `lock status` (FR-K4, P1.M3.T2.S1)**: both surfaces must agree on "is the holder
  orphaned?" The new `lock.IsOrphaned` helper mirrors `lock.Status`'s internal alive→orphan logic exactly
  (same processAlive + appearsOrphaned calls in the same order) so the read path and the Busy hint can
  never disagree.
- **Self-termination preserved (FR52)**: the Busy message never force-breaks anything — it only ADVISES.
  The hint is an invitation for the USER to `kill`/`rm`; stagecoach itself still never reaps a live pid.
- **Dead holders are correctly excluded**: a dead pid holds no flock (auto-released on death; file reaped
  by pid-liveness on next Acquire), so `IsOrphaned` returns false for a dead holder (no kill hint for a
  process that's already gone). Only an ALIVE, reparented holder triggers the hint.

## What

**User-visible behavior** ( Busy path only — the no-op fast path's "nothing to do…" message is unchanged):
```
$ stagecoach          # contention, holder NOT orphaned (exit 5)
stagecoach: another stagecoach run is already in progress on /home/me/proj (pid 4242 on devbox).
Your newly-staged changes will remain staged — re-run stagecoach after it finishes.

Lock: /run/user/1000/stagecoach/locks/ab12….lock

$ stagecoach          # contention, holder APPEARS orphaned (launcher exited) (exit 5)
stagecoach: another stagecoach run is already in progress on /home/me/proj (pid 4242 on devbox).
Your newly-staged changes will remain staged — re-run stagecoach after it finishes.

Lock: /run/user/1000/stagecoach/locks/ab12….lock
The holder's launcher appears to have exited — it may be orphaned and holding this lock uselessly. You may safely `kill 4242` or `rm /run/user/1000/stagecoach/locks/ab12….lock` to clear it. See `stagecoach lock status`.
```

**Technical change**: reformat one `fmt.Fprintf` + add one exported lock helper + one testability seam.

### Success Criteria
- [ ] `internal/lock/lock.go` exports `func IsOrphaned(contents LockContents) bool` that returns false on
      empty/malformed pid (`strconv.Atoi` failure), false when `!processAlive(pid, contents.Hostname)`,
      and otherwise `appearsOrphaned(pid)`. `Status` is NOT modified.
- [ ] `internal/cmd/default_action.go` adds `var orphanChecker = lock.IsOrphaned` (package-level seam,
      mirrors `interactiveStdinIsTTY` at config_init_interactive.go:20).
- [ ] handleLockContention's Busy `fmt.Fprintf` block (lines 326-330) is rewritten: main message WITHOUT
      the trailing ` Lock: %s.`, two sentences on two lines (`\n`), a BLANK line (`\n\n`), then
      `Lock: <heldErr.Path>` on its own line; THEN, iff `heldErr.Contents.Pid != "" &&
      orphanChecker(heldErr.Contents)`, the hint line (verbatim wording) with `heldErr.Contents.Pid` and
      `heldErr.Path`.
- [ ] The no-op fast path (303-309), the Issue-4b fallback substitutions (314-325), and the SILENT
      `return exitcode.New(exitcode.Busy, nil)` are byte-identical to today.
- [ ] `internal/lock/lock_unix_test.go` adds a `lock.IsOrphaned` test (empty pid→false, malformed→false,
      self with Hostname==this host → `appearsOrphaned(os.Getpid())`).
- [ ] `internal/cmd/lock_contention_test.go` updated: the new "Lock: <path>" own-line format is asserted
      (incl. a regression guard that the old buried `Lock: %s.` form is GONE); 3 NEW tests drive the seam
      (hint present, hint absent, hint empty-pid-guarded). Existing tests still pass.
- [ ] [Mode A] Godoc on `IsOrphaned` states: read-only, conservative (false on any ambiguity), mirrors
      Status, consumed by the Busy hint (FR-K5) + `lock status` (FR-K4).
- [ ] `go build ./...` + GOOS=linux/darwin/windows clean; `go vet ./internal/cmd/... ./internal/lock/...`
      clean; `gofmt -l` empty on the 4 files.
- [ ] `go test ./internal/cmd/ ./internal/lock/ -race` green; `make test` + `make lint` + `make build` clean.
- [ ] `git status --porcelain` shows ONLY the 4 files (scope guard).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim current Busy `fmt.Fprintf` (quoted with line refs), the exact new format (with the
3-branch conditional), the verbatim hint wording, the exact `IsOrphaned` body (and WHY it takes
LockContents not a bare pid), the exact seam precedent (`interactiveStdinIsTTY`), the analysis that the
blank line `\n\n` does NOT trip the existing `"  "` double-space guard, the per-existing-test pass/fail
analysis, the exact test additions (seam-driven), the scope fences (4 files; no edit to Status /
orphan_*.go / lock.go-the-subcommand / root.go), and 10 grep guards.

### Documentation & References

```yaml
# MUST READ — the architecture spec for FR-K5 (the reformat target + orphan hint) + FR-K4 (the orphan
#              check IsOrphaned mirrors) + the lock-package export surface.
- docfile: plan/014_37208f58ffa2/architecture/lock_extension.md
  why: "'FR-K5: Busy message reformat' gives the target format (Lock on own line + the orphan hint) and
        lists the 3 changes. 'FR-K4: Lock status read path' documents the Status alive→orphan order that
        IsOrphaned must mirror, and the conservative appearsOrphaned invariant (false on any ambiguity)."
  critical: "The hint fires ONLY when the holder appears orphaned AND (per the item contract) the pid is
             non-empty. The fallback diagnostics + no-op fast path must be PRESERVED (the arch doc says so
             explicitly under FR-K5 'Changes: 3. Preserve the fallback diagnostics')."

# MUST READ — codebase-specific findings for THIS item (verbatim current Fprintf, call site, the seam
#              precedent, per-test pass/fail analysis, the \n\n-vs-'  ' guard, scope fences, validation).
- docfile: plan/014_37208f58ffa2/P1M3T3S1/research/findings.md
  why: "§0 the verbatim handleLockContention (lines 300-330, the 3 blocks); §1 the single call site;
        §2 why appearsOrphaned/processAlive are unreachable from cmd (→ need IsOrphaned); §3 the IsOrphaned
        body + why LockContents not pid; §4 the seam precedent (interactiveStdinIsTTY) + .golangci.yml has
        no gochecknoglobals; §5 the new format + verbatim hint wording + PRD line anchors (568, 2047);
        §6 the per-existing-test pass analysis (the \n\n guard); §7 the exact test additions; §8 scope
        fences; §9 validation commands."

# MUST READ — the parallel sibling PRP (P1.M3.T2.S1, the `stagecoach lock status` subcommand). It is
#              being implemented IN PARALLEL; it touches ONLY internal/cmd/lock.go + lock_test.go — a
#              DIFFERENT file from this item (NO merge conflict). It CONSUMES lock.Status (LANDED). This
#              item ADDS lock.IsOrphaned which that PRP does not need. Read it to avoid ANY overlap.
- docfile: plan/014_37208f58ffa2/P1M3T2S1/PRP.md
  why: "Confirms the sibling's scope fence: 'Touches ONLY internal/cmd/lock.go + lock_test.go. Does NOT
        edit root.go, default_action.go, …, internal/lock/*.' → this item's 4 files do NOT overlap. The
        sibling's runLockStatus uses lock.Status (not IsOrphaned) — so adding IsOrphaned here is safe."
  critical: "Do NOT edit internal/cmd/lock.go or internal/cmd/lock_test.go (the sibling owns them). Do NOT
             modify lock.Status (the sibling CONSUMES its exact signature). This item's lock-package change
             is ADDITIVE ONLY (a new func IsOrphaned; Status untouched)."

# MUST READ — the function under edit (the verbatim Busy Fprintf block to rewrite, lines 326-330).
- file: internal/cmd/default_action.go
  why: "handleLockContention (line 300): blocks (1) no-op fast path 303-309 [PRESERVE], (2) Issue-4b
        fallback substitutions 314-325 [PRESERVE], (3) the Busy fmt.Fprintf 326-330 [REWRITE] +
        `return exitcode.New(exitcode.Busy, nil)` 330 [PRESERVE]. Call site at line 77 (single)."
  pattern: "SILENT returns: the helper prints the message to stderr itself, then returns
            exitcode.New(exitcode.<code>, nil) (nil err → no 'stagecoach: <msg>' double-print by main).
            Match handleGenError/handleDecomposeError."
  gotcha: "imports already present: fmt, io, context, exitcode, lock, git — NO new import needed for the
           reformat. The seam var `orphanChecker = lock.IsOrphaned` uses the already-imported `lock`."

# MUST READ — the lock package: the helper signatures IsOrphaned reuses + where to add the new func.
- file: internal/lock/lock.go
  why: "LockContents (line 47: Pid/Hostname/Repo/Timestamp/Snapshot string). Status (line ~321): the
        alive→orphan order IsOrphaned MIRRORS (`pid, perr := strconv.Atoi(contents.Pid); … alive =
        processAlive(pid, contents.Hostname); if alive { orphan = appearsOrphaned(pid) }`). strconv is
        ALREADY imported (Status uses Atoi) — IsOrphaned needs no new import."
  pattern: "Add IsOrphaned as a NEW exported func near Status (after it). Godoc tone mirrors Status's
            (read-only FR52; conservative). It calls the UNEXPORTED processAlive + appearsOrphaned (same
            package — reachable)."
  gotcha: "Do NOT modify Status (LANDED + tested in lock_unix_test.go:169-261). Do NOT touch orphan_unix.go
           / orphan_windows.go / lock_unix.go / lock_windows.go — IsOrphaned reuses them as-is."

# CONTEXT — the orphan/liveness twins IsOrphaned delegates to (READ-ONLY; understand their conservatism).
- file: internal/lock/orphan_unix.go
  why: "appearsOrphaned(pid): ppid==1 → true; ANY error (proc gone, ps fail, parse fail) → false. This is
        why a DEAD pid yields IsOrphaned==false (ppidOf errors) — correct: a dead holder is reaped, not a
        'uselessly-held' orphan worth a kill hint."
- file: internal/lock/lock_unix.go
  why: "processAlive(pid, hostname): hostname==''||!=thisHost → true (foreign host, conservative); else
        Kill(pid,0): nil/EPERM → true, ESRCH → false. IsOrphaned calls this BEFORE appearsOrphaned (a dead
        holder short-circuits to false). Windows twins: processAlive always-true, appearsOrphaned always-
        false → IsOrphaned(self) is false on Windows (FR-K7)."

# CONTEXT — the seam precedent (clone this idiom for orphanChecker).
- file: internal/cmd/config_init_interactive.go
  why: "Line 20: `var interactiveStdinIsTTY = func() bool { return ui.IsTerminal(os.Stdin) }` — the
        codebase's PROVEN package-level func-var seam (swapped in tests). orphanChecker is the same shape:
        `var orphanChecker = lock.IsOrphaned`. .golangci.yml does NOT enable gochecknoglobals → lint-clean."

# CONTEXT — the tests to update + the fake-git injection idiom already in use.
- file: internal/cmd/lock_contention_test.go
  why: "contentionFakeGit (line 11) embeds git.Git + overrides WriteTree — the injected-dependency idiom.
        TestHandleLockContention_Busy_EmptyDiagnostics (line ~120): the `strings.Contains(msg, "  ")`
        STRICT double-space guard — PROVEN to still pass under the new format (the blank line is \\n\\n,
        not two spaces). The 8 existing handleLockContention calls (lines 45,76,108,147,195,217,228) take
        (stderr, heldErr, g, ctx) — signature UNCHANGED by this item (the seam is a package var, NOT a param)."
  pattern: "Each Busy test: build a *lock.HeldError (Contents + Path) + contentionFakeGit, call
            handleLockContention(&buf, held, g, ctx), assert exitcode.For(err)==exitcode.Busy + err.Error()=='' (silent)
            + buf contents. The seam is swapped via `orig := orphanChecker; t.Cleanup(func(){ orphanChecker = orig });
            orphanChecker = func(lock.LockContents) bool { return true/false }`."

# CONTEXT — the orphan-test placement (add the IsOrphaned unit test HERE, Unix-only, next to Status tests).
- file: internal/lock/lock_unix_test.go
  why: "The Status + appearsOrphaned tests live here (//go:build !windows). Line 224:
        `wantOrphan := appearsOrphaned(os.Getpid())` — the EXACT idiom for the IsOrphaned self-test. Add
        TestIsOrphaned here: empty/malformed pid → false; self (Hostname=this host) → appearsOrphaned(getpid())."

# CONTEXT — the exit-code mapper (verify Busy stays 5, SILENT).
- file: internal/exitcode/exitcode.go
  why: "Busy = 5 (line 27). New(exitcode.Busy, nil) → For() returns 5; nil err → main does NOT print
        'stagecoach: <msg>' (no double-print). The reformat keeps this exact return — ONLY the stderr
        text changes."
```

### Current Codebase tree (relevant slice)

```bash
internal/lock/
  lock.go             # EDIT (ADD func IsOrphaned only; Status UNCHANGED) — P1.M3.T3.S1
  lock_unix.go        # READ-ONLY — processAlive (delegated to by IsOrphaned)
  lock_windows.go     # READ-ONLY — processAlive always-true twin
  orphan_unix.go      # READ-ONLY — appearsOrphaned (delegated to by IsOrphaned)
  orphan_windows.go   # READ-ONLY — appearsOrphaned always-false twin
  lock_unix_test.go   # EDIT (ADD TestIsOrphaned) — P1.M3.T3.S1
internal/cmd/
  default_action.go        # EDIT (ADD seam var orphanChecker + REWRITE Busy Fprintf 326-330) — P1.M3.T3.S1
  lock_contention_test.go  # EDIT (UPDATE assertions + ADD 3 hint tests) — P1.M3.T3.S1
  lock.go                  # READ-ONLY — owned by P1.M3.T2.S1 (parallel); DO NOT TOUCH
  lock_test.go             # READ-ONLY — owned by P1.M3.T2.S1 (parallel); DO NOT TOUCH
  root.go                  # READ-ONLY
internal/exitcode/exitcode.go  # READ-ONLY — Busy=5
Makefile                # test=line 70 (-race); lint=line 103; build=line 52
.golangci.yml           # READ-ONLY — NO gochecknoglobals (seam var is lint-clean)
```

### Desired Codebase tree with files to be added/edited

```bash
internal/lock/lock.go             # EDIT — +func IsOrphaned(contents LockContents) bool (ADDITIVE; Status untouched)
internal/lock/lock_unix_test.go   # EDIT — +TestIsOrphaned (empty/malformed/self branches)
internal/cmd/default_action.go    # EDIT — +var orphanChecker = lock.IsOrphaned; REWRITE Busy Fprintf block (326-330)
internal/cmd/lock_contention_test.go  # EDIT — UPDATE format assertions + 3 NEW seam-driven hint tests
# NOTHING ELSE. No edit to Status, orphan_*.go, lock_unix/windows.go, lock.go(the subcommand), root.go,
# main.go, go.mod, or any PRD/task file. NO new type/flag/dependency.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the blank line is \n\n, NOT two spaces): the strict guard in
// TestHandleLockContention_Busy_EmptyDiagnostics is `strings.Contains(msg, "  ")` (two SPACES). The new
// format's blank line is "\n\n" (two NEWLINES). These do NOT match → the guard PASSES unchanged. Do NOT
// introduce a literal double-space anywhere in the new message (main text, hint). Single-space throughout.

// CRITICAL (IsOrphaned MIRRORS Status's alive→orphan order — do not diverge): Status does
// `alive = processAlive(pid, hostname); if alive { orphan = appearsOrphaned(pid) }`. IsOrphaned MUST do
// the same: Atoi → if !processAlive return false → return appearsOrphaned. A dead holder → processAlive
// false → IsOrphaned false (NO hint — correct; a dead pid holds no flock, reaped by pid-liveness). This
// is WHY a dead holder is not flagged: it's not the "lock stays forever" hazard (that needs an ALIVE
// reparented holder).

// CRITICAL (the hint uses the REAL pid, NOT the <unknown> fallback): the hint fires ONLY when
// heldErr.Contents.Pid != "" (the explicit guard), so `kill %s` uses heldErr.Contents.Pid (a real number)
// and `rm %s` uses heldErr.Path. Never substitute the fallback "<unknown>" into the kill instruction.

// CRITICAL (preserve the SILENT Busy return): handleLockContention prints to stderr ITSELF, then returns
// exitcode.New(exitcode.Busy, nil) — the nil err means main does NOT prepend "stagecoach: <msg>" (no
// double-print). The reformat keeps `return exitcode.New(exitcode.Busy, nil)` byte-identical. ONLY the
// stderr text changes. (Same SILENT pattern as handleGenError/handleDecomposeError.)

// GOTCHA (the seam is a package VAR, not a param — do NOT change handleLockContention's signature): the
// 8 existing test calls + the prod call site (default_action.go:77) take (stderr, heldErr, g, ctx).
// Adding the orphan check as a 5th param would churn all 9 call sites. Instead, a package-level
// `var orphanChecker = lock.IsOrphaned` (the interactiveStdinIsTTY idiom) lets tests swap it with ZERO
// signature change. Tests save/restore via `orig := orphanChecker; t.Cleanup(func(){ orphanChecker = orig })`.

// GOTCHA (lock.IsOrphaned is reachable from package cmd; appearsOrphaned/processAlive are NOT): the cmd
// layer cannot call the unexported helpers directly. IsOrphaned lives in package lock (same package as
// processAlive/appearsOrphaned) so it CAN call them, and it's EXPORTED so cmd can call IsOrphaned. This is
// the entire reason a new export is needed (re-calling lock.Status would re-read the file (race) +
// recompute the path we already hold + need repoPath).

// GOTCHA (Windows build): IsOrphaned calls processAlive + appearsOrphaned, both build-tagged per-OS. All
// four GOOS targets (linux/darwin/windows) MUST build. On Windows processAlive is always-true and
// appearsOrphaned always-false → IsOrphaned returns false for any live pid (FR-K7) — correct, no hint.

// GOTCHA (do NOT modify lock.Status): it is LANDED (P1.M3.T1.S1) and tested (lock_unix_test.go:169-261).
// IsOrphaned is ADDITIVE — a parallel helper with a godoc comment noting it mirrors Status. Editing Status
// risks the sibling P1.M3.T2.S1 PRP (which CONSUMES Status's exact signature) and the LANDED tests.
```

## Implementation Blueprint

### Data models and structure

None NEW. The edit reuses the existing `lock.LockContents` (`Pid, Hostname, Repo, Timestamp, Snapshot
string`) and `*lock.HeldError` (`Contents LockContents; Path string`). No new types, fields, packages,
flags, or imports (strconv already imported in lock.go; fmt/lock already imported in default_action.go).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD func IsOrphaned to internal/lock/lock.go (ADDITIVE; place after Status; do NOT touch Status)
  - GODOC [Mode A]: "IsOrphaned reports whether the holder described by contents APPEARS orphaned — i.e.
    reparented to init/a subreaper (appearsOrphaned) — AND is alive (processAlive). READ-ONLY (FR52
    preserved). It is the SHARED holder-orphan predicate consumed by `stagecoach lock status` (FR-K4, via
    Status) and the Busy-message orphan hint (FR-K5). It mirrors Status's alive→orphan order EXACTLY so
    the diagnostic read path and the Busy message can never disagree. Conservative: empty/malformed pid →
    false (strconv.Atoi fails); dead holder → false (a dead pid holds no flock — reapStaleLocks reaps it
    on the next Acquire — so it is never a 'uselessly-held' orphan worth a kill hint); any appearsOrphaned
    ambiguity → false. On Windows processAlive is always-true and appearsOrphaned always-false → false
    (FR-K7). The orphan==true path (a genuine ppid==1 holder) is proven by the E2E harness (P1.M4.T1.S1)."
  - BODY (verbatim):
      func IsOrphaned(contents LockContents) bool {
          pid, err := strconv.Atoi(contents.Pid)
          if err != nil {
              return false // empty/malformed pid → can't assess
          }
          if !processAlive(pid, contents.Hostname) {
              return false // dead holder is reaped, not "uselessly held" — no kill hint
          }
          return appearsOrphaned(pid) // alive → ppid==1 ⇒ reparented (conservative: false on ambiguity)
      }
  - FOLLOW pattern: Status's internal alive→orphan sequence (lock.go ~line 340-345).
  - NAMING: IsOrphaned (exported; the item contract floats lock.IsOrphaned — LockContents arg subsumes
    the pid-only variant by handling parse + the hostname-aware processAlive).
  - GOTCHA: strconv is ALREADY imported (Status uses it). Do NOT add an import. Do NOT modify Status.
  - PLACEMENT: immediately AFTER Status's closing brace (keeps the read-path exports together).

Task 2: ADD the testability seam + REWRITE the Busy Fprintf block in internal/cmd/default_action.go
  - STEP 2a — ADD the seam var (near the top of the file, after the import block, OR just above
    handleLockContention — co-locate with its sole user for readability):
      // orphanChecker is the holder-orphan predicate used by the Busy-message orphan hint (FR-K5). It
      // defaults to lock.IsOrphaned; tests override it to exercise the hint branch without producing a
      // genuine reparented-to-init holder (which only the E2E harness — P1.M4.T1.S1 — can do reliably).
      // Same package-level func-var seam idiom as interactiveStdinIsTTY (config_init_interactive.go).
      var orphanChecker = lock.IsOrphaned
  - STEP 2b — REWRITE ONLY the Busy fmt.Fprintf block (current lines 326-330). The CURRENT code:
        fmt.Fprintf(stderr,
            "stagecoach: another stagecoach run is already in progress on %s (pid %s on %s). "+
                "Your newly-staged changes will remain staged — re-run stagecoach after it finishes. Lock: %s.\n",
            repo, pid, hostname, heldErr.Path)
        return exitcode.New(exitcode.Busy, nil) // exit 5, SILENT
    becomes:
        fmt.Fprintf(stderr,
            "stagecoach: another stagecoach run is already in progress on %s (pid %s on %s).\n"+
                "Your newly-staged changes will remain staged — re-run stagecoach after it finishes.\n\n"+
                "Lock: %s\n",
            repo, pid, hostname, heldErr.Path)
        if heldErr.Contents.Pid != "" && orphanChecker(heldErr.Contents) {
            fmt.Fprintf(stderr,
                "The holder's launcher appears to have exited — it may be orphaned and holding this "+
                    "lock uselessly. You may safely `kill %s` or `rm %s` to clear it. See `stagecoach lock status`.\n",
                heldErr.Contents.Pid, heldErr.Path)
        }
        return exitcode.New(exitcode.Busy, nil) // exit 5, SILENT
  - PRESERVE UNCHANGED: the no-op fast path (the `if snap := …` block, current 303-309) and the Issue-4b
    fallback substitutions (the repo/pid/hostname `if == ""` block, current 314-325). The `repo`, `pid`,
    `hostname` locals are STILL used in the main message (keep them). ONLY the trailing ` Lock: %s.` was
    excised (moved to its own line). The `heldErr.Contents.Pid` (raw, pre-fallback) is used for BOTH the
    guard and the kill instruction; `heldErr.Path` for the rm instruction.
  - NAMING: orphanChecker (the seam var). No new funcs/types.
  - FOLLOW pattern: the existing SILENT-return pattern (`exitcode.New(exitcode.Busy, nil)`); the
    interactiveStdinIsTTY seam idiom for the var.
  - GOTCHA: print to `stderr` (the param) — NOT stdout. The hint is part of the Busy diagnostic → stderr
    (consistent with the main message). main does NOT re-print (SILENT nil-err return).

Task 3: ADD TestIsOrphaned to internal/lock/lock_unix_test.go (Unix-only, next to the Status/orphan tests)
  - IMPORTS: strconv + os are ALREADY used in this file (line 224 uses os.Getpid; strconv appears in
    ppidOf tests). Verify; add only if missing.
  - BODY:
      func TestIsOrphaned(t *testing.T) {
          // Parse guard — empty / malformed pid → false (cross-platform-conservative).
          if lock.IsOrphaned(lock.LockContents{Pid: ""}) {
              t.Errorf("IsOrphaned(empty Pid) = true, want false")
          }
          if lock.IsOrphaned(lock.LockContents{Pid: "not-a-pid"}) {
              t.Errorf("IsOrphaned(malformed Pid) = true, want false")
          }
          // Self: alive on this host → IsOrphaned mirrors appearsOrphaned(self) (the same predicate
          // Status reports). A dead-pid case is covered by the Status tests; the orphan==true path is E2E.
          host, _ := os.Hostname()
          self := strconv.Itoa(os.Getpid())
          want := appearsOrphaned(os.Getpid()) // same primitive Status uses
          if got := lock.IsOrphaned(lock.LockContents{Pid: self, Hostname: host}); got != want {
              t.Errorf("IsOrphaned(self=%s) = %v, want %v (appearsOrphaned(self))", self, got, want)
          }
      }
  - FOLLOW pattern: TestStatus_* in this file (line ~188-225), esp. line 224 `wantOrphan := appearsOrphaned(os.Getpid())`.
  - GOTCHA: Hostname MUST be set to this host so processAlive takes the Kill(pid,0) path (== thisHost →
    not the foreign-host short-circuit). On Windows this file is excluded (//go:build !windows); the
    Windows twin of IsOrphaned is exercised by the always-false invariant (no separate test needed).
  - PLACEMENT: after the last Status test in lock_unix_test.go.

Task 4: UPDATE + ADD tests in internal/cmd/lock_contention_test.go
  - STEP 4a (UPDATE existing Busy tests for the new format + determinism):
      In TestHandleLockContention_Busy_TreeDiffers (and optionally _EmptySnapshot / _WriteTreeErr), add:
        - a regression guard that the OLD buried form is GONE: the message must NOT contain "finishes. Lock:"
          (the old trailing-in-sentence glue) and must NOT end the lock with a period on the same line.
        - the NEW own-line form IS present: `strings.Contains(msg, "\nLock: /x.lock\n")` (Lock on its own
          line, path verbatim).
        - (determinism) at the top of each non-hint Busy test, pin the seam so the hint's absence does not
          depend on the runner: `orig := orphanChecker; t.Cleanup(func(){ orphanChecker = orig });
          orphanChecker = func(lock.LockContents) bool { return false }`. Then assert the hint is ABSENT:
          `if strings.Contains(msg, "holder's launcher") { t.Errorf(...) }`.
      NOTE: the EmptyDiagnostics test's `strings.Contains(msg, "  ")` double-space guard STILL PASSES —
      do NOT relax it (it's the Issue-4b regression guard). Just verify it still passes.
  - STEP 4b (ADD TestHandleLockContention_Busy_OrphanHint — the hint PRESENT path):
      orig := orphanChecker; t.Cleanup(func(){ orphanChecker = orig })
      orphanChecker = func(lock.LockContents) bool { return true }   // force orphan==true
      held := &lock.HeldError{Contents: lock.LockContents{Pid: "4242", Hostname: "testhost", Repo: "/r", Snapshot: ""}, Path: "/x.lock"}
      g := &contentionFakeGit{}
      var buf bytes.Buffer
      err := handleLockContention(&buf, held, g, context.Background())
      // assert exitcode.For(err)==exitcode.Busy + err.Error()=="" (silent)
      msg := buf.String()
      // Lock on own line:
      if !strings.Contains(msg, "\nLock: /x.lock\n") { t.Errorf("want '\\nLock: /x.lock\\n' own line; got %q", msg) }
      // hint present with REAL pid + path + the status pointer:
      if !strings.Contains(msg, "kill 4242")        { t.Errorf("hint must name the real pid; got %q", msg) }
      if !strings.Contains(msg, "rm /x.lock")       { t.Errorf("hint must name the lock path; got %q", msg) }
      if !strings.Contains(msg, "stagecoach lock status") { t.Errorf("hint must point at lock status; got %q", msg) }
      if !strings.Contains(msg, "holder's launcher appears to have exited") { t.Errorf("hint wording; got %q", msg) }
  - STEP 4c (ADD TestHandleLockContention_Busy_NoOrphanHint — the hint ABSENT path):
      orig := orphanChecker; t.Cleanup(func(){ orphanChecker = orig })
      orphanChecker = func(lock.LockContents) bool { return false }  // holder not orphaned
      held := &lock.HeldError{Contents: lock.LockContents{Pid: "4242", Hostname: "testhost", Repo: "/r", Snapshot: ""}, Path: "/x.lock"}
      ... call, assert Busy + silent ...
      msg := buf.String()
      if !strings.Contains(msg, "\nLock: /x.lock\n") { t.Errorf("Lock own line still present; got %q", msg) }
      if strings.Contains(msg, "holder's launcher")  { t.Errorf("hint must be ABSENT when not orphaned; got %q", msg) }
  - STEP 4d (ADD TestHandleLockContention_Busy_NoOrphanHintWhenPidEmpty — the Pid!="" guard is independent):
      orig := orphanChecker; t.Cleanup(func(){ orphanChecker = orig })
      orphanChecker = func(lock.LockContents) bool { return true }   // predicate CLAIMS orphan...
      held := &lock.HeldError{Contents: lock.LockContents{Pid: "", Hostname: "testhost", Repo: "/r", Snapshot: ""}, Path: "/x.lock"}
      ... call, assert Busy + silent ...
      msg := buf.String()
      if strings.Contains(msg, "holder's launcher") { t.Errorf("hint must be ABSENT when pid is empty (guard independent of predicate); got %q", msg) }
      if !strings.Contains(msg, "\nLock: /x.lock\n") { t.Errorf("Lock own line still present; got %q", msg) }
      // the fallback "<unknown>" still renders in the main message (Issue-4b preserved):
      if !strings.Contains(msg, "<unknown>") { t.Errorf("pid fallback '<unknown>' must still render; got %q", msg) }
  - FOLLOW pattern: the existing contentionFakeGit + handleLockContention(&buf, held, g, ctx) idiom.
  - NAMING: TestHandleLockContention_Busy_<Scenario> (matches existing _Busy_* names).
  - GOTCHA: t.Cleanup restores the seam (NOT defer — t.Cleanup is the codebase's idiom and survives
    t.Run subtests). The seam is package-scoped → ALWAYS restore (a leaked swap corrupts other tests).

Task 5: VERIFY — build (native+cross-compile), vet, format, full regression, lint, grep guards
  - go build ./... ; GOOS=linux/darwin/windows go build ./...
  - go vet ./internal/cmd/... ./internal/lock/... ; gofmt -l <the 4 files>   # empty
  - go test ./internal/cmd/ -run 'TestHandleLockContention' -race -v
  - go test ./internal/lock/ -run 'TestIsOrphaned' -race -v
  - go test ./internal/cmd/ ./internal/lock/ -race ; make test ; make lint ; make build
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN (the seam — clone of interactiveStdinIsTTY at config_init_interactive.go:20):
var orphanChecker = lock.IsOrphaned // tests swap this; default is the real predicate

// PATTERN (the rewritten Busy block — SILENT return preserved; ONLY stderr text changes):
fmt.Fprintf(stderr,
	"stagecoach: another stagecoach run is already in progress on %s (pid %s on %s).\n"+
		"Your newly-staged changes will remain staged — re-run stagecoach after it finishes.\n\n"+
		"Lock: %s\n",
	repo, pid, hostname, heldErr.Path)
if heldErr.Contents.Pid != "" && orphanChecker(heldErr.Contents) {
	fmt.Fprintf(stderr,
		"The holder's launcher appears to have exited — it may be orphaned and holding this "+
			"lock uselessly. You may safely `kill %s` or `rm %s` to clear it. See `stagecoach lock status`.\n",
		heldErr.Contents.Pid, heldErr.Path)
}
return exitcode.New(exitcode.Busy, nil) // exit 5, SILENT (unchanged)

// PATTERN (the new export — mirrors Status's alive→orphan order; ADDITIVE only):
func IsOrphaned(contents LockContents) bool {
	pid, err := strconv.Atoi(contents.Pid)
	if err != nil {
		return false
	}
	if !processAlive(pid, contents.Hostname) {
		return false // dead holder is reaped, not "uselessly held"
	}
	return appearsOrphaned(pid) // alive → ppid==1 ⇒ reparented
}

// PATTERN (swapping the seam in tests — restore via t.Cleanup):
orig := orphanChecker
t.Cleanup(func() { orphanChecker = orig })
orphanChecker = func(lock.LockContents) bool { return true } // or false
```

### Integration Points

```yaml
CLI SURFACE (user-facing stderr):
  - The Busy message (exit 5) now has `Lock: <path>` on its OWN line (copy-pasteable) and, conditionally,
    a one-line orphan hint naming the real pid + path + `stagecoach lock status`. Exit code UNCHANGED (5).
  - No new subcommand, no new flag, no new exit code.

LOCK PACKAGE (internal/lock/lock.go):
  - ADD exported `func IsOrphaned(contents LockContents) bool` (consumed by internal/cmd via the seam).
  - Status is UNCHANGED (the sibling P1.M3.T2.S1 CONSUMES its exact signature; LANDED tests pin it).

CMD PACKAGE (internal/cmd/default_action.go):
  - ADD `var orphanChecker = lock.IsOrphaned` (seam). REWRITE the Busy Fprintf block (326-330) only.
  - handleLockContention's SIGNATURE is UNCHANGED (the 8 test calls + 1 prod call site need NO edit).
  - No new import (fmt + lock already imported).

NO database / migration / routes / new types / new flags / config change / root.go edit / go.mod change.
SCOPE FENCES (no overlap with siblings):
  - Touches ONLY: internal/lock/lock.go (+IsOrphaned), internal/lock/lock_unix_test.go (+TestIsOrphaned),
    internal/cmd/default_action.go (+seam, rewrite Fprintf), internal/cmd/lock_contention_test.go (update+add).
  - Does NOT touch: internal/cmd/lock.go / lock_test.go (P1.M3.T2.S1 — parallel, different file),
    internal/lock/orphan_*.go / lock_unix.go / lock_windows.go, internal/lock/lock.go's Status, root.go,
    main.go, go.mod, any PRD/task file.
  - The README CLI-reference / message-format sync is P1.M4.T2.S1 (NOT here — [Mode A] godoc only).
  - The real-orphan E2E (genuine ppid==1 holder) is P1.M4.T1.S1 (NOT here — unit tests use the seam).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Native + cross-compile (IsOrphaned calls build-tagged processAlive/appearsOrphaned — all 4 OS targets).
go build ./...
GOOS=linux   go build ./...
GOOS=darwin  go build ./...
GOOS=windows go build ./...
# Expected: all clean. GOOS=windows must build (processAlive/appearsOrphaned always-false/true twins).

# Vet (the two changed packages).
go vet ./internal/cmd/... ./internal/lock/...
# Expected: clean.

# Format (the 4 touched files).
gofmt -l internal/lock/lock.go internal/lock/lock_unix_test.go internal/cmd/default_action.go internal/cmd/lock_contention_test.go
# Expected: empty. If listed: gofmt -w <those files>.

# Lint.
make lint   # errcheck/gosimple/govet/ineffassign/staticcheck/unused (NO gochecknoglobals → seam var is clean)
# Expected: zero errors. orphanChecker is USED (handleLockContention) + ASSIGNABLE (tests); IsOrphaned is
#           USED (the seam default). No unused-symbol findings.

# Scope guard: ONLY the 4 files changed.
git status --porcelain
# Expected: internal/lock/lock.go, internal/lock/lock_unix_test.go, internal/cmd/default_action.go,
#           internal/cmd/lock_contention_test.go. ZERO changes elsewhere (esp. NOT internal/cmd/lock.go,
#           lock_test.go, Status, orphan_*.go, root.go).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new + updated contention tests.
go test ./internal/cmd/ -run 'TestHandleLockContention' -race -v
# Expected: ALL PASS —
#   _NoOpFastPath (unchanged, exit 0); _Busy_TreeDiffers / _EmptySnapshot / _WriteTreeErr / _SilentExits
#     (updated: Lock own-line asserted, old buried form absent, hint absent with seam pinned false);
#   _Busy_EmptyDiagnostics (Issue-4b fallbacks + the "  " double-space guard STILL PASS — \n≠space);
#   NEW _Busy_OrphanHint (seam true → hint present: kill 4242 / rm /x.lock / stagecoach lock status);
#   NEW _Busy_NoOrphanHint (seam false → hint absent, Lock own-line present);
#   NEW _Busy_NoOrphanHintWhenPidEmpty (seam true BUT pid empty → hint absent; <unknown> fallback present).

# The new IsOrphaned unit test (+ Status regression).
go test ./internal/lock/ -run 'TestIsOrphaned|TestStatus' -race -v
# Expected: TestIsOrphaned PASS (empty/malformed→false; self→appearsOrphaned(getpid())); Status tests green.

# Full cmd + lock package regression (the seam is package-scoped — ensure no leak across tests).
go test ./internal/cmd/ ./internal/lock/ -race
# Expected: green. (The hint tests restore orphanChecker via t.Cleanup → no cross-test contamination.)
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary (proves the reformat links into the real run path).
make build

# Manual: reproduce a Busy contention with a held lock, observe the new format.
#   Terminal A: acquire a long-held lock by running a generation that blocks (e.g. a stub provider that sleeps),
#   OR plant contention by running two stagecoach invocations.
#   Terminal B (while A holds the lock):
bin/stagecoach          # → the Busy message; verify:
#   - `Lock: <path>` is on its OWN line (copy-pasteable), NOT buried mid-sentence;
#   - exit code is 5: echo "exit=$?";
#   - (if the holder is your own live shell, no orphan hint — correct; the hint needs a reparented holder,
#      which is the E2E harness's job — P1.M4.T1.S1.)

# Manual: the no-op fast path is UNCHANGED — a contender whose index matches the holder's snapshot exits 0
# with "nothing to do…" (verify the text is byte-identical to before).

# Expected: Busy → exit 5, Lock on own line; no-op fast path → exit 0, "nothing to do…".
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: IsOrphaned is EXPORTED + mirrors the alive→orphan order (Atoi → processAlive → appearsOrphaned).
grep -n 'func IsOrphaned(contents LockContents) bool' internal/lock/lock.go
grep -n 'processAlive(pid, contents.Hostname)' internal/lock/lock.go   # 2 hits: Status + IsOrphaned
grep -n 'appearsOrphaned(pid)' internal/lock/lock.go                   # 2 hits: Status + IsOrphaned
# Expect: IsOrphaned present; processAlive+appearsOrphaned each appear in BOTH Status and IsOrphaned.

# Guard 2: Status is UNCHANGED (still the LANDED 5-value signature; not edited).
grep -n 'func Status(repoPath string) (path string, contents LockContents, alive bool, orphan bool, err error)' internal/lock/lock.go
git diff internal/lock/lock.go | grep -E '^-.*func Status|^\+.*func Status' && echo "FAIL: Status edited" || echo "OK: Status untouched"

# Guard 3: the seam var exists + defaults to lock.IsOrphaned.
grep -n 'var orphanChecker = lock.IsOrphaned' internal/cmd/default_action.go
# Expect: 1 hit.

# Guard 4: the OLD buried form is GONE; the new own-line form is present.
grep -n 'Lock: %s\\.\n' internal/cmd/default_action.go   # the OLD trailing-in-sentence form
# Expect: ZERO hits (removed).
grep -n '"Lock: %s\\n"' internal/cmd/default_action.go    # the NEW own-line form
# Expect: 1 hit.

# Guard 5: the orphan hint is CONDITIONAL on non-empty pid + the seam predicate.
grep -n 'heldErr.Contents.Pid != "" && orphanChecker(heldErr.Contents)' internal/cmd/default_action.go
# Expect: 1 hit.

# Guard 6: the hint uses the REAL pid + path (not fallbacks) + names lock status.
grep -n 'kill %s` or `rm %s' internal/cmd/default_action.go
grep -n 'heldErr.Contents.Pid, heldErr.Path' internal/cmd/default_action.go
grep -n 'stagecoach lock status' internal/cmd/default_action.go
# Expect: the kill/rm Fprintf + the (pid, path) args + the status pointer each present.

# Guard 7: the SILENT Busy return is byte-identical.
grep -n 'return exitcode.New(exitcode.Busy, nil)' internal/cmd/default_action.go
# Expect: 1 hit (unchanged).

# Guard 8: no double-space introduced anywhere in the new message (the Issue-4b invariant).
# (The EmptyDiagnostics test enforces this at runtime; this is a static sanity grep.)
internal/cmd/default_action.go grep -nE '"  ' internal/cmd/default_action.go || echo "OK: no literal double-space in source"

# Guard 9: handleLockContention's signature is UNCHANGED (seam is a var, not a param).
grep -n 'func handleLockContention(stderr io.Writer, heldErr \*lock.HeldError, g git.Git, ctx context.Context) error' internal/cmd/default_action.go
# Expect: 1 hit (identical to today — the 8 test calls + 1 prod call site need NO edit).

# Guard 10: scope — only the 4 files.
git status --porcelain
# Expect: internal/lock/lock.go, internal/lock/lock_unix_test.go, internal/cmd/default_action.go,
#         internal/cmd/lock_contention_test.go ONLY.
git diff --name-only | grep -E 'internal/cmd/lock\.go$|internal/cmd/lock_test\.go$|orphan_|lock_unix\.go|lock_windows\.go|root\.go' && echo "FAIL: out-of-scope file edited" || echo "OK: scope clean"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + `GOOS=linux` + `GOOS=darwin` + `GOOS=windows` all clean
- [ ] `go vet ./internal/cmd/... ./internal/lock/...` clean
- [ ] `gofmt -l` empty on the 4 touched files
- [ ] `make lint` zero errors (orphanChecker + IsOrphaned both used; no gochecknoglobals)
- [ ] `go test ./internal/cmd/ ./internal/lock/ -race` green (updated + new tests; seam restored via t.Cleanup)
- [ ] `make test` (full race suite) green; `make build` clean

### Feature Validation
- [ ] Busy message prints `Lock: <path>` on its OWN line (not buried mid-sentence) (grep guard 4)
- [ ] When holder pid non-empty + orphaned: the one-line hint follows, naming the REAL pid (`kill <N>`),
      the path (`rm <path>`), and `stagecoach lock status` (grep guards 5,6)
- [ ] When pid empty: hint ABSENT even if the predicate claims orphan (TestHandleLockContention_Busy_NoOrphanHintWhenPidEmpty)
- [ ] When not orphaned: hint ABSENT, Lock own-line present (TestHandleLockContention_Busy_NoOrphanHint)
- [ ] No-op fast path ("nothing to do…" exit 0) byte-unchanged
- [ ] Issue-4b fallbacks (`an unknown repo`, `<unknown>`) still render; no `on  (` / double-space
      (TestHandleLockContention_Busy_EmptyDiagnostics double-space guard STILL PASSES)
- [ ] Exit code stays Busy (5), SILENT (grep guards 7,9); main does not double-print

### Scope-Boundary Validation
- [ ] `git status` shows ONLY the 4 files (grep guard 10)
- [ ] NO edit to internal/cmd/lock.go / lock_test.go (P1.M3.T2.S1), Status, orphan_*.go, lock_unix.go /
      lock_windows.go, root.go, main.go, go.mod, or any PRD/task file
- [ ] NO new exported TYPE, NO new flag, NO new third-party dependency, NO new import (strconv + fmt + lock
      already present), NO signature change to handleLockContention
- [ ] NO README/docs sync (P1.M4.T2.S1); NO real-orphan E2E (P1.M4.T1.S1)

### Code Quality & Docs
- [ ] [Mode A] Godoc on `IsOrphaned`: read-only (FR52), conservative (false on ambiguity), mirrors Status,
      consumed by the Busy hint (FR-K5) + `lock status` (FR-K4); the orphan==true path is E2E-proven
- [ ] Comment on `orphanChecker` explains the seam (defaults to lock.IsOrphaned; tests override; mirrors
      interactiveStdinIsTTY; the real-orphan path is P1.M4.T1.S1)
- [ ] Follows the SILENT-return pattern (exitcode.New(code, nil)); the reformat changes ONLY stderr text
- [ ] The new format introduces no double-space (Issue-4b invariant preserved)

---

## Anti-Patterns to Avoid

- ❌ Don't modify `lock.Status`. It is LANDED (P1.M3.T1.S1) + tested (lock_unix_test.go:169-261), and the
  parallel sibling P1.M3.T2.S1 CONSUMES its exact 5-value signature. `IsOrphaned` is ADDITIVE — a parallel
  helper. If you want DRY, note in IsOrphaned's godoc that it mirrors Status; do NOT refactor Status to call
  IsOrphaned (that risks the LANDED tests + the sibling's contract).
- ❌ Don't re-call `lock.Status(repoDir)` inside handleLockContention to get the orphan flag. It re-reads the
  lock file (a race window — the file may change between Acquire's contention and now), recomputes the path
  you already hold in heldErr.Path, and needs repoPath (not the helper's natural input). Use the NEW
  `lock.IsOrphaned(heldErr.Contents)` — it operates on the data you already have.
- ❌ Don't change `handleLockContention`'s signature to inject the orphan check as a 5th param. That churns
  the 8 test calls + the 1 prod call site (default_action.go:77). Use the package-level `orphanChecker` seam
  var (the `interactiveStdinIsTTY` idiom) — ZERO signature change, and it's the codebase's proven pattern.
- ❌ Don't emit the orphan hint for an EMPTY pid. The `heldErr.Contents.Pid != ""` guard is INDEPENDENT of
  the predicate (a test pins `orphanChecker→true` with `Pid==""` and asserts the hint is ABSENT). The hint's
  `kill %s` MUST use the real pid — never the `<unknown>` fallback.
- ❌ Don't relax the `strings.Contains(msg, "  ")` double-space guard in EmptyDiagnostics. It is the Issue-4b
  regression guard and STILL PASSES under the new format (the blank line is `\n\n`, two newlines, NOT two
  spaces). Verify; do not delete.
- ❌ Don't introduce a literal double-space anywhere in the new message (main text or hint). Single-space
  throughout — the EmptyDiagnostics guard will catch any regression.
- ❌ Don't print the hint (or the Lock line) to stdout. The Busy message is a stderr diagnostic (the helper's
  `stderr` param). Keep the SILENT `return exitcode.New(exitcode.Busy, nil)` so main does not double-print.
- ❌ Don't forget to restore the seam in tests. `orphanChecker` is package-scoped; a leaked `true`/`false`
  swap corrupts every later cmd test. Use `orig := orphanChecker; t.Cleanup(func(){ orphanChecker = orig })`
  (t.Cleanup, not defer — it survives t.Run subtests and is the codebase's idiom).
- ❌ Don't touch `internal/cmd/lock.go` / `lock_test.go`. They are owned by the parallel P1.M3.T2.S1
  (`stagecoach lock status` subcommand). This item's cmd-package edit is `default_action.go` +
  `lock_contention_test.go` ONLY — different files, NO merge conflict, but DO NOT edit them.
- ❌ Don't try to unit-test the orphan hint with a REAL orphaned process (ppid==1). It is flaky/OS-dependent
  (subreapers, CI runners). Drive the hint via the `orphanChecker` seam (force true/false); the genuine
  orphan==true path is the E2E harness's job (P1.M4.T1.S1).
- ❌ Don't sync the README / CLI-reference message format here. That is P1.M4.T2.S1. This item adds [Mode A]
  godoc on `IsOrphaned` + a comment on `orphanChecker` ONLY.
