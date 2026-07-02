# Research — P1.M6.T3.S1: internal/generate/stubprovider_test.go (fake agent)

## What this task is
A reusable **test asset** (not shipped code): a fake coding-agent CLI used as
`provider.Manifest.Command` so the generate integration suite (T3.S2) and the
invariant/property tests (T3.S3) can drive `generate.CommitStaged` end-to-end
with deterministic, scriptable agent behavior — replacing the real agent.
PRD §20.1 layer 3; decisions.md §8; reference_impl.md (loop structure).

## Input dependency: provider.Executor (P1.M2.T4.S1) — `internal/provider/executor.go`
- `Executor.Run(ctx, m Manifest, model, provider, sys, payload string) (string, error)`.
- Builds `exec.Command(m.Command, r.Args...)` from `m.Render(...)`; **never sh -c**.
- `cmd.Dir = e.Dir`; `cmd.Env = append(os.Environ(), r.Env...)` ⇒ manifest `Env`
  additions reach the child AND take precedence on conflict (appended last).
- stdin: when `r.DeliverViaStdin` (PromptDelivery "stdin"/""), `cmd.Stdin = bytes.NewReader([]byte(payload))`.
- Timeout/cancel: `ctx.Done()` → SIGTERM whole process group (`Setpgid`), 2s grace, SIGKILL.
  Deadline → `*provider.TimeoutError` (Unwrap = context.DeadlineExceeded); cancel → context.Canceled.
- Non-zero exit → `*provider.AgentError{Name, Command, Code, Stderr}`.
- Conclusion: the stub is just an executable at `m.Command`; config reaches it via the
  manifest's `Env` map (clean, per-invocation, NOT leaked to the test's `os.Environ`).

## Manifest fields the stub wiring uses (internal/provider/manifest.go)
- `Command` = path to the stub binary.
- `PromptDelivery` = `provider.DeliveryStdin` ("stdin", the §12.1 default) ⇒ payload piped to stub stdin.
- `Env map[string]string` ⇒ merged into child env by the Executor (the config channel).

## Design decision: testdata compiled helper binary (NOT test-binary re-exec)
Considered two approaches:
1. **Re-exec the test binary** (`m.Command = os.Args[0]`, child branch gated by env,
   `-test.run` baked into Subcommand) — matches signal_test.go but: re-runs test machinery
   per invocation, needs `-test.run` hacks, couples stub to test framework.
2. **testdata helper binary** (`internal/generate/testdata/stubagent/main.go`, `package main`,
   compiled once via `go build`) — clean standalone executable; `m.Command` is just a path;
   downstream consumers get a trivial `NewStubManifest(t, cfg)`; lean per-invocation.

**Chose #2.** Verified Go tooling behavior empirically in this repo's toolchain (go1.26):
- `go build ./internal/generate/` does NOT build testdata (testdata excluded). ✅
- `go build ./internal/generate/testdata/stubagent` (explicit path) DOES build it. ✅
- `go vet ./...` / `go vet ./internal/generate/` skips testdata. ✅
- `gofmt -l internal/generate/` DOES walk into testdata (lists the binary too). ✅
GOTCHA: a compile error in testdata/stubagent is invisible to `go build ./internal/generate/`
and `go vet`; it surfaces ONLY via (a) explicit `go build ./internal/generate/testdata/stubagent`
or (b) the test's `BuildStubBinary` running that build. Both are validation gates below.

## Configuration model (env-based, stateful across calls)
The two-nested-loop orchestrator spawns a NEW stub process per agent call (inner parse loop
+ outer dup loop), so per-call state can't live in-process memory. Mechanism:
- `STAGEHAND_STUB_SCRIPT` = JSON `[]StubResponse` (fields `emit`, `hang`, `fail`).
- `STAGEHAND_STUB_STATE` = path to a counter file; stub reads index N, writes N+1, selects
  `script[min(N, len-1)]` (CLAMP — predictable "last entry wins after exhaustion").
- `STAGEHAND_STUB_STDIN` = optional path; stub writes received stdin here (payload-delivery proof).
- Precedence per entry: Hang (block forever) > Fail>0 (exit code + stderr) > Emit (stdout, exit 0).

This covers all 4 contract behaviors:
- (a) canned valid: `[{Emit:"feat: x\n\nbody"}]`.
- (b) empty/garbage then success: `[{Emit:""},{Emit:"feat: ok"}]` (stateful); always-fail: `[{Emit:""}]`.
- (c) dup subject: `[{Emit:"<dup subject>"},{Emit:"<unique>"}]` (dup-retry-then-success) or single dup.
- (d) hang/timeout: `[{Hang:true}]` ⇒ Executor ctx timeout kills the group → `*TimeoutError`.

## Coupling note
The testdata binary's `stubResponse` struct MUST mirror `generate.StubResponse` JSON tags
exactly (the test marshals, the binary unmarshals the same bytes). Documented as a gotcha.

## Self-test coverage (MOCKING: assert stub emits configured behavior)
Each behavior asserted THROUGH the real Executor (proves manifest wiring: Command path,
Env propagation, stdin piping, process-group timeout/AgentError typing):
- valid message emitted exactly.
- stateful empty→then→valid (counter advanced across two Run calls).
- hang → `*provider.TimeoutError` under a short ctx (real process-group kill path).
- non-zero Fail → `*provider.AgentError{Code}`.
- stdin captured to StdinLog file contains the delivered payload.
- dup-subject-then-unique script emits in order (outer-loop wiring proof).

## Downstream consumption (T3.S2 e2e, T3.S3 invariants)
Helpers must be reusable by sibling _test.go files in `internal/generate`. House convention is
WHITE-BOX (`package generate`) for every generate _test.go (dedupe/rescue/signal all `package generate`).
So helpers live in `package generate`; consumers follow the same convention. Names exported
(`BuildStubBinary`, `NewStubManifest`, `StubConfig`, `StubResponse`) for unambiguous reuse.

## Validation gates (verified executable; ONE command each, no chained checks)
1. `test -z "$(gofmt -l internal/generate/)"` — formats test file AND testdata binary.
2. `go build ./internal/generate/` — package compiles (testdata excluded).
3. `go build ./internal/generate/testdata/stubagent` — testdata binary compiles.
4. `go vet ./internal/generate/` — vet package (testdata excluded; covered by build+gofmt).
5. `go test ./internal/generate/ -run StubProvider -v` — self-tests build the binary + assert all behaviors.
6. `go test ./internal/generate/` — full package regression (rescue/dedupe/signal unaffected).

## Scope boundary
- This task delivers the stub asset + its self-tests ONLY. It MUST NOT implement
  CommitStaged/generate.go (P1.M6.T1.S1) or the e2e/invariant suites (T3.S2/T3.S3).
- No real LLM, no network, no new external deps (stdlib + internal/provider only).
- Docs: no Mode-A per-item DOCS line; defers to Mode-B changeset-level doc sync in M8.
