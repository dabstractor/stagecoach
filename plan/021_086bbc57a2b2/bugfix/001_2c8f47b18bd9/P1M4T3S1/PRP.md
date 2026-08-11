name: "P1.M4.T3.S1 — BUG-011: document the appearsOrphaned subreaper limitation (Option B, comment-only) + FR-K2 cross-reference to arm_unix.go"
description: >
  A COMMENT-ONLY rewrite of the `appearsOrphaned` doc comment (+ the inline `return ppid == 1` comment) in
  `internal/lock/orphan_unix.go` (//go:build !windows). BUG-011: the `lock status` orphan HINT returns
  `ppid == 1`, which under-reports orphans under subreapers (PR_SET_CHILD_SUBREAPER — systemd, systemd-run,
  Docker/containerd/runc, podman, supervisord, some shells) where a reparented orphan's ppid is the subreaper
  pid, not 1. DECISION: Option B (document) over Option A (comm-based subreaper detection) — A adds
  false-positive risk (a process legitimately parented to systemd would be wrongly flagged, violating the
  conservative "never false-positive" invariant); B records the limitation + the rationale. The strengthened
  comment adds: (1) SPECIFIC subreaper examples; (2) a CROSS-REFERENCE to the authoritative detector
  internal/watchdog/arm_unix.go's `osGetppid() != originalPpid` poll (FR-K2, subreaper-safe — the watchdog
  catches orphaning under subreapers and routes the holder through rescue + lock release, so NO live lock is
  ever left orphaned-forever by this hint's false negative); (3) the FR-K2 citation VERBATIM (FR-K2 itself
  calls the getppid()==1 test "brittle" and "wrong under subreapers" — appearsOrphaned is the one place that
  still uses it, and the comment explains WHY: it is a SNAPSHOT diagnostic with no startup baseline to diff,
  so it cannot use the CHANGE detection the watchdog uses); (4) explicit DISPLAY-ONLY framing (FR-K4 lock
  status + FR-K5 busy hint; changes nothing; never force-breaks); (5) Option A recorded as a deferred
  enhancement. NO behavior change (the `return ppid == 1` body stays), NO new tests (existing behavioral
  tests stay green), NO user-facing surface change (Mode A — code comment only; no status-output / README /
  docs edit). Mirrors the parallel P1.M4.T2.S1 (BUG-010 Windows processAlive, also comment-only). Validated
  by go build/vet/gofmt + `go test ./internal/lock/ -race` (green) + make lint + grep guards on the
  cross-references (arm_unix.go / FR-K2 / subreaper examples).

---

## Goal

**Feature Goal**: Comprehensively document (Option B) the known subreaper limitation of `appearsOrphaned` so
that a future maintainer understands WHY this display hint knowingly uses the `ppid == 1` test that FR-K2
explicitly rejects for the watchdog — and knows the authoritative detector lives in `arm_unix.go`. The bug
is cosmetic (no live lock is ever force-broken); the fix is documentation precision, not a heuristic change.

**Deliverable**: EDIT `internal/lock/orphan_unix.go` — rewrite the `appearsOrphaned` doc comment (the
HEURISTIC + CONSERVATIVE INVARIANT + LIMITATION paragraphs + the inline comment on the `return ppid == 1`
line) to add: specific subreaper examples, the `arm_unix.go` parent-pid-CHANGE cross-reference, the verbatim
FR-K2 citation, the "snapshot, cannot diff" rationale, the display-only/no-force-break framing, and the
Option-A-deferred note. The function BODY (`return ppid == 1`) is UNCHANGED. Comment-only.

**Success Definition**:
- `go build ./...` clean; `go vet ./internal/lock/...` clean; `gofmt -l internal/lock/orphan_unix.go` empty.
- `go test ./internal/lock/ -race` green (the behavioral tests — TestAppearsOrphaned_DeadPidIsConservativeFalse,
  TestAppearsOrphaned_SelfIsNotOrphan, TestStatus_*, TestIsOrphaned — are unaffected; they assert return
  values, not comment text).
- `make lint` clean.
- grep guards pass: the comment names FR-K2, arm_unix.go, `osGetppid() != originalPpid`, ≥4 subreaper names
  (systemd, systemd-run, Docker/containerd, supervisord/podman), "display-only" (or equivalent), and records
  Option A as deferred.
- `git status --porcelain` == `internal/lock/orphan_unix.go` ONLY (one file; comment-only; no body/behavior
  change; no test/doc/spec/lock_windows/arm_unix/cmd/lock edits).

## User Persona (if applicable)

**Target User**: The future Stagecoach maintainer reading `orphan_unix.go` who wonders why `appearsOrphaned`
uses the `ppid == 1` test that FR-K2 warns against — and whether to "fix" it.
**Use Case**: Maintainer diagnoses a "lock status says not-orphaned but the holder IS orphaned under
systemd/Docker" report; the comment explains it is a known display-only limitation, points at the
authoritative watchdog, and records why the heuristic wasn't extended (false-positive risk).
**Pain Points Addressed**: Prevents a well-meaning maintainer from adding comm-based subreaper detection
(Option A) that would introduce false-positive kill hints; documents the real safety property (the watchdog,
not this hint, guarantees no orphaned-forever lock).

## Why

- **BUG-011 (cosmetic/informational)**: `appearsOrphaned` under-reports orphans under subreapers. The
  architecture doc (§BUG-011) and prd_snapshot Issue 8 both conclude this is display-only — the authoritative
  self-termination watchdog (`arm_unix.go`) is correct (FR-K2). The fix is documentation, per the contract's
  explicit recommendation: "Recommended: Option B (document)".
- **The existing comment already has a LIMITATION paragraph but is INCOMPLETE**: it omits the `arm_unix.go`
  cross-reference, the FR-K2 citation, the "snapshot cannot diff" rationale, and the Option-A-deferred note.
  A maintainer could misread the current comment as "just lazily using ppid==1" rather than "deliberately
  using the only zero-false-positive snapshot test, with the watchdog as the real backstop".
- **Consistency with the sibling fix**: the parallel P1.M4.T2.S1 (BUG-010 Windows processAlive) is the same
  shape — a comment-only strengthening of a known, accepted, documented limitation, citing the requirement by
  name + cross-referencing the authoritative path. This item is its Unix-orphan analogue.

## What

Rewrite the `appearsOrphaned` doc comment in `internal/lock/orphan_unix.go` (and the inline comment on its
`return ppid == 1` line). Keep the function body byte-for-byte identical. The comment becomes the
comprehensive record of the subreaper limitation, the authoritative detector, and the Option-A deferral.

### Success Criteria
- [ ] `appearsOrphaned`'s doc comment is rewritten to cover ALL of: (1) what it is + who calls it (Status/
      IsOrphaned → FR-K4 lock status + FR-K5 busy hint; READ-ONLY); (2) the ppid==1 heuristic; (3) the
      CONSERVATIVE INVARIANT (return false on any error/ambiguity; false-positive worse than false-negative);
      (4) the STRENGTHENED LIMITATION with ≥4 named subreapers (systemd, systemd-run, Docker/containerd/runc,
      podman, supervisord — pick a representative set) + PR_SET_CHILD_SUBREAPER; (5) the FR-K2 citation
      (FR-K2 itself rejects the getppid()==1 test as "brittle"/"wrong under subreapers"); (6) the
      `arm_unix.go` cross-reference (`osGetppid() != originalPpid`, subreaper-safe — the AUTHORITATIVE
      detector that routes the holder through rescue + lock release, so no live lock is orphaned-forever);
      (7) the "snapshot diagnostic, no startup baseline ⇒ cannot use CHANGE detection" rationale; (8) the
      DISPLAY-ONLY / never-force-breaks framing; (9) Option A (comm-based subreaper detection) recorded as
      DEFERRED with the false-positive rationale.
- [ ] The inline comment on `return ppid == 1` is strengthened to name subreapers + point at the comment above
      + reference arm_unix.go (FR-K2) as authoritative.
- [ ] The function BODY is UNCHANGED: still `ppid, err := ppidOf(pid); if err != nil { return false }; return ppid == 1`.
- [ ] `gofmt -l` empty on the file (comment wrapping/alignment is gofmt-clean).
- [ ] `go test ./internal/lock/ -race` green; `make lint` clean; `git status` shows ONLY orphan_unix.go.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim current comment (the exact text to replace), the exact FR-K2 wording (verbatim), the
exact `arm_unix.go` cross-reference line, the consumer list (FR-K4/K5), the existing tests (confirm
behavior-only assertions), the Option-B-over-A rationale, and the grep guards. A maintainer with zero prior
context can write the comment and validate it.

### Documentation & References

```yaml
# MUST READ — the file I EDIT (the comment to rewrite + the body I keep unchanged).
- file: internal/lock/orphan_unix.go
  why: "appearsOrphaned (line ~30 doc comment; line ~42 `return ppid == 1`). The current doc already has a
        HEURISTIC + CONSERVATIVE INVARIANT + LIMITATION structure + a cross-platform note. I STRENGTHEN the
        LIMITATION + add the arm_unix.go cross-reference + the FR-K2 citation + the snapshot rationale + the
        Option-A-deferred note. ppidOf/ppidLinux/ppidViaPs are READ-ONLY (do NOT edit)."
  critical: "The function BODY stays `return ppid == 1` (Option A is DEFERRED, documented in the comment —
             NOT implemented). gofmt the file after editing (comment line-wrapping). //go:build !windows at
             the top — keep it."

# MUST READ — the AUTHORITATIVE detector I cross-reference (read-only; do NOT edit).
- file: internal/watchdog/arm_unix.go
  why: "armImpl (line ~30 doc; line ~51 `if osGetppid() != originalPpid { signal.Trigger(SIGTERM); n.fire() }`).
        Its doc already states 'NOT getppid()==1, which is wrong under subreapers per FR-K2'. This is the
        cross-reference target: appearsOrphaned is the DISPLAY hint; armImpl is the AUTHORITATIVE self-
        termination that routes rescue + lock release. Name the line + the osGetppid()!=originalPpid test."

# MUST READ — the verbatim FR-K2 wording (cite it in the comment).
- docfile: plan/014_37208f58ffa2/prd_snapshot.md
  section: "§9.27 FR-K2 (line 565) — 'Detection signal — parent-pid change, not getppid() == 1'"
  why: "FR-K2 is the requirement that DEFINES subreaper-safe detection and explicitly rejects the
        getppid()==1 test as 'brittle' and 'wrong under subreapers — systemd-run, some shells — and for
        processes legitimately spawned by init'. appearsOrphaned is the one place still using it; the comment
        must cite FR-K2 verbatim + explain why this hint knowingly keeps the limited test."
  critical: "FR-K1 (parent-death watchdog), FR-K4 (lock status — the consumer), FR-K5 (busy hint), FR-K7
             (Unix-only; Windows twin) are the surrounding FRs to name where relevant."

# MUST READ — the consumers (so the comment names the right FRs + call sites).
- file: internal/lock/lock.go
  why: "Status (line ~371) calls appearsOrphaned at line ~390 (only when alive); IsOrphaned (line ~407) is
        the SHARED holder-orphan predicate. Both feed FR-K4 (lock status) + FR-K5 (busy hint). The comment
        should say 'consumed by Status/IsOrphaned → FR-K4 lock status + FR-K5 busy hint; READ-ONLY'."

# MUST READ — the bug definition (the contract source).
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/architecture/bugfix_subsystems.md
  section: "§BUG-011 (line ~176) + the Recommendations (Option A vs B)"
  why: "Confirms Option B (document) is recommended; Option A (comm-based subreaper detection) adds false-
        positive risk and is 'correct per the design' to leave as the conservative false-negative. Cosmetic/
        informational ONLY — the watchdog (arm_unix.go) is correct."

# MUST READ — the existing tests (confirm a comment-only change is safe).
- file: internal/lock/lock_unix_test.go
  why: "TestAppearsOrphaned_DeadPidIsConservativeFalse + TestAppearsOrphaned_SelfIsNotOrphan + TestStatus_* +
        TestIsOrphaned assert on RETURN VALUES (behavior), never on comment text. A comment-only rewrite
        leaves all green. (SelfIsNotOrphan Skipf's under ppid==1 CI — unaffected.)"

# REFERENCE — the Windows twin (FR-K7); named in the cross-platform note, NOT edited.
- file: internal/lock/orphan_windows.go
  why: "//go:build windows; `func appearsOrphaned(pid int) bool { return false }`. The always-false twin
        (FR-K7: Windows has no init-reparenting analog; flock is a no-op; CAS is the guarantee). My comment's
        cross-platform note already references it — keep/confirm that reference."
```

### Current Codebase tree (relevant slice)

```bash
internal/lock/
  orphan_unix.go       # EDIT (comment-only) — appearsOrphaned doc comment + inline `return ppid == 1` comment
  orphan_windows.go    # READ-ONLY (FR-K7 twin) — referenced by the cross-platform note; NOT edited
  lock.go              # READ-ONLY — Status (line ~390) + IsOrphaned (line ~407) consume appearsOrphaned
  lock_unix_test.go    # READ-ONLY — behavioral tests (unaffected by a comment change)
internal/watchdog/
  arm_unix.go          # READ-ONLY (the cross-REFERENCE target) — osGetppid() != originalPpid (FR-K2)
internal/cmd/
  lock.go              # READ-ONLY — `lock status` (FR-K4) prints orphaned: true/false/unknown; NOT edited
plan/014_37208f58ffa2/prd_snapshot.md   # READ-ONLY — §9.27 FR-K1–K7 definitions (cite FR-K2 verbatim)
```

### Desired Codebase tree with files to be added/modified

```bash
internal/lock/orphan_unix.go   # EDIT (comment-only): strengthened appearsOrphaned doc + inline comment
# NOTHING ELSE. Comment-only; no new files; no body/behavior/test/doc/spec change.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (comment-ONLY — do NOT change behavior): the function body MUST stay
//   ppid, err := ppidOf(pid); if err != nil { return false }; return ppid == 1
// Option A (comm-based subreaper detection via /proc/<ppid>/comm) is DEFERRED — recorded in the comment,
// NOT implemented. Implementing A would add false-positive risk (a process legitimately parented to systemd
// — a systemd unit, a user-session child — would be wrongly flagged orphaned) and violate the conservative
// "never false-positive" invariant. The contract + architecture doc both recommend Option B (document).

// CRITICAL (FR-K2 is the keystone citation): FR-K2 VERBATIM rejects the getppid()==1 test: "never by the
// brittle 'getppid() == 1' test (wrong under subreapers — systemd-run, some shells — and for processes
// legitimately spawned by init)". appearsOrphaned is the ONE place that still uses it. The comment must
// explain WHY it knowingly does: it is a SNAPSHOT diagnostic (sees the holder's CURRENT ppid, with NO startup
// baseline to diff), so it CANNOT use the parent-pid-CHANGE detection the watchdog (arm_unix.go) uses;
// ppid==1 is the only ZERO-false-positive snapshot test, and the watchdog is the authoritative backstop.

// CRITICAL (the authoritative detector is arm_unix.go, NOT this hint): cross-reference
// internal/watchdog/arm_unix.go's `osGetppid() != originalPpid` poll (FR-K2, subreaper-safe). That watchdog
// captures originalPpid at startup, polls for a CHANGE, and routes the holder through rescue + OnRescueExit
// lock release on parent death — so NO live lock is ever left orphaned-forever by this hint's false negative.
// Make this explicit: appearsOrphaned's false negative is display-only; the safety property is the watchdog's.

// GOTCHA (gofmt wraps comments): gofmt reformats comment block line-wrapping only within tables/struct
// literals, NOT free-floating // comments — BUT the repo's lint (golangci-lint) + the existing file's style
// wrap doc comments at ~78-80 cols. Match the existing line width (orphan_unix.go's current comments wrap
// ~78). Run `gofmt -w` then eyeball; `gofmt -l` must be empty. Misaligned/wrapped comments fail `make lint`
// (gofumpt/golines may be active — check .golangci.yml).

// GOTCHA (//go:build !windows constraint tag): the file's FIRST line is `//go:build !windows`. Do NOT remove
// or alter it (it gates the whole file out of Windows builds; orphan_windows.go is the twin). A comment edit
// below it is fine; the tag stays at line 1 with a blank line before `package lock`.

// GOTCHA (AGENTS.md rule #2 — never edit spec/ autonomously): the cross-reference to FR-K2 goes IN THE CODE
// COMMENT, not into spec/SPEC.md or PRD.md. FR-K2 in the spec is CORRECT (it warns against getppid()==1);
// the bug is in the code heuristic's documentation, not the spec. Do NOT touch any spec/PRD/task file.

// GOTCHA (no user-facing surface change — Mode A): do NOT edit internal/cmd/lock.go's status output text, do
// NOT add a runtime "orphan detection may miss subreaper-reparented processes" note to `lock status`, do NOT
// touch README.md / docs/. The contract DOCS line is explicit: "Update the code comment … No user-facing doc
// surface change." The deliverable is the CODE COMMENT only.
```

## Implementation Blueprint

### Data models and structure

None. Comment-only; no types, no fields, no constants, no imports change.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/lock/orphan_unix.go — rewrite the appearsOrphaned doc comment (comment-ONLY)
  - SCOPE: replace ONLY the doc-comment block above `func appearsOrphaned(pid int) bool` (the HEURISTIC +
    CONSERVATIVE INVARIANT + LIMITATION + cross-platform paragraphs) AND the inline comment on the
    `return ppid == 1` line. The function SIGNATURE + BODY + ppidOf/ppidLinux/ppidViaPs are UNCHANGED.
  - THE REWRITTEN DOC COMMENT must contain (use the exact identifiers so grep guards pass):
      (a) PURPOSE + consumer: appearsOrphaned is a HEURISTIC reporting whether holder pid APPEARS orphaned
          (reparented to init/launchd — §9.27's orphaned-but-alive case, FR-K4's diagnostic). Called by
          Status/IsOrphaned ONLY when the holder is alive (a dead pid is reaped regardless). READ-ONLY — it
          feeds the user's kill/rm decision (FR-K4 lock status + FR-K5 busy hint); it changes nothing.
      (b) HEURISTIC: ppid == 1 ⇒ reparented to init.
      (c) CONSERVATIVE INVARIANT: returns false on ANY error/ambiguity (proc gone, ps failure, parse failure);
          a false-positive orphan claim (prompting the user to kill a legitimately-parented run) is worse than
          a false negative. The only `true` is ppid == 1.
      (d) STRENGTHENED LIMITATION: under a subreaper (PR_SET_CHILD_SUBREAPER — systemd, systemd-run,
          Docker/containerd/runc, podman, supervisord, some shells) an orphan's ppid is the SUBREAPER's pid,
          not 1, so this can MISS orphans (false negative); it never false-positives in the common case.
      (e) FR-K2 CITATION (verbatim phrasing): FR-K2 explicitly rejects this very test for the watchdog —
          "never by the brittle 'getppid() == 1' test (wrong under subreapers)". Quote it.
      (f) RATIONALE (why appearsOrphaned still uses it): it is a SNAPSHOT diagnostic — it sees the holder's
          CURRENT ppid via ppidOf with NO startup baseline to diff, so it CANNOT use the parent-pid-CHANGE
          detection the watchdog uses. ppid==1 is the only ZERO-false-positive snapshot test.
      (g) CROSS-REFERENCE (the authoritative detector): the AUTHORITATIVE orphan self-termination is
          internal/watchdog/arm_unix.go's poll `osGetppid() != originalPpid` (FR-K2, subreaper-safe): it
          captures the parent pid at startup, detects the CHANGE on reparenting (to init OR a subreaper), and
          routes the holder through the §18.3 rescue + §18.5 OnRescueExit lock-release exit path. Therefore
          this hint's false negative NEVER leaves a live lock orphaned-forever — the watchdog catches it.
          appearsOrphaned is DISPLAY-ONLY; arm_unix.go is the safety property.
      (h) OPTION A DEFERRED: a comm-based enhancement (read /proc/<ppid>/comm; treat ppid as a subreaper if
          comm ∈ systemd/containerd/dockerd/supervisord/init/launchd) is DELIBERATELY DEFERRED — it adds
          false-positive risk (a process legitimately parented to systemd — a systemd unit, a user-session
          child — would be wrongly flagged), violating the conservative invariant. The conservative false
          negative is the accepted design (architecture §BUG-011).
      (i) CROSS-PLATFORM: orphan_windows.go is the always-false twin (FR-K7 — Windows has no init-reparenting
          analog; flock is a no-op; the §13.5 CAS is the guarantee). Platform dispatch is runtime.GOOS in a
          single file (every import referenced by ≥1 compiled function).
  - THE INLINE COMMENT on `return ppid == 1`:
      `return ppid == 1 // reparented to init/launchd; subreapers (systemd/docker/supervisord) reparent to
       their own pid≠1 → false-negative (documented above). The AUTHORITATIVE detector is arm_unix.go's
       osGetppid()!=originalPpid (FR-K2); this hint is display-only.`
      (Keep it to ONE line if it fits ~100 cols, else a tight 2-line // comment. gofmt-clean.)
  - NAMING/PLACEMENT: edit in place; no new symbols; no import changes. Package `lock`, //go:build !windows.

Task 2: VERIFY — build, vet, format, test, lint, grep guards, scope guard
  - go build ./... ; go vet ./internal/lock/... ; gofmt -l internal/lock/orphan_unix.go   # empty
  - go test ./internal/lock/ -race        # behavioral tests green (comment change can't break them)
  - make test && make lint                # whole-repo green; golangci-lint clean (comment style/wrapping)
  - grep guards + git status (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN (the strengthened LIMITATION paragraph — the heart of the change). Adapt wording; KEEP the
// identifiers the grep guards look for (FR-K2, arm_unix.go, osGetppid() != originalPpid, the subreaper names,
// "display-only", Option A "deferred"):
//
// LIMITATION (subreapers — BUG-011, accepted): under a subreaper (PR_SET_CHILD_SUBREAPER — systemd,
// systemd-run, Docker/containerd/runc, podman, supervisord, some shells) a reparented orphan's ppid is the
// SUBREAPER's pid, not 1, so this hint can MISS orphans (false negative); it never false-positives a
// legitimately-parented process. This is the SAME test FR-K2 explicitly REJECTS for the watchdog — "never by
// the brittle 'getppid() == 1' test (wrong under subreapers)" — and appearsOrphaned is the one place that
// still uses it. It does so DELIBERATELY: it is a SNAPSHOT diagnostic that sees only the holder's CURRENT
// ppid (via ppidOf), with NO startup baseline to diff, so it CANNOT use the parent-pid-CHANGE detection the
// watchdog uses (arm_unix.go). ppid==1 is the only ZERO-false-positive snapshot answer, and the watchdog is
// the authoritative backstop: internal/watchdog/arm_unix.go's poll `osGetppid() != originalPpid` (FR-K2,
// subreaper-safe) captures the parent pid at startup, fires on the CHANGE (reparenting to init OR a
// subreaper), and routes the holder through the §18.3 rescue + §18.5 lock-release exit path — so this hint's
// false negative NEVER leaves a live lock orphaned-forever. appearsOrphaned is DISPLAY-ONLY (FR-K4 lock
// status + FR-K5 busy hint); arm_unix.go is the safety property. A comm-based enhancement (read
// /proc/<ppid>/comm; flag ppid as a subreaper for systemd/containerd/dockerd/supervisord/init/launchd) is
// DELIBERATELY DEFERRED — it would false-positive a process legitimately parented to systemd (a unit, a
// user-session child), violating the conservative invariant. The false negative is the accepted design.
//
// PATTERN (the inline return comment):
//	return ppid == 1 // reparented to init/launchd; subreapers (systemd/docker/supervisord) reparent to their pid≠1 → false-negative (see LIMITATION above; arm_unix.go osGetppid()!=originalPpid is authoritative, FR-K2)

// PATTERN (comment-only discipline — mirror BUG-010/P1.M4.T2.S1):
//   - cite the requirement (FR-K2) BY NAME in the comment (AGENTS.md rule #2 → cross-refs live in code, not spec).
//   - cross-reference the authoritative path (arm_unix.go) + the caller (Status/IsOrphaned).
//   - record the deferred option (A) WITH its rationale (false-positive risk).
//   - the BODY is unchanged → no test added → existing behavioral tests are the regression gate.
```

### Integration Points

```yaml
CODE (comment-ONLY edit):
  - internal/lock/orphan_unix.go: appearsOrphaned doc comment (rewrite) + inline `return ppid == 1` comment.
NO database / migration / routes / config / exit-code / go.mod / docs / spec change.
CONSUMES (READ-ONLY — referenced, not edited):
  - internal/watchdog/arm_unix.go (the authoritative osGetppid()!=originalPpid detector — FR-K2).
  - internal/lock/lock.go (Status/IsOrphaned — the callers; FR-K4/K5).
  - plan/014_37208f58ffa2/prd_snapshot.md §9.27 FR-K2 (the verbatim citation source).
SCOPE FENCES:
  - Touches ONLY: internal/lock/orphan_unix.go (comment-only).
  - Does NOT edit: the appearsOrphaned BODY (return ppid == 1 stays), ppidOf/ppidLinux/ppidViaPs,
    orphan_windows.go (FR-K7 twin), lock.go, arm_unix.go (cross-ref target), internal/cmd/lock.go (status
    output — no user-facing change), any test file, README.md / docs/ (Mode A), PRD.md / spec/* / tasks.json /
    prd_snapshot (AGENTS.md rules #2 + forbidden-operations), go.mod.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build (a comment change can't break the build, but confirm the //go:build tag + package decl are intact).
go build ./...
# Expected: clean.

# Vet the changed package.
go vet ./internal/lock/...
# Expected: clean.

# Format (comment line-wrapping must be gofmt-clean; match the file's ~78-col style).
gofmt -l internal/lock/orphan_unix.go
# Expected: empty. If listed: gofmt -w internal/lock/orphan_unix.go.

# Lint (golangci-lint — comment style/wrapping, godot if configured). Check .golangci.yml for godumpt/golines.
make lint
# Expected: zero errors. If godot fires on a comment lacking terminal period, end each doc-comment sentence
#           with a period (the existing comments do).

# Scope guard: ONLY orphan_unix.go; comment-only.
git status --porcelain
git diff --stat
# Expected: internal/lock/orphan_unix.go ONLY; insertions/deletions are comment lines only.
git diff internal/lock/orphan_unix.go | grep -E '^\+.*return ppid == 1|^-.*return ppid == 1' | head
# Expected: the `return ppid == 1` line appears ONLY in + context if the inline comment changed, and the BODY
#           logic is identical (no `ppidOf`/`if err` line removed). Eyeball: the diff is all `//` comment lines.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The behavioral tests assert return values, not comment text — a comment change leaves them green.
go test ./internal/lock/ -race -run 'TestAppearsOrphaned|TestStatus|TestIsOrphaned' -v
# Expected: TestAppearsOrphaned_DeadPidIsConservativeFalse PASS; TestAppearsOrphaned_SelfIsNotOrphan PASS
#           (or Skipf under ppid==1 CI — pre-existing, not a regression); TestStatus_* / TestIsOrphaned PASS.

# Full lock-package regression (comment change can't affect reaping/acquire/CAS).
go test ./internal/lock/ -race
# Expected: green.

# Whole-repo test + lint + build.
make test && make lint && make build
# Expected: all green.
```

### Level 3: Integration Testing (System Validation)

```bash
# (No integration behavior to test — this is a comment-only change with zero runtime effect.) Optional manual
#  proof the display path still renders (the hint output is unchanged; run on a repo with no lock):
cd "$(mktemp -d)" && git init -q && git -C . commit -q --allow-empty -m init
<path-to-stagecoach-binary> lock status; echo "exit=$?"
# Expected: "no run lock for <repo>" exit 0 (or the alive/orphan block if a lock is held). Output UNCHANGED
#           from before this edit — there is NO user-facing surface change (Mode A).
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: ONLY one file changed; comment-only (no body/behavior change).
git status --porcelain
test "$(git status --porcelain | wc -l)" -eq 1 && echo "OK: one file" || echo "FAIL: expected one file"
git diff --name-only | grep -vE '^internal/lock/orphan_unix\.go$' && echo "FAIL: out-of-scope file" || echo "OK: scope clean"

# Guard 2: the BODY is unchanged — `return ppid == 1` still present; ppidOf still called.
grep -n 'return ppid == 1' internal/lock/orphan_unix.go        # 1 hit (the return)
grep -n 'ppid, err := ppidOf(pid)' internal/lock/orphan_unix.go # 1 hit (unchanged body)

# Guard 3: the FR-K2 citation is present (the keystone cross-reference).
grep -n 'FR-K2' internal/lock/orphan_unix.go                    # ≥1 (the citation)

# Guard 4: the arm_unix.go cross-reference + the authoritative CHANGE test are named.
grep -n 'arm_unix.go' internal/lock/orphan_unix.go              # ≥1
grep -n 'osGetppid() != originalPpid' internal/lock/orphan_unix.go   # ≥1

# Guard 5: ≥4 named subreapers (the specific examples the contract asks for).
for s in systemd systemd-run containerd dockerd supervisord podman; do
  grep -qi "$s" internal/lock/orphan_unix.go && echo "OK: $s named" || echo "MISSING: $s"
done
# (Aim for ≥4 OK; pick the set the comment uses. PR_SET_CHILD_SUBREAPER must also appear:)
grep -n 'PR_SET_CHILD_SUBREAPER' internal/lock/orphan_unix.go   # ≥1

# Guard 6: the display-only / no-force-break framing is present.
grep -niE 'display.only|read.only|never.*force.break|changes nothing' internal/lock/orphan_unix.go  # ≥1

# Guard 7: Option A is recorded as deferred (not implemented).
grep -niE 'deferred|deliberately not|option a' internal/lock/orphan_unix.go   # ≥1

# Guard 8: the consumers (FR-K4/K5) are named.
grep -n 'FR-K4' internal/lock/orphan_unix.go                    # ≥1
grep -n 'FR-K5' internal/lock/orphan_unix.go                    # ≥1 (or named in the consumer line)

# Guard 9: NO user-facing surface change — status output untouched.
git diff --name-only | grep -q 'internal/cmd/lock.go' && echo "FAIL: edited status output (Mode A forbids)" || echo "OK: no user-facing change"
git diff --name-only | grep -qE 'README\.md|docs/' && echo "FAIL: edited docs (Mode A forbids)" || echo "OK: no docs change"

# Guard 10: no spec/task/snapshot edit (AGENTS.md rules #2 + forbidden-operations).
git diff --name-only | grep -qE 'PRD\.md|spec/|tasks\.json|prd_snapshot' && echo "FAIL: edited forbidden file" || echo "OK: no spec/task edit"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./internal/lock/...` clean; `gofmt -l` empty on orphan_unix.go
- [ ] `make lint` zero errors (comment style/wrapping; godot terminal periods if configured)
- [ ] `go test ./internal/lock/ -race` green (behavioral tests unaffected)
- [ ] `make test` + `make build` green

### Feature Validation (the comment covers the contract)
- [ ] Specific subreaper examples present (≥4: systemd, systemd-run, Docker/containerd, supervisord/podman) + PR_SET_CHILD_SUBREAPER (grep guards 5)
- [ ] FR-K2 cited verbatim (the "brittle getppid()==1" rejection) (grep guard 3)
- [ ] arm_unix.go cross-reference + `osGetppid() != originalPpid` named as the AUTHORITATIVE detector (grep guard 4)
- [ ] "snapshot, cannot diff" rationale present (why this hint can't use CHANGE detection)
- [ ] display-only / never-force-breaks framing present (grep guard 6)
- [ ] Option A recorded as deferred with the false-positive rationale (grep guard 7)
- [ ] consumers named (FR-K4 lock status + FR-K5 busy hint) (grep guard 8)

### Scope-Boundary Validation
- [ ] `git status` shows ONLY `internal/lock/orphan_unix.go` (grep guard 1)
- [ ] the function BODY is unchanged — `return ppid == 1` + `ppidOf(pid)` intact (grep guard 2)
- [ ] NO edit to orphan_windows.go, lock.go, arm_unix.go, internal/cmd/lock.go (status output), any test,
      README.md / docs/ (Mode A), PRD.md / spec/* / tasks.json / prd_snapshot / go.mod (grep guards 9, 10)

### Code Quality & Docs
- [ ] Comment line-wrapping matches the file's ~78-col style; gofmt-clean
- [ ] The //go:build !windows tag + blank line + `package lock` are intact at the top
- [ ] Every doc-comment sentence ends with a period (godot, if configured in .golangci.yml)
- [ ] The comment is self-contained: a maintainer reading ONLY it understands the limitation, the authoritative
      detector, and why Option A was deferred

---

## Anti-Patterns to Avoid

- ❌ Don't implement Option A (comm-based subreaper detection). The contract recommends Option B (document). A
  reads `/proc/<ppid>/comm` and flags ppid as a subreaper — but a process legitimately parented to systemd (a
  unit, a user-session child) would be WRONGLY flagged orphaned, a false-positive kill hint that violates the
  conservative "never false-positive" invariant. Record A as DEFERRED in the comment; do not code it.
- ❌ Don't change the function BODY. The deliverable is the COMMENT. `return ppid == 1` stays; ppidOf/
  ppidLinux/ppidViaPs are untouched. Any body change is out of scope and would need behavioral tests this
  item does not add.
- ❌ Don't add a user-facing note to `lock status` (e.g. "orphan detection may miss subreaper-reparented
  processes"). The contract DOCS line is explicit: "No user-facing doc surface change." Mode A = code comment
  only. Don't edit internal/cmd/lock.go, README.md, or docs/.
- ❌ Don't edit the spec/PRD. AGENTS.md rule #2 forbids autonomous spec edits; FR-K2 in the spec is CORRECT
  (it warns against getppid()==1). The bug is in the code heuristic's DOCUMENTATION, not the spec. The
  cross-reference to FR-K2 goes IN THE CODE COMMENT. Don't touch PRD.md, spec/*, tasks.json, prd_snapshot.
- ❌ Don't weaken the existing CONSERVATIVE INVARIANT framing. The "return false on any error/ambiguity;
  false-positive worse than false-negative" principle is the design's load-bearing safety property and must
  stay prominent — it's the reason Option A is deferred.
- ❌ Don't drop the arm_unix.go cross-reference. It is the WHOLE point of the strengthening: the comment must
  make clear that the authoritative orphan detector (and thus the real no-orphaned-forever-lock guarantee)
  lives in arm_unix.go, and that this hint's false negative is display-only. A comment that says "limitation:
  subreapers" WITHOUT pointing at the authoritative detector is the incomplete status quo this item fixes.
- ❌ Don't edit orphan_windows.go (the FR-K7 twin) or arm_unix.go (the cross-reference TARGET). Both are
  read-only consumers/references for this item. One file changes: orphan_unix.go.
- ❌ Don't skip gofmt. Comment line-wrapping must be gofmt-clean and match the file's style; `make lint`
  (golangci-lint, possibly godot/golines) will flag ragged comments or missing terminal periods.