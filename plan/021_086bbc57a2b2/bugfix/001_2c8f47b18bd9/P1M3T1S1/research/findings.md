# Findings — P1.M3.T1.S1 (BUG-004: cmdRunner.Run per-query timeout + WaitDelay)

## 1. The bug + the reference (both verified verbatim)

**The bug** — `internal/cmd/upgrade_run.go:152` `cmdRunner.Run`:
```go
func (cmdRunner) Run(ctx context.Context, name string, args ...string) (string, int, error) {
	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)   // NO context.WithTimeout; NO cmd.WaitDelay
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		if cerr := ctx.Err(); cerr != nil { return buf.String(), 0, cerr }  // ctx.Err() check IS present...
		var ee *exec.ExitError
		if errors.As(err, &ee) { return buf.String(), ee.ExitCode(), nil }
		return "", 0, err
	}
	return buf.String(), 0, nil
}
```
The error-handling (ctx.Err() FIRST, then ExitError⇒skip, then start-failure⇒err) is ALREADY CORRECT and
mirrors osRunner — but it was DEAD CODE for the hang case: with no per-query timeout, `ctx.Err()` is only
non-nil on a signal-cancel (Ctrl-C), so a hung PM query (brew refreshing DB, NFS home, broken PM) blocks
`cmd.Run()` indefinitely. The stale comment at lines 141-142 ("There is NO per-query timeout here … because
the command's ctx already bounds the whole upgrade") is FALSE: main.go:62 builds the root ctx via
`signal.Install(context.Background(), …)` — signal-cancelable, NO deadline.

**The reference** — `internal/upgrade/detect.go:121` `osRunner.Run` (CORRECT; unreachable from package cmd
because `osRunner` is UNEXPORTED):
```go
const defaultQueryTimeout = 3 * time.Second            // detect.go:112
func (r *osRunner) Run(ctx context.Context, name string, args ...string) (string, int, error) {
	timeout := r.timeout
	if timeout == 0 { timeout = defaultQueryTimeout }   // 0 ⇒ 3s default
	ctx, cancel := context.WithTimeout(ctx, timeout)    // ← THE missing piece #1
	defer cancel()
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &out
	cmd.WaitDelay = timeout                              // ← THE missing piece #2
	if err := cmd.Run(); err != nil {
		if cerr := ctx.Err(); cerr != nil { return out.String(), 0, cerr }  // deadline ⇒ skip
		...ExitError ⇒ skip; start-failure ⇒ err...
	}
	return out.String(), 0, nil
}
```

## 2. The fix — replicate the 3s deadline + WaitDelay (Option A; the contract's PREFERRED)

Two missing pieces, both in `cmdRunner.Run`:
1. `ctx, cancel := context.WithTimeout(ctx, upgrade.DefaultQueryTimeout); defer cancel()` — at the top of Run.
2. `cmd.WaitDelay = upgrade.DefaultQueryTimeout` — right after `cmd := exec.CommandContext(...)`.

The error-handling block is UNCHANGED (it's already correct; the timeout now makes `ctx.Err()` reachable).
The stale comment (141-142) is rewritten (Mode A: remove the false "no per-query timeout / ctx bounds the
upgrade" claim; document the 3s deadline + WaitDelay + why the root ctx has no deadline).

**Shared constant (prevents drift — the root cause of this bug):** the two runners DRIFTED (cmdRunner lost
the timeout). Export `defaultQueryTimeout` → `DefaultQueryTimeout` in detect.go and reference
`upgrade.DefaultQueryTimeout` from cmdRunner so they CANNOT drift again. Verified safe: `defaultQueryTimeout`
is referenced ONLY in detect.go (lines 106/109/112/124) — ZERO test references, ZERO other callers.

**No new import in upgrade_run.go:** `upgrade.DefaultQueryTimeout` is a `time.Duration`; assigning it to
`cmd.WaitDelay` (a `time.Duration` field) and passing it to `context.WithTimeout` does NOT require package
cmd to import `"time"` (Go only requires importing a package when you NAME its identifiers). upgrade_run.go
already imports `context` + `os/exec` + `upgrade`.

**Alternative (hardcode `3 * time.Second`):** simpler diff (no detect.go change) BUT requires adding
`"time"` to upgrade_run.go's import block AND reintroduces the drift risk. The contract marks the shared
constant "Preferred" — use it.

**Rejected (Option B):** exporting `osRunner` itself and using it in `prodDetect` "changes the seam
architecture" (the contract's words). cmdRunner is the package-cmd test-seam twin (P1.M4.T3 overrides
`upgradeDetect`); keep it, just give it the timeout.

## 3. The exact detect.go rename (3 edits; export the constant)

- `const defaultQueryTimeout = 3 * time.Second` (line 112) → `const DefaultQueryTimeout = 3 * time.Second`
  (+ update its godoc to note it's shared with cmd.cmdRunner).
- osRunner.timeout field comment (line 106): `0 ⇒ 3s default (defaultQueryTimeout)` → `(DefaultQueryTimeout)`.
- line 124: `timeout = defaultQueryTimeout` → `timeout = DefaultQueryTimeout`.

## 4. Scope fences

- **THIS task (S1)**: cmdRunner.Run timeout+WaitDelay (upgrade_run.go) + the comment rewrite + the
  detect.go constant export (3 edits).
- **NOT this task**: BUG-007 (Linuxbrew Cellar heuristic — P1.M3.T2), BUG-008 (escape c.Repo — P1.M3.T3),
  the docs sync (P1.M3.T4), osRunner's logic (just export the constant), the parallel P1.M2.T3.S1 (decompose
  docs — zero overlap).
- The previous PRP (P1.M2.T3.S1) is a docs-only audit of README/how-it-works for the decompose fast-path —
  completely unrelated to upgrade_run.go. No file overlap, no conflict.

## 5. Test consideration (optional; the contract lists no test)

The contract's OUTPUT + DOCS list no test. The existing upgrade tests (P1.M4.T3) inject a canned `fakeRunner`,
NOT the real cmdRunner — so no existing test exercises the timeout. A regression test would run a hanging
command (`sleep 30`, Unix-only `//go:build !windows`) and assert Run returns within ~3.5s with
`errors.Is(err, context.DeadlineExceeded)`. It's a 3s test (slow but deterministic). Recommend as OPTIONAL
value-add; the PRIMARY validation is build/vet/gofmt + the existing upgrade suite green + grep guards.

## 6. Validation

- `go build ./...` + cross-build; `go vet ./internal/cmd/... ./internal/upgrade/...`; `gofmt -l` on both files.
- `go test ./internal/upgrade/...` + `go test ./internal/cmd/...` (existing suites green — additive change).
- `make test` + `make lint`.
- grep guards (see PRP Validation Level 4).