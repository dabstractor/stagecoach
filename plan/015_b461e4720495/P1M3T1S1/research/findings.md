# Codebase Findings — P1.M3.T1.S1 (generate.go CommitStaged → message-role timeout)

## 1. The dependency contract (ALREADY LANDED — consume, don't rebuild)

P1.M2.T1.S1 (COMPLETE per plan_status) shipped `ResolveRoleTimeout` in `internal/config/roles.go`:

```go
// roles.go:80 — THE function this task consumes (verified verbatim)
func ResolveRoleTimeout(role string, cfg Config) time.Duration {
	if rc, ok := cfg.Roles[role]; ok && rc.Timeout != 0 {
		return rc.Timeout                 // tier 1: per-role override (non-zero wins)
	}
	if d, ok := defaultRoleTimeouts[role]; ok {
		return d                          // tier 2: built-in role default (PLANNER=480s ONLY)
	}
	return cfg.Timeout                    // tier 3: global fallback
}
```

`defaultRoleTimeouts` has ONLY `"planner": 480 * time.Second` — there is NO "message" built-in.
**Therefore `ResolveRoleTimeout("message", cfg)` returns `cfg.Roles["message"].Timeout` if set
(non-zero), else `cfg.Timeout`.** This is the behavior-preserving key (§4).

Signature confirmed: `(role string, cfg Config) time.Duration`. The `config` package is ALREADY
imported by generate.go (for `config.ResolveRoleModel` at line 264). NO new import.

## 2. The EXACT three edit sites in generate.go (verified by read, anchored by STRING)

```go
// generate.go:264 — the ADD site (co-locate with the message-role MODEL resolution, FR-R7 parity)
_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)
// >>> ADD HERE: msgTimeout := config.ResolveRoleTimeout("message", cfg)   (immediately after this line)

// generate.go:335 — the one-shot Execute (the behavior change)
out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)
//                                  ^^^^^^^^^^  →  msgTimeout

// generate.go:426 — the multi-turn budget DISPLAY line (FR-T5: total = message-timeout × (N+1))
totalMin := int((cfg.Timeout * time.Duration(turns)).Minutes())
//               ^^^^^^^^^^  →  msgTimeout
```

`provider.Execute` signature (executor.go:44, verified): `func Execute(ctx context.Context, spec
CmdSpec, timeout time.Duration, vb *ui.Verbose) (stdout string, stderr string, err error)`. So the
3rd arg is a plain `time.Duration` — `msgTimeout` (time.Duration) drops in directly. No type change.

generate.go ALREADY imports `"time"` (import block line ~9) and `config` (for ResolveRoleModel).
**NO import changes.** `time.Duration` is used at line 426 already.

## 3. The S1/S2 scope boundary — generate.go vs multiturn.go/workdesc.go/hook-exec.go (THE crux)

The `cfg.Timeout` usages in the generate package split cleanly:

| Site | File | S1 (this) | S2 (sibling) |
|------|------|-----------|--------------|
| one-shot Execute | generate.go:335 | ✅ EDIT → msgTimeout | — |
| budget display | generate.go:426 | ✅ EDIT → msgTimeout | — |
| msgTimeout resolve | generate.go:264 | ✅ ADD | — |
| multi-turn Run CALL | generate.go:436 | ❌ LEAVE (see §5) | — |
| per-turn Execute ×3 | multiturn.go:165/176/187 | — | ✅ S2 |
| workdesc Execute ×3 | workdesc.go:75/106/122 | — | ✅ S2 |
| hook exec Execute | hook/exec.go (TBD) | — | ✅ S2 |

S1 touches ONLY `internal/generate/generate.go`. S2 owns `multiturn.go` + `workdesc.go` + `hook/exec.go`.

## 4. Behavior-preserving by default (the existing tests stay GREEN)

`ResolveRoleTimeout("message", cfg)` returns `cfg.Timeout` UNLESS `cfg.Roles["message"].Timeout`
is non-zero. The existing timeout test sets the GLOBAL, not the per-role:

```go
// generate_test.go:449 TestCommitStaged_Timeout (PRE-EXISTING — must stay green)
cfg := config.Defaults()
cfg.Timeout = 150 * time.Millisecond        // NO cfg.Roles["message"] → ResolveRoleTimeout returns this
m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: slow", SleepMS: 2000})
// ... CommitStaged → ErrTimeout at 150ms (msgTimeout == cfg.Timeout == 150ms — UNCHANGED behavior)
```

So after the change, with no message-role override, `msgTimeout == cfg.Timeout` → byte-identical
behavior. **The change ONLY diverges when a user sets `[role.message].timeout` / `--message-timeout`
/ `STAGECOACH_MESSAGE_TIMEOUT` / `stagecoach.role.message.timeout`** — exactly the FR-R7 feature.

The NEW behavioral proof (the test to ADD): flip WHICH field carries the small timeout — set the
GLOBAL large and the MESSAGE-ROLE small, assert the message-role timeout fires.

## 5. The line-436 Run call — DO NOT touch (deferred to S2; §d "see S2")

`Run` (multiturn.go:145) signature: `func Run(ctx, deps Deps, cfg config.Config, manifest,
sysPrompt, payload, msgModel, msgReasoning string) (msg, ok, cause)`. It takes `cfg` and uses
`cfg.Timeout` internally at multiturn.go:165/176/187 (the per-turn Execute calls).

The contract point (d) "Pass msgTimeout into the multi-turn Run call if it takes cfg.Timeout
internally (see S2)" is QUALIFIED by "(see S2)" — the Run-internal mechanism is S2's scope.

**Decision: S1 leaves line 436 and Run's signature UNTOUCHED.** S2 will resolve
`config.ResolveRoleTimeout("message", cfg)` locally at Run's Execute sites (multiturn.go). Why this
is safe + clean:
- `ResolveRoleTimeout` is PURE/DETERMINISTIC — calling it in CommitStaged (264) and again inside Run
  (S2) returns the IDENTICAL value for the same cfg. So the budget display (426, S1's msgTimeout) and
  Run's per-turn Execute (S2's resolved timeout) ALWAYS AGREE.
- NO signature change → S1 stays entirely in generate.go (its stated scope); S2 stays in multiturn.go.
  Zero S1/S2 coordination on a signature.
- With no message-role override (the default), msgTimeout == cfg.Timeout, so even before S2 lands the
  display (426) and Run's actual per-turn bound (cfg.Timeout) agree. The ONLY transient gap (display
  shows msgTimeout×turns while Run uses cfg.Timeout×turns) is when a user sets a message-role override
  AND runs between S1-landing and S2-landing — a narrow, sibling-pair transient S2 closes. S1+S2 are
  sibling subtasks of P1.M3.T1 meant to land together.

## 6. The stub-test harness (for the NEW test — reuse TestCommitStaged_Timeout's shape)

`stubtest.Manifest(bin, stubtest.Options{Out: "...", SleepMS: N})` renders a CmdSpec that the REAL
`provider.Execute` runs (a stub binary that sleeps N ms then prints Out). So a `cfg.Timeout` < SleepMS
→ `context.DeadlineExceeded` → `*RescueError{Kind: ErrTimeout}`. This is how TestCommitStaged_Timeout
works (150ms timeout vs 2000ms sleep). The NEW test reuses this EXACT mechanism — just sets the 150ms
on `cfg.Roles["message"]` (and the global to 30s) to prove the message-role timeout is the one used.

`RoleConfig` (config.go:38): `Provider, Model, Reasoning string; Timeout time.Duration` — set via
`cfg.Roles = map[string]config.RoleConfig{"message": {Timeout: 150 * time.Millisecond}}`.

## 7. ErrTimeout godoc (generate.go:87) — OPTIONAL polish, NOT required

Line 87: `// ErrTimeout is returned when generation exceeded cfg.Timeout (the agent was killed)`. After
this change the message-role one-shot is bounded by msgTimeout (== cfg.Timeout by default for message).
The comment is still accurate in spirit for the default case; updating it to "the resolved per-role
timeout" is a Mode-A nicety but the contract says "DOCS: none — internal wiring". LEAVE it (out of
scope) — optionally refresh in a later docs task. Do not expand scope for it.

## 8. Validation commands (verified against Makefile)

- `go build ./...` + `GOOS=windows` + `GOOS=linux` (no platform tags in generate.go).
- `go vet ./internal/generate/...`
- `gofmt -l internal/generate/generate.go internal/generate/generate_test.go` (empty)
- `go test ./internal/generate/ -run 'TestCommitStaged_Timeout' -v` (REGRESSION — must stay green)
- `go test ./internal/generate/ -run 'TestCommitStaged_MessageRoleTimeout' -v` (NEW)
- `make test` (race) ; `make lint` ; `make coverage-gate` (internal/generate IS gated ≥85%).
- grep guards (see PRP Validation Level 4).

## 9. What this task is NOT (scope fences)

- NOT multiturn.go / workdesc.go / hook-exec.go (S2).
- NOT the Run signature (S2; §5).
- NOT ResolveRoleTimeout itself (P1.M2.T1.S1, LANDED).
- NOT the global 480s→120s flip (P1.M2.T2.S1, in-flight parallel — but my change is INDEPENDENT of the
  global's value: msgTimeout resolves to cfg.Timeout when no message override, whatever that is).
- NOT the planner/stager/arbiter roles (P1.M3.T2 — decompose path).
- NOT docs/README (P1.M4.T2; contract: "DOCS: none").
- NOT a new flag (P1.M1.T2.S2 added `--message-timeout`; already LANDED — this task CONSUMES it via cfg).
