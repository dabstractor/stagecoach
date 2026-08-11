# Findings — P1.M4.T2.S1 (Document accepted Windows reaping behavior, BUG-010)

BUG-010 (PRD §h3.9 / architecture §BUG-010): `internal/lock/lock_windows.go:processAlive` returns
`true` unconditionally → `reapStaleLocks` never reaps on Windows. The contract recommends **Option A**
(document the accepted behavior + cite FR-K7) over Option B (a real Windows liveness probe). This is a
**code-comment-only** task (Mode A doc fix; no user-facing doc surface). 1 file, ~1 comment rewrite.

## 0. ⚠️ THE CONTRACT'S "lock files aren't created on Windows" CLAIM IS FALSE — do NOT propagate it

THREE sources (the task contract, the bugfix architecture §BUG-010, AND `prd_snapshot.md:125`) all state:
"Lock files are not created on the Windows commit path, so reaping is moot." **This is factually wrong.**
The code proves the file IS created on Windows:

- `internal/lock/lock.go:85` — `os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)` has **NO build gate** →
  it runs on EVERY platform (incl. Windows). The file is created.
- `internal/cmd/default_action.go:73` — `locker, lockErr := lock.Acquire(repoDir)` runs on EVERY platform
  (no `//go:build` gate) → the Windows commit path DOES reach Acquire → DOES create the file.
- `internal/lock/lock_windows.go:flock` — the no-op is ONLY `flock(fd)` (returns nil). OpenFile is NOT
  stubbed on Windows.

So the ACCURATE state on Windows is:
1. The lock FILE is created (OpenFile O_CREATE, cross-platform).
2. The FLOCK is a no-op (lock_windows.go) → no real mutual exclusion; the §13.5 CAS is the guarantee.
3. The deferred `Release()` (`lock.go`) removes the file on every NORMAL exit (the common case).
4. Only an ABNORMAL exit (taskkill /F, a crash, a console-close that bypasses the deferred cleanup) leaves
   the file as litter. That litter is **inert + benign** (no flock hazard; the CAS guarantees a killed run
   can never land a wrong commit). It is bounded (normal exits dominate) but not zero.
5. `reapStaleLocks` calls `processAlive` → `true` on Windows → never reaps → abnormal-exit litter accumulates.

**The strengthened comment MUST state this accurately** (file IS created but inert + CAS-guaranteed +
deferred-Release-cleaned), NOT "files aren't created." Propagating the false claim would mislead future
maintainers and contradict the code. This is the single most important finding for this PRP.

## 1. The current comment (lock_windows.go) — already substantive, but missing FR-K7 + inaccurate-ish
```go
// processAlive is a conservative no-op on Windows: it always reports the pid as alive (never reap).
// flock is a no-op on Windows (no inode-bound-flock hazard — see flock above), so there is no
// dead-file reaping to do; the §13.5 CAS (update-ref HEAD compare-and-swap) is the safety guarantee.
// The "never reap a live pid" invariant is trivially satisfied (reap nothing). Cross-platform twin
// of lock_unix.go's processAlive; used by reapStaleLocks (P1.M2.T1.S2).
func processAlive(pid int, hostname string) bool { return true }
```
It cites §13.5 CAS + §18.5 but NOT **FR-K7** (the requirement that pins this as accepted behavior), and it
implies "no dead-file reaping to do" (true-ish, but the accurate framing is "reaping is non-functional,
not unnecessary" — files exist). The fix: explicitly cite FR-K7 + the accurate file-creation/inert/CAS
reasoning + document Option B (real probe) as a deferred low-value option.

## 2. FR-K7 wording (the requirement to cite) — verified
- `prd_snapshot.md` (the bugfix plan's PRD) §9.27 FR-K7: "the watchdog is Unix-only — on Windows `flock`
  is already a no-op and the §13.5 CAS is the guarantee (FR-K7)."
- `spec/06-reliability.md` §18.5 "Orphaned-but-alive" paragraph already covers "the watchdog is Unix-only
  and has an opt-out ... (FR-K6–K7)" and §18.5's reaping prose (Unix `kill(pid,0)`/ESRCH). FR-K7 is the
  requirement; the code comment documents how the implementation honors it on Windows.

## 3. The spec cross-reference — DO NOT autonomously edit spec/* (AGENTS.md hard rule #2)
The contract point 3(b) says "Add a cross-reference to FR-K7 in spec/SPEC.md." TWO blockers:
1. **`spec/SPEC.md` is now a 67-line HEADER** that `@`-imports 7 split files (`01-product.md`…
   `07-reference.md`). FR-K7's content lives in **`spec/06-reliability.md`** (§9.27 + §18.5), not the
   67-line header. "Edit spec/SPEC.md" is not even the right target.
2. **AGENTS.md hard rule #2**: "Never modify `spec/SPEC.md` outside of an interactive session ... If you
   find a spec gap or contradiction while working, surface it (open an issue / raise it in chat) and stop —
   do not edit `spec/SPEC.md` to 'fix' it on your own." This governs autonomous agent work.

RESOLUTION (rule-respecting): the FR-K7 cross-reference goes **IN THE CODE COMMENT** (cite "FR-K7" by name
in lock_windows.go — that IS the cross-reference: the implementation pointing at the requirement). The spec
itself is ALREADY adequate: FR-K7 (§9.27) states the watchdog is Unix-only + flock is a no-op + CAS is the
guarantee; §18.5 covers reaping generally. If the implementer believes the spec should EXPLICITLY call out
the Windows `processAlive` no-op, they **SURFACE it to the human** (chat/issue) per AGENTS.md rule #2 — they
do NOT auto-edit any `spec/*.md` file. The PRP must NOT instruct an autonomous spec edit.

## 4. The cross-platform twin (lock_unix.go) — already accurate, OUT OF SCOPE
`lock_unix.go:processAlive` (syscall.Kill(pid,0); ESRCH→dead, EPERM→alive-different-user, foreign-host→true)
already says: "Cross-platform: lock_windows.go provides an always-true twin (flock is a no-op there → no
reaping; the §13.5 CAS is the guarantee)." It is accurate; leave it alone (tight scope). Optionally the
implementer MAY add a reciprocal "FR-K7" cite there, but it is NOT required.

## 5. Parallel-safety + scope
- **P1.M4.T1.S1 (BUG-009, parallel/Implementing)** touches `lock.go` (reapStaleLocks + staleLockUnchanged)
  + `lock_test.go` + `lock_unix_test.go`. MY task touches ONLY `lock_windows.go`. **ZERO file overlap.** ✓
- **P1.M4.T3.S1 (BUG-011, Planned)** touches `orphan_unix.go`. Different file. ✓
- **P1.M4.T4.S1 (Mode B docs, Planned)** touches README/docs. Different surface. ✓
- TOUCH: `internal/lock/lock_windows.go` ONLY (comment rewrite). NOT lock.go/lock_unix.go/orphan_*.go/
  lock_*_test.go/any spec/*.md/PRD.md/tasks.json/docs.

## 6. Validation specifics for a Windows-build-tagged file
`lock_windows.go` has `//go:build windows` → it is NOT compiled on the Linux dev host by `go build ./...`.
To actually compile/vet the edited file: `GOOS=windows go build ./internal/lock/` +
`GOOS=windows go vet ./internal/lock/`. `gofmt -l` works cross-platform (parses regardless of build tag).
A comment-only change cannot change behavior → no test changes; `make test`/`make lint` stay green (they
just don't exercise the windows file on a non-windows host — the GOOS=windows build is the real gate).
There is NO `lock_windows_test.go` (only `lock_unix_test.go`) → nothing to update.

## 7. The accurate strengthened comment (the deliverable, verbatim substance)
```go
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
func processAlive(pid int, hostname string) bool { return true }
```
(This corrects the false "files aren't created" claim, cites FR-K7, and documents Option B as deferred.)