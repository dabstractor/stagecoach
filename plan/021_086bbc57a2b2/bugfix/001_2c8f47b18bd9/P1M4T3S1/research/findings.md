# Findings — P1.M4.T3.S1 (BUG-011: document the appearsOrphaned subreaper limitation)

OPTION B (document) — a COMMENT-ONLY rewrite of the `appearsOrphaned` doc comment in
`internal/lock/orphan_unix.go`. No behavior change, no new tests, no user-facing surface change.
Mirrors the parallel P1.M4.T2.S1 (BUG-010 Windows processAlive, also comment-only).

## 0. The bug (from architecture/bugfix_subsystems.md §BUG-011 + prd_snapshot Issue 8)

`internal/lock/orphan_unix.go:42` — `appearsOrphaned(pid) bool` returns `ppid == 1`. Under a subreaper
(`PR_SET_CHILD_SUBREAPER` — systemd, systemd-run, Docker/containerd/runc, podman, supervisord, some shells),
a reparented orphan's ppid is the SUBREAPER's pid, NOT 1. So the hint reports "not orphaned" for a genuinely
orphaned holder → **false negative** (under-reporting). It NEVER false-positives (ppid==1 only happens on
real reparenting to init/launchd). **Cosmetic/informational ONLY.**

## 1. WHY it is cosmetic (the authoritative detector is elsewhere — the cross-reference)

The ACTUAL orphan self-termination is `internal/watchdog/arm_unix.go:40`:
```go
if osGetppid() != originalPpid {   // parent-pid CHANGE — NOT getppid()==1
    signal.Trigger(syscall.SIGTERM)
    n.fire()
}
```
This is **FR-K2**-compliant detection (parent-pid CHANGE, subreaper-safe). Its arm_unix.go doc already
states: "On a parent-pid CHANGE (osGetppid() != originalPpid — NOT getppid()==1, which is wrong under
subreapers per FR-K2)". So the WATCHDOG correctly catches orphaning under subreapers and routes the holder
through rescue + lock-release. `appearsOrphaned`'s false negative therefore NEVER leaves a live lock
orphaned-forever — it only makes the `lock status` DISPLAY hint under-report.

## 2. The exact FR-K2 wording (cite it verbatim in the comment)

From plan/014_37208f58ffa2/prd_snapshot.md:565 (the v2.7 orphan-watchdog feature that DEFINED FR-K1–K7):
> **FR-K2. Detection signal — parent-pid change, not `getppid() == 1`.** Orphaning is detected by the parent
> pid **changing** from the value captured at startup (reparenting to init or a subreaper), never by the
> brittle "`getppid() == 1`" test (wrong under subreapers — `systemd-run`, some shells — and for processes
> legitimately spawned by init).

KEY: FR-K2 ITSELF calls the `getppid()==1` test "brittle" and "wrong under subreapers". `appearsOrphaned` is
the ONE place in the codebase that still uses that test — and the comment must say WHY it knowingly does so
(the rationale for Option B over A).

## 3. WHY appearsOrphaned cannot just use the watchdog's CHANGE detection (the rationale for B)

`appearsOrphaned` is a **SNAPSHOT diagnostic**: it sees the holder's CURRENT ppid (via ppidOf → /proc or ps)
with NO startup baseline to diff. The watchdog (arm_unix.go) captures `originalPpid` AT STARTUP and polls for
a CHANGE — appearsOrphaned has neither the baseline nor a poll loop; it is invoked once by `Status`/`IsOrphaned`
to render a display line. So it CANNOT use the FR-K2 CHANGE detection; it can only ask "is this CURRENT ppid
likely a reparenting target?" — and ppid==1 (init/launchd) is the only ZERO-false-positive answer to that.

## 4. WHY Option A (comm-based subreaper detection) is deferred (the false-positive risk)

Option A: read `/proc/<ppid>/comm` and treat ppid as a subreaper if comm ∈ {systemd, containerd, dockerd,
supervisord, init, launchd, …}. RISK: a process legitimately parented to systemd (a systemd unit/service, a
user-session child under `systemd --user`, a container whose entrypoint IS the pid-1 child) would be WRONGLY
flagged orphaned → a false-positive kill hint → violates the conservative invariant ("a false-positive orphan
claim prompting the user to kill a legitimately-parented run is worse than a false negative"). The
architecture doc agrees: "the current conservative approach (never false-positive) is correct per the design."
So Option A is DEFERRED — documented as a known future enhancement in the comment, not implemented.

## 5. The consumers (so the comment names the right FRs)

`appearsOrphaned` is called by `Status` (internal/lock/lock.go:390) and `IsOrphaned` (lock.go:~407), which
feed:
- **FR-K4** `stagecoach lock status` — `internal/cmd/lock.go:66,85-90` prints `orphaned: true/false/unknown`.
  It is READ-ONLY ("It changes nothing").
- **FR-K5** the Busy-message orphan hint.

## 6. The existing comment to STRENGTHEN (orphan_unix.go, current)

The file ALREADY has a LIMITATION paragraph (the prior pass added it). The contract wants it STRENGTHENED:
(a) add specific subreaper examples, (b) cross-reference arm_unix.go's CHANGE detection, (c) cite FR-K2,
(d) make the display-only/no-live-lock-force-broken framing explicit, (e) record Option A as deferred.

Current LIMITATION block (what's there now):
```
// LIMITATION: under a subreaper (PR_SET_CHILD_SUBREAPER — systemd, systemd-run,
// some shells, containers) an orphan's ppid may be != 1 ... it never
// false-positives ... The only `true` is ppid == 1.
```
Current inline (line 42): `return ppid == 1 // reparented to init/launchd (subreapers may have ppid≠1 — limitation)`

MISSING from current: the arm_unix.go cross-reference; the FR-K2 citation (FR-K2 explicitly rejects this very
test for the watchdog); the "snapshot, can't diff" rationale; the Option-A-deferred note.

## 7. Existing tests (confirm comment-only change is safe)

`internal/lock/lock_unix_test.go`: `TestAppearsOrphaned_DeadPidIsConservativeFalse` (dead pid ⇒ false),
`TestAppearsOrphaned_SelfIsNotOrphan` (self ppid≠1 ⇒ false; Skipf under init). Plus `TestStatus_*` and
`TestIsOrphaned`. ALL assert on BEHAVIOR (return values), NOT comment text. A comment-only rewrite leaves
every one green. `go test ./internal/lock/ -race` is the regression gate.

## 8. Scope fence (mirror BUG-010's comment-only discipline)

TOUCH: `internal/lock/orphan_unix.go` (the doc comment on `appearsOrphaned` + the inline comment on the
`return ppid == 1` line). DO NOT TOUCH: the function BODY (`return ppid == 1` stays — Option A deferred),
ppidOf/ppidLinux/ppidViaPs (read-only), orphan_windows.go (FR-K7 twin), lock.go (Status/IsOrphaned
consumers), arm_unix.go (the cross-REFERENCE target, not edited), internal/cmd/lock.go (status output —
"No user-facing doc surface change"), any test file, README.md / docs/ (Mode A = code comment only),
PRD.md / spec/* (AGENTS.md rule #2 forbids autonomous spec edits; FR-K2 in the spec is CORRECT — the bug is
in the code heuristic, not the spec), go.mod. ZERO behavior change. Validated by build/vet/gofmt/test/lint +
grep guards on the cross-references.