# Research Notes — P1.M2.T1.S3 (thread 13 role call sites to ResolveRoleDeadlines)

Scope: replace every `config.ResolveRoleTimeout(role, …)` + positional-duration Execute call with
`config.ResolveRoleDeadlines(role, cfg)` threaded as `provider.Deadlines{Idle, Hard}`; rework the
multiturn budget line (hard × (N+1) + "(worst case)") in generate.go AND its hook twin; update the
stale FR-R7 comments; Mode-A docs (how-it-works.md). Zero error-branch edits (FR-A5).

## 1. Ground truth: the Execute call sites (grep-verified at research time)

Non-test `provider.Execute(` callers — 14 total = 13 ROLE sites + models.go:
1. internal/generate/generate.go:374 — message one-shot (resolve @ :303)
2-4. internal/generate/multiturn.go:172 / 183 / 194 — message turns 1 / 2..N / N+1 (resolve @ :153 inside Run)
5-7. internal/generate/workdesc.go:82 / 131 / 149 — workdesc turn1 / forced-conclusion / read-answer (resolve @ :71)
8. internal/decompose/planner.go:128 (resolve @ :69)
9. internal/decompose/stager.go:119 (resolve @ :98)
10. internal/decompose/message.go:173 (resolve @ :145)
11. internal/decompose/arbiter.go:101 (resolve @ :83)
12. internal/hook/exec.go:187 (resolve @ :167)
13. pkg/stagecoach/stagecoach.go:569 — the LIBRARY twin of the one-shot loop, passes raw `cfg.Timeout`
    (NOT ResolveRoleTimeout today). It IS a message-role invocation (FR-A4 "every role invocation
    deadline-bound") — thread it to ResolveRoleDeadlines("message", cfg) too. This reconciles S2's
    "13 role sites" (12 enumerated + this twin).
14. internal/cmd/models.go:149 — NOT a role call; S2 leaves it on `Deadlines{Idle: 0, Hard: timeout}`
    + comment permanently. S3 NEVER touches it.

S2 (parallel, Implementing) reworks Execute to `(ctx, spec, dl Deadlines, vb)` and mechanically
migrates all 14 sites to `provider.Deadlines{Hard: X}` (idle disarmed, byte-identical v3.4).
**S3 replaces the 13 role sites' placeholders with fully-resolved Deadlines.** If S2 hasn't landed,
the Deadlines type/shape won't exist — the expected prerequisite signal.

## 2. Resolution API (internal/config/roles.go — landed, P1.M1.T1.S1 Complete)
`func ResolveRoleDeadlines(role string, cfg Config) (idle, hard time.Duration)` — per-role
`cfg.Roles[role].{IdleTimeout,Timeout}` (0 ⇒ inherit) > globals `cfg.IdleTimeout/cfg.Timeout`
(Defaults() seeds 60s/600s). Independent resolution. Call-site shape:
```go
msgIdle, msgHard := config.ResolveRoleDeadlines("message", cfg)
...
provider.Execute(ctx, *spec, provider.Deadlines{Idle: msgIdle, Hard: msgHard}, deps.Verbose)
```
`ResolveRoleTimeout` becomes production-caller-FREE after S3 but keeps its roles_test.go tests
(TestResolveRoleTimeout_* @195+) — KEEP the function, UPDATE its stale doc line "KEPT for its 11
existing callers" (now test-only / hard-cap accessor).

## 3. The budget lines (verbatim current shapes)
generate.go:465-477: `turns := len(chunkPayload(...)) + 1; totalMin := int((msgTimeout * time.Duration(turns)).Minutes()); if totalMin < 1 {totalMin=1}; fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns (chunks of ~%d tokens), ~%dm total\n", turns, cfg.MultiTurnChunkTokens, totalMin)`
hook/exec.go:~255-262 twin: same, `turns := generate.ChunkCount(mtPayload, cfg.MultiTurnChunkTokens) + 1`, Fprintf split across 2 lines.
S3 edit (both): multiply the resolved message HARD cap (msgHard) by (N+1); append " (worst case)" to
the format string. Rationale: N+1 INDEPENDENT per-invocation deadlines (fresh idle clock per turn,
FR-A4) — hard×(N+1) is only the worst-case ceiling, not a shared budget.

## 4. Stale FR-R7 comments to rewrite (name BOTH deadlines + FR-A4 fresh-idle-clock)
- generate.go:299-303 ("and the multi-turn total budget, FR-T5" clause is the stale shared-budget assumption)
- multiturn.go:150-153 (same clause; doc @132-135 "Execute shadows ctx with WithTimeout")
- workdesc.go:68-71
- planner.go:66-69 (480s narrative — MUST-CHANGE per docs_drift_inventory; planner 480s retired)
- stager.go ~96-98, message.go ~143-145, arbiter.go ~81-83 (same FR-R7 single-timeout comments)
- hook/exec.go:163-167 ("and the multi-turn total budget, FR-T5" clause)
Comment shape: "FR-R7/§9.30: resolve BOTH deadlines (idle + hard cap); each Execute invocation binds
them individually — a fresh idle clock per turn/round (FR-A4). No per-role override ⇒ the globals
(60s idle / 600s hard)."

## 5. Error branches — DO NOT EDIT (FR-A5 / item contract)
All 8 consumers' branches already match `context.DeadlineExceeded`; S2's ErrIdleDeadline sentinel
satisfies errors.Is unchanged. PRESERVE the existing exit asymmetry verbatim: one-shot (generate.go:376)
+ workdesc (generate.go:340) mint ErrTimeout → exit 124; multiturn/workdesc turn aborts surface as
ErrRescue → exit 3; arbiter swallows DeadlineExceeded‖Canceled → graceful null; hook never-block.
models.go keeps Idle:0. NO exitcode/rescue/CAS/lock changes.

## 6. Docs (Mode A, per docs_drift_inventory.md)
- docs/how-it-works.md L346 (verified): "Each turn is a separate provider invocation bounded by the
  **message role's** resolved timeout (/); total wall-clock ≈ `message-timeout × (N+1)`" — STALE.
  Rework: two deadlines (idle + hard cap) bound each turn INDIVIDUALLY (fresh idle clock per turn,
  FR-A4); the progress line's total is the worst-case ceiling `hard-cap × (N+1)`.
- docs/cli.md L452 exit-124 row: ALREADY reworded to "Deadline exceeded (idle or hard cap)…" (verified
  — landed with P1.M1.T1.S3's Mode-A docs). S3 = VERIFY-ONLY (grep the row; no edit if present).

## 7. Test harness (per-family proof that resolved Deadlines reach Execute)
Idiom: LARGE/SMALL role knobs + sleeping stub (`stubtest.Manifest(bin, Options{Out:"…", SleepMS:2000})`).
Existing hard-cap proofs to mirror: invariants_test.go Timeout row (generate one-shot),
multiturn_test.go TestRun_MessageRoleTimeoutBoundsTurn (L246), generate_workdesc_test.go
TestCommitStaged_WorkDescription_MessageRoleTimeout (L288), decompose planner_test.go:348 /
stager_test.go:167 / message_test.go:219 / arbiter_test.go:237, hook exec_test.go
TestRun_MessageRoleTimeoutNeverBlock (L298).
S3 idle-variant recipe: `cfg.Timeout = 3s` (bounded so non-Linux completes via hard cap),
`cfg.Roles["message"] = RoleConfig{IdleTimeout: 150ms}` (SMALL idle), stub SleepMS 2000 (silent) →
assert errors.Is(err, context.DeadlineExceeded). On Linux (dev box + linux CI) the idle watcher arms
(rchar channel; D4) and kills at ~150ms; on darwin/windows print-mode the idle is DISARMED and the 3s
hard cap lands — same sentinel, same assert, bounded duration either way. One new test per family:
generate / multiturn turn / workdesc round / decompose role (planner) / hook.
Note: the hook budget line is grep-guarded by an existing exec_test note (~L315) — update the guard
string to the "(worst case)" wording.
