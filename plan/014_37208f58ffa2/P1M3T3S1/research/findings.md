# P1.M3.T3.S1 — Research Findings (Busy-message reformat + orphan hint)

Source: direct codebase reads + PRD/architecture review. No external research needed (pure stdlib
`fmt`/`strconv` + the existing `internal/lock` API; no new library).

## §0 — The function under edit: handleLockContention (internal/cmd/default_action.go:300-330)

```go
func handleLockContention(stderr io.Writer, heldErr *lock.HeldError, g git.Git, ctx context.Context) error {
	// (1) No-op fast path (lines 303-309): snapshot set + contender WriteTree == snapshot → exit 0.
	if snap := heldErr.Contents.Snapshot; snap != "" {
		contenderTree, werr := g.WriteTree(ctx)
		if werr == nil && contenderTree == snap { ... return exitcode.New(exitcode.Success, nil) }
	}
	// (2) Fallback diagnostics (lines 314-325): repo→"an unknown repo", pid/hostname→"<unknown>".
	repo := heldErr.Contents.Repo; if repo == "" { repo = "an unknown repo" }
	pid  := heldErr.Contents.Pid;  if pid  == "" { pid  = "<unknown>" }
	hostname := heldErr.Contents.Hostname; if hostname == "" { hostname = "<unknown>" }
	// (3) THE Busy Fprintf (lines 326-329) — THIS is what changes:
	fmt.Fprintf(stderr,
		"stagecoach: another stagecoach run is already in progress on %s (pid %s on %s). "+
			"Your newly-staged changes will remain staged — re-run stagecoach after it finishes. Lock: %s.\n",
		repo, pid, hostname, heldErr.Path)
	return exitcode.New(exitcode.Busy, nil) // exit 5, SILENT
}
```

CONTRACT (must preserve unchanged): (1) no-op fast path; (2) fallback substitutions; the SILENT
Busy return (`exitcode.New(exitcode.Busy, nil)` — main does NOT double-print). Only block (3) changes.

## §1 — Call site (single)

`internal/cmd/default_action.go:77`:
```go
locker, lockErr := lock.Acquire(repoDir)   // repoDir = os.Getwd() at line 52
if errors.As(lockErr, &held) {
	return handleLockContention(stderr, held, g, ctx)
}
```
`repoDir` and `held` are in scope, but we do NOT need repoDir — the orphan check operates on
`heldErr.Contents` (the pid), NOT a repoPath re-read. No signature change needed for the core edit.

## §2 — The orphan detection helpers (internal/lock/)

- `lock.Status(repoPath) (path, contents, alive, orphan, err)` — EXPORTED, LANDED (P1.M3.T1.S1),
  tested in `lock_unix_test.go:169-261`. Internally: `Atoi(contents.Pid)` → `processAlive(pid, hostname)`
  → `if alive { orphan = appearsOrphaned(pid) }`.
- `appearsOrphaned(pid int) bool` — UNEXPORTED (`orphan_unix.go` / `orphan_windows.go`). Conservative:
  `ppid == 1` → true; ANY error/ambiguity → false. Windows twin always-false.
- `processAlive(pid int, hostname string) bool` — UNEXPORTED (`lock_unix.go` / `lock_windows.go`).
  `hostname=="" || hostname!=thisHost` → true (foreign host, conservative). Else `Kill(pid,0)`: nil/EPERM
  → true; ESRCH → false. Windows always-true.

PROBLEM: from `package cmd` neither `appearsOrphaned` nor `processAlive` is reachable. The cmd layer
already HAS `heldErr.Contents` (Pid/Hostname) — re-calling `lock.Status(repoDir)` would re-read the
file (race window) AND recompute the path we already hold. Cleanest: a NEW export that takes
`LockContents` (the data we already have) and mirrors Status's alive→orphan logic.

## §3 — The new export: `lock.IsOrphaned(contents LockContents) bool`

DECISION (rationale below): add a self-contained exported helper to `internal/lock/lock.go`:
```go
func IsOrphaned(contents LockContents) bool {
	pid, err := strconv.Atoi(contents.Pid)
	if err != nil { return false }                       // empty/malformed → false
	if !processAlive(pid, contents.Hostname) { return false } // dead holder is reaped, not "uselessly held"
	return appearsOrphaned(pid)                          // alive → check ppid==1
}
```
- `strconv` is ALREADY imported in lock.go (Status uses it). No new import.
- Mirrors Status's alive→orphan order EXACTLY → the read path (`lock status`) and the Busy hint can
  never disagree. A dead holder returns false (correct: a dead pid holds no flock — reapStaleLocks
  handles it — so it is never a "uselessly-held" orphan worth a kill hint).
- WHY NOT `IsOrphaned(pid int)`: the item description floats `lock.IsOrphaned(pid)`, but taking
  `LockContents` subsumes it (handles parse + the processAlive hostname logic) and matches what the
  caller already has. WHY NOT re-call `lock.Status`: re-reads the file (race) + recomputes path we
  already hold + needs repoPath not in the helper's natural input.
- Does NOT modify Status (LANDED + tested) — just adds a parallel helper with a comment linking them.

## §4 — Testability: the orphan==true branch is NOT unit-testable with a real orphan

A real orphan (ppid==1) is flaky/OS-dependent — P1.M3.T1.S1 and the T2.S1 PRP BOTH defer the
orphan==true path to the E2E harness (P1.M4.T1.S1). For THIS item we still want a FAST unit test of
the hint's CONDITIONAL + message format (orphan present / absent / empty-pid). Solution: a package-
level func-var SEAM in default_action.go, swappable in tests.

PRECEDENT (the codebase already does exactly this): `internal/cmd/config_init_interactive.go:20`:
```go
var interactiveStdinIsTTY = func() bool { return ui.IsTerminal(os.Stdin) }
```
So:
```go
// orphanChecker is the holder-orphan predicate for the Busy-message hint (FR-K5). Defaults to
// lock.IsOrphaned; tests override it to exercise the hint branch without a real orphaned process
// (only the E2E harness — P1.M4.T1.S1 — produces a genuine reparented-to-init holder).
var orphanChecker = lock.IsOrphaned
```
`.golangci.yml` does NOT enable `gochecknoglobals` (only errcheck/gosimple/govet/ineffassign/
staticcheck/unused) → the var is lint-clean, and it IS used (handleLockContention calls it).

## §5 — The new Busy format (FR-K5; item-contract wording verbatim)

```
stagecoach: another stagecoach run is already in progress on <repo> (pid <N> on <host>).
Your newly-staged changes will remain staged — re-run stagecoach after it finishes.

Lock: <path>
[ONLY if heldErr.Contents.Pid != "" AND orphanChecker(heldErr.Contents):]
The holder's launcher appears to have exited — it may be orphaned and holding this lock uselessly. You may safely `kill <N>` or `rm <path>` to clear it. See `stagecoach lock status`.
```
- Main message: SAME text as today MINUS the trailing ` Lock: %s.` (moved to its own line). Two
  sentences become two lines (`\n`), then a BLANK line (`\n\n`), then `Lock: <path>\n`.
- The hint uses the REAL pid `heldErr.Contents.Pid` (guaranteed non-empty by the `Pid != ""` guard —
  NOT the `<unknown>` fallback) and `heldErr.Path`.
- PRD anchors: FR-K5 = PRD line 568; example hint tone = PRD line 2047 ("If the holder's launcher has
  exited … orphaned and holding this lock uselessly; stagecoach lock status confirms, then kill/rm").

## §6 — Existing test impact (lock_contention_test.go) — MOSTLY UNCHANGED

CRUCIAL: the blank line is `\n\n` (two NEWLINES), NOT `"  "` (two spaces). The strict guard at
`TestHandleLockContention_Busy_EmptyDiagnostics` (`strings.Contains(msg, "  ")`) checks for two
SPACES → the new format PASSES it unchanged. Verified: no double-space anywhere in the new message
(main text, blank line, Lock line, hint all single-spaced).

Analysis of each existing test under the new format (default orphanChecker = lock.IsOrphaned):
- `_NoOpFastPath` — exit 0 path, UNCHANGED (no Busy print). PASS.
- `_Busy_TreeDiffers` (pid 4242, host "testhost") — assertions `Contains "4242"`/`"testhost"` still
  hold (main message unchanged). Hint? hostname "testhost" != this host → processAlive=true (foreign-
  host branch); appearsOrphaned(4242) reads local /proc|ps for 4242 → pid absent → false → NO hint.
  Deterministic-enough, but see §7 (pin orphanChecker=false for full determinism).
- `_Busy_EmptySnapshot` — PASS (no hint assertions).
- `_Busy_EmptyDiagnostics` (pid EMPTY) — `Pid==""` → hint guarded out → no hint; double-space guard
  still passes (\n\n). PASS.
- `_Busy_WriteTreeErr`, `_SilentExits` — PASS.
- `_RunDefault_LockReleasedAfterRun` — integration, no contention. UNAFFECTED.

## §7 — Test additions

1. UPDATE: pin the new "Lock: <path>" on its own line in a Busy test (regression guard against the old
   buried `Lock: %s.` form). Optional but recommended: set `orphanChecker = func(...) bool {return false}`
   (restore via t.Cleanup) in the non-hint Busy tests for full determinism (so absence of the hint does
   not depend on whether pid 4242 happens to look orphaned on the runner).
2. ADD `TestHandleLockContention_Busy_OrphanHint` — seam→true, pid set → assert hint present with the
   REAL pid (`kill 4242`) + path (`rm /x.lock`) + `stagecoach lock status`, Lock on own line.
3. ADD `TestHandleLockContention_Busy_NoOrphanHint` — seam→false → assert hint ABSENT, Lock present.
4. ADD `TestHandleLockContention_Busy_NoOrphanHintWhenPidEmpty` — seam→true BUT pid empty → hint ABSENT
   (proves the `Pid != ""` guard is independent of the predicate).
5. ADD `lock.IsOrphaned` unit test in `internal/lock/lock_unix_test.go` (next to the Status/orphan
   tests): empty pid→false, malformed pid→false, self (Hostname=this host)→`appearsOrphaned(getpid())`.

## §8 — Scope fences (no overlap with siblings)

- TOUCHES: `internal/lock/lock.go` (ADD IsOrphaned only), `internal/lock/lock_unix_test.go` (ADD test),
  `internal/cmd/default_action.go` (ADD seam var + REWRITE the Busy Fprintf block only), 
  `internal/cmd/lock_contention_test.go` (UPDATE/ADD assertions).
- DOES NOT TOUCH: `internal/cmd/lock.go` / `lock_test.go` (P1.M3.T2.S1 — parallel, different file, NO
  conflict), `internal/lock/orphan_*.go` / `lock_unix.go` / `lock_windows.go` (no edit — IsOrphaned
  reuses them as-is), `internal/lock/lock.go`'s `Status` (LANDED, tested — leave it), root.go, main.go,
  go.mod, any PRD/task file. NO new type/flag/dependency. [Mode A] godoc only — README sync is P1.M4.T2.S1.

## §9 — Validation commands (verified against Makefile + .golangci.yml)

```bash
go build ./... && GOOS=linux go build ./... && GOOS=darwin go build ./... && GOOS=windows go build ./...
go vet ./internal/cmd/... ./internal/lock/...
gofmt -l internal/lock/lock.go internal/lock/lock_unix_test.go internal/cmd/default_action.go internal/cmd/lock_contention_test.go
go test ./internal/cmd/ -run 'TestHandleLockContention' -race -v
go test ./internal/lock/ -run 'TestIsOrphaned|TestStatus' -race -v
go test ./internal/cmd/ ./internal/lock/ -race
make test && make lint && make build
```
IsOrphaned uses processAlive/appearsOrphaned which are build-tagged per-OS — all 4 GOOS targets must build.
