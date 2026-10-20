# PRP — P1.M2.T1.S3: thread the 13 role call sites to ResolveRoleDeadlines + multiturn '(worst case)' budget line + Mode-A docs (FR-A4/FR-R7, §9.30)

> **Mechanical threading task.** S2 (parallel, Implementing) lands `provider.Execute(ctx, spec, dl
> Deadlines, vb)` + the mechanical `provider.Deadlines{Hard: X}` placeholders at every call site.
> **S3 replaces the 13 ROLE sites' placeholders with fully-resolved deadlines** from
> `config.ResolveRoleDeadlines(role, cfg)` (landed, P1.M1.T1.S1 Complete), reworks the two multiturn
> budget lines, rewrites the stale FR-R7 comments, and lands the Mode-A docs. **Zero error-branch
> edits** — FR-A5 guarantees parity; the existing exit asymmetry (one-shot/workdesc → 124, multiturn
> turn → 3) is explicitly PRESERVED, not fixed. Bug-fix-style task: brings call sites INTO line with
> FR-A4 ("every role invocation deadline-bound"). Stdlib-only.

---

## Goal

**Feature Goal**: Every role agent invocation (one-shot, each multi-turn turn, each work-description
round, planner/stager/message/arbiter, the hook one-shot, and the library twin) is bound by BOTH
§9.30 deadlines — idle (primary) + hard cap — resolved per role via the FR-R7 ladder, passed as
`provider.Deadlines{Idle, Hard}` to `provider.Execute`. The multiturn progress line stops claiming a
shared wall-clock budget and reports `hard × (N+1)` as a worst-case ceiling.

**Deliverable** (all edits; no new files):
1. **8 resolution sites** across `internal/generate/{generate,multiturn,workdesc}.go`,
   `internal/decompose/{planner,stager,message,arbiter}.go`, `internal/hook/exec.go` — each
   `xTimeout := config.ResolveRoleTimeout(role, cfg)` becomes `xIdle, xHard :=
   config.ResolveRoleDeadlines(role, cfg)` feeding **all** Execute calls in scope (12 sites).
2. **pkg/stagecoach/stagecoach.go:569** — the 13th role site (library twin): raw `cfg.Timeout` →
   `ResolveRoleDeadlines("message", cfg)`.
3. **generate.go:465-477 + hook/exec.go:~255-262** — budget line: `msgHard × (N+1)` + `" (worst case)"`.
4. **Stale FR-R7 comments** rewritten at every former ResolveRoleTimeout site (both deadlines + FR-A4
   fresh-idle-clock; purge the 480s narrative in planner.go).
5. **internal/config/roles.go** — ResolveRoleTimeout's doc line updated (production-caller-free; kept
   as the test-pinned hard-cap accessor). Do NOT delete the function.
6. **5 new per-family tests** (generate / multiturn / workdesc / decompose-planner / hook) asserting
   the resolved per-role DEADLINES actually reach Execute.
7. **docs/how-it-works.md** L346 deadline prose rework; **docs/cli.md** L452 exit-124 row verify-only.

**Success Definition**: with a per-role `IdleTimeout` configured (global hard cap large), a silently
hung stub agent at any of the five families is killed and surfaces the SAME error/exit as a timeout
today (DeadlineExceeded through the unchanged branches); with defaults, behavior is byte-identical to
S2's placeholders (60s/600s resolve to the same hard cap the placeholders carried, idle armed per D4).
`go build ./...`, `make test`, `make lint` green; `go vet` clean.

---

## Why

- **FR-A4/FR25 (§9.30)**: "Both bind every provider invocation of that role individually — one-shot,
  each multi-turn turn (FR-T5), each work-description round (§9.26), hook-mode message generation —
  with a fresh idle clock per invocation." The structure is already per-invocation (each Execute call
  passes its own duration); only the *kind* of deadline changes. S3 is the arming step: S2 built the
  engine, S1 built the resolver, P1.M1 built the config — S3 connects them.
- **Consumed downstream**: P1.M3.T1.S2-S4 (pipeline parity tests) exercise idle-kills through these
  call sites; they require the threading this task lands.
- **Honesty fix (budget line)**: the progress line's `~%dm total` currently implies one shared
  budget; under N+1 independent deadlines it is only the worst case — the wording must say so.

---

## What

**User-visible behavior** (post-S3, on Linux where the idle channel arms): a silently hung agent is
killed at the role's idle deadline (default 60s) instead of burning the full hard cap; everything
else (exit codes, rescue, no-retry) is unchanged (FR-A5). The multiturn fallback line gains
" (worst case)".

**Technical change**: mechanical. At each site the resolution line swaps to the two-value form and
each in-scope `provider.Execute(ctx, *spec, provider.Deadlines{Hard: X}, deps.Verbose)` becomes
`provider.Execute(ctx, *spec, provider.Deadlines{Idle: xIdle, Hard: xHard}, deps.Verbose)`. NO
error-branch, exit-code, rescue, retry, CAS, lock, or index changes anywhere.

### Success Criteria

- [ ] All 13 role Execute calls pass `provider.Deadlines{Idle: xIdle, Hard: xHard}` resolved via
      `config.ResolveRoleDeadlines(role, cfg)`; `grep -rn 'ResolveRoleTimeout' --include=*.go` in
      non-test production code returns ZERO hits.
- [ ] models.go keeps `Deadlines{Idle: 0, Hard: timeout}` + its not-a-role-call comment (untouched).
- [ ] Both budget lines multiply the HARD cap by turns and append " (worst case)".
- [ ] The 8 stale FR-R7 comments name both deadlines + the FR-A4 fresh-idle-clock; planner.go's 480s
      narrative is purged.
- [ ] ResolveRoleTimeout retained with an updated doc comment (tests still pin it).
- [ ] 5 new per-family tests pass; existing per-caller suites pass with the new call shape.
- [ ] how-it-works.md L346 reworked; cli.md L452 verified already-correct.
- [ ] No error-branch/exit-code edits (the one-shot→124 vs multiturn→3 asymmetry preserved verbatim).

---

## All Needed Context

### Context Completeness Check

An implementer who has never seen this repo gets: the exact 13-site table (file:line + role +
resolution site), the verbatim before/after call shapes, the budget-line verbatim text, the comment
template, the test recipe with its platform nuance (D4 arming), and the S2 contract (Deadlines struct
shape). All below.

### Documentation & References

```yaml
# MUST READ — the authoritative call-site + error-contract inventory (verbatim code at every site)
- docfile: plan/022_856008a8cbb5/architecture/call_sites_and_error_contract.md
  why: "§2 = the call-site table + §2.1-2.10 verbatim code for every site this task edits (the
        error branches that must stay UNTOUCHED). §5 = the budget line verbatim (both twins). §7 = the
        exit-code asymmetry (multiturn turn-timeout → ErrRescue/3, not 124) — PRESERVE it. §9.3/9.5 =
        the existing test harness + the per-role timeout proofs to mirror."
  critical: "§7's asymmetry note is the trap: do NOT 'fix' the multiturn caller to mint ErrTimeout —
             the item contract explicitly says DO NOT change it (FR-A5: only the verbose cause line
             differs; the mapping stays as-is today)."

# MUST READ — the S2 PRP (the Execute/Deadlines CONTRACT this task consumes)
- docfile: plan/022_856008a8cbb5/P1M2T1S2/PRP.md
  why: "S2 lands provider.Deadlines{Idle, Hard time.Duration} + Execute(ctx, spec, dl, vb) and
        mechanically migrates all 14 non-test sites to Deadlines{Hard: X} placeholders. S3 replaces
        the 13 ROLE placeholders (models.go keeps Idle:0 permanently). S2's '13 role sites' = the 12
        enumerated + pkg/stagecoach.go:569 (the library twin)."
  critical: "Treat S2 as a CONTRACT. If Deadlines/Execute's new signature isn't in the tree, S2 hasn't
             landed — that's the prerequisite signal, not a bug. Do NOT edit executor.go/activity*.go/
             render.go (S2/S1/P2 domains)."

# MUST READ — the resolver (landed; the API being threaded)
- file: internal/config/roles.go
  why: "ResolveRoleDeadlines(role, cfg) (idle, hard time.Duration) @78 — the two-value form; globals
        cfg.IdleTimeout/cfg.Timeout (Defaults() 60s/600s); independent per-field inherit (0 ⇒ inherit).
        ResolveRoleTimeout @104 = thin hard-cap accessor — after S3 it has ZERO production callers but
        keeps its roles_test.go tests (TestResolveRoleTimeout_* @195+)."
  gotcha: "Update ResolveRoleTimeout's doc line 'KEPT for its 11 existing callers' (now stale) but do
           NOT delete the function — its tests pin the accessor and any external caller surface."

# MUST READ — the FR spec
- docfile: plan/022_856008a8cbb5/prd_snapshot.md
  section: "§9.30 (FR-A1-A5) + §9.15 FR-R7 + FR25"
  why: "FR-A4 (both deadlines bind every invocation individually; fresh idle clock per turn/round),
        FR-A5 (idle kill downstream-identical to timeout; only the --verbose cause line differs),
        FR-R7 (the per-role precedence ladder for both deadlines)."

# MUST READ — the docs drift inventory (the Mode-A docs anchors)
- docfile: plan/022_856008a8cbb5/architecture/docs_drift_inventory.md
  why: "Names docs/how-it-works.md L346 (the stale 'message-timeout × (N+1)' shared-budget prose) and
        the internal comment MUST-CHANGE rows (planner.go:66-67's 480s narrative etc.)."
  gotcha: "docs/cli.md's exit-124 row ALREADY reads 'Deadline exceeded (idle or hard cap)' (landed
           with P1.M1.T1.S3) — VERIFY-ONLY, do not re-edit."

# The files being edited (locate by content, not line — S2 lands in parallel and shifts lines)
- file: internal/generate/generate.go       # :303 resolution, :374 Execute, :465-477 budget line
- file: internal/generate/multiturn.go      # :153 resolution, :172/183/194 Executes, :132-135 doc
- file: internal/generate/workdesc.go       # :71 resolution, :82/131/149 Executes
- file: internal/decompose/planner.go       # :69 resolution (480s comment), :128 Execute
- file: internal/decompose/stager.go        # :98 resolution, :119 Execute
- file: internal/decompose/message.go       # :145 resolution, :173 Execute
- file: internal/decompose/arbiter.go       # :83 resolution, :101 Execute
- file: internal/hook/exec.go               # :167 resolution, :187 Execute, ~:255-262 budget twin
- file: pkg/stagecoach/stagecoach.go        # :569 Execute (raw cfg.Timeout today)
- file: internal/cmd/models.go              # READ-ONLY — Idle:0 permanent; NEVER touch

# CONTEXT — the S1 arming rule (why the idle test has a platform nuance)
- file: internal/provider/executor.go   # (post-S2) watcher armed iff Idle>0 && (linux || ProgressiveStdout)
  why: "Explains the test recipe: on Linux the rchar channel arms the idle watcher for the stub
        (print mode); on darwin/windows print mode the idle is DISARMED and the bounded hard cap lands
        — same DeadlineExceeded sentinel either way, so one assert serves both."
```

### Current Codebase tree (relevant slice — S2 in flight adds Deadlines/Execute rework)

```bash
internal/config/roles.go            # EDIT (doc line only) — ResolveRoleDeadlines exists, Complete
internal/generate/generate.go       # EDIT — resolution + Execute + budget line + comment
internal/generate/multiturn.go      # EDIT — resolution + 3 Executes + doc comment
internal/generate/workdesc.go       # EDIT — resolution + 3 Executes + comment
internal/decompose/planner.go       # EDIT — resolution + Execute + comment (purge 480s)
internal/decompose/stager.go        # EDIT — resolution + Execute + comment
internal/decompose/message.go       # EDIT — resolution + Execute + comment
internal/decompose/arbiter.go       # EDIT — resolution + Execute + comment
internal/hook/exec.go               # EDIT — resolution + Execute + budget twin + comment
pkg/stagecoach/stagecoach.go        # EDIT — :569 library twin threaded
internal/provider/executor.go       # READ-ONLY — S2's engine (Deadlines, ErrIdleDeadline, watchIdle)
internal/cmd/models.go              # READ-ONLY — Idle:0 permanently
docs/how-it-works.md                # EDIT — L346 deadline prose
docs/cli.md                         # VERIFY-ONLY — exit-124 row already reworded
+ the per-family test files (see Tasks)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL 1 (site count): 13 ROLE sites = the 12 enumerated (generate:374, multiturn:172/183/194,
//   workdesc:82/131/149, planner:128, stager:119, message:173, arbiter:101, hook:187) + pkg/stagecoach.
//   go:569 (the library twin — a message-role one-shot passing raw cfg.Timeout today). models.go is
//   the 14th non-test site and is NEVER a role site. Verify with:
//   grep -rn 'provider.Execute(' --include='*.go' internal/ pkg/ cmd/ | grep -v _test.go   → 14 hits.

// CRITICAL 2 (S2 dependency): the tree shows the OLD Execute(ctx, spec, timeout, vb) until S2 lands.
//   S3 assumes S2's contract: Execute(ctx, spec, dl provider.Deadlines, vb) with Deadlines{Idle, Hard}.
//   If it hasn't landed, compile failure at the first edit is the expected prerequisite signal.

// CRITICAL 3 (NO error-branch edits — FR-A5): every site's DeadlineExceeded/Canceled/exit branch
//   stays byte-identical. PRESERVE the asymmetry: one-shot (generate.go:376) + workdesc (generate.go:340)
//   → ErrTimeout/124; multiturn/workdesc TURN aborts → ErrRescue/3 (caller maps cause generically);
//   arbiter → graceful null; hook → never-block error. S2's ErrIdleDeadline sentinel satisfies every
//   errors.Is(err, context.DeadlineExceeded) branch unchanged — that's the whole design.

// CRITICAL 4 (one resolution, many Executes): multiturn.Run resolves ONCE (:153) and its 3 Executes
//   share the pair; workdesc resolves once (:71) for its 3. Pass the SAME Deadlines value to each —
//   each Execute call arms its own fresh idle clock internally (FR-A4 is per-INVOCATION, not per-
//   resolution). Do not re-resolve per turn.

// CRITICAL 5 (budget line uses the HARD cap, not the pair): totalMin := int((msgHard *
//   time.Duration(turns)).Minutes()) — idle never scales by turns (each turn gets its own full idle
//   window). Append " (worst case)" INSIDE the format string so the grep-guard in hook/exec_test.go
//   (~L315 note) must be updated to match the new wording.

// CRITICAL 6 (ResolveRoleTimeout survives): after S3 it has zero production callers but roles_test.go
//   still tests it. KEEP it; update only its stale 'KEPT for its 11 existing callers' doc line to
//   'retained as the hard-cap accessor (test-pinned); production callers use ResolveRoleDeadlines'.

// GOTCHA 7 (the idle test's platform nuance — D4): the watcher arms iff Idle>0 && (linux ||
//   ProgressiveStdout). The stub is print-mode (ProgressiveStdout=false until P2), so on darwin/
//   windows the idle is DISARMED. Keep the test's hard cap BOUNDED (e.g. 3s, the cfg.Timeout global
//   with the role overriding only IdleTimeout) so non-Linux completes via the hard cap; assert only
//   errors.Is(err, context.DeadlineExceeded) — true for both kill kinds. On Linux/CI the 150ms idle
//   fires. Do NOT assert ErrIdleDeadline at these sites (that's S2's provider-level test's job).

// GOTCHA 8 (locate by content): S2 lands in parallel and shifts every line number cited here.
//   grep -n 'ResolveRoleTimeout' internal/ → the 8 resolution sites; grep -rn 'provider.Execute(' →
//   the call sites; grep -n 'falling back to multi-turn' → the 2 budget lines.

// GOTCHA 9 (comments cite the fresh-clock structure): the rewritten FR-R7 comments should note WHY
//   one resolution serves many Executes: one Execute per turn/round ⇒ each gets a fresh idle clock
//   (FR-A4) — the structural guarantee that makes per-turn binding automatic.
```

---

## Implementation Blueprint

### The edit shape (every site, mechanical)

```go
// BEFORE (S2 placeholder era):
	// FR-R7/FR25: resolve the message role's timeout so [role.message].timeout / --message-timeout
	// bound the message agent's one-shot generation (and the multi-turn total budget, FR-T5)…
	msgTimeout := config.ResolveRoleTimeout("message", cfg)
	...
	out, stderr, execErr := provider.Execute(ctx, *spec, provider.Deadlines{Hard: msgTimeout}, deps.Verbose)

// AFTER:
	// FR-R7/§9.30: resolve the message role's BOTH deadlines — idle (primary: kill after 60s default
	// of silence) + hard cap (600s default backstop) — via the same per-role ladder as provider/model.
	// Each Execute invocation below binds them individually: a FRESH idle clock per invocation (FR-A4
	// — structurally one Execute per attempt/turn/round). No per-role override ⇒ the 60s/600s globals.
	msgIdle, msgHard := config.ResolveRoleDeadlines("message", cfg)
	...
	out, stderr, execErr := provider.Execute(ctx, *spec,
		provider.Deadlines{Idle: msgIdle, Hard: msgHard}, deps.Verbose)
```

Budget line (generate.go twin shown; hook twin identical modulo ChunkCount + 2-line Fprintf):

```go
	// FR-T5/FR-A4: N+1 INDEPENDENT per-turn deadlines (fresh idle clock each turn) — the total shown
	// is the WORST-CASE ceiling: hard cap × (N+1). Each turn's idle window does not scale with N.
	turns := len(chunkPayload(mtPayload, cfg.MultiTurnChunkTokens)) + 1 // N chunks + 1 final turn
	totalMin := int((msgHard * time.Duration(turns)).Minutes())
	if totalMin < 1 {
		totalMin = 1
	}
	fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns (chunks of ~%d tokens), ~%dm total (worst case)\n",
		turns, cfg.MultiTurnChunkTokens, totalMin)
```

pkg/stagecoach/stagecoach.go:569 (the 13th site — resolve near the loop, mirroring generate.go):

```go
	// FR-R7/§9.30 (library twin of CommitStaged's one-shot loop): the message role's BOTH deadlines,
	// per-invocation binding (FR-A4). Raw cfg.Timeout before; the ladder now applies.
	libIdle, libHard := config.ResolveRoleDeadlines("message", cfg)
	...
	out, _, execErr := provider.Execute(ctx, *spec, provider.Deadlines{Idle: libIdle, Hard: libHard}, deps.Verbose)
```

### The per-family idle test (recipe; one per family)

```go
// Mirrors TestRun_MessageRoleTimeoutBoundsTurn's LARGE-global/SMALL-role idiom, swapped to the idle knob.
// cfg.Timeout = 3s (bounded hard cap: non-Linux print-mode disarms idle and completes via the cap);
// cfg.Roles["message"] = RoleConfig{IdleTimeout: 150 * time.Millisecond}; stub SleepMS 2000, silent.
// Assert ONLY errors.Is(err/cause, context.DeadlineExceeded) — true for both kill kinds (FR-A5).
cfg := config.Defaults()
cfg.Timeout = 3 * time.Second
cfg.Roles = map[string]config.RoleConfig{"message": {IdleTimeout: 150 * time.Millisecond}}
```

### Implementation Tasks (ordered)

```yaml
Task 1: EDIT the 8 resolution sites + their 12 Execute calls (internal/generate, internal/decompose, internal/hook)
  - generate.go: msgTimeout → msgIdle, msgHard; :374 Execute; rewrite the :299-303 comment.
  - multiturn.go: msgTimeout → msgIdle, msgHard (resolve once in Run); :172/183/194 Executes; rewrite
    the :150-153 comment + the :132-135 doc ("Execute shadows ctx" stays true — keyed on Hard).
  - workdesc.go: same at :71 → :82/131/149 + comment.
  - planner.go:69 → :128 (purge the 480s narrative — planner inherits the globals, §9.30).
  - stager.go:98 → :119; message.go:145 → :173; arbiter.go:83 → :101 (comments rewritten).
  - hook/exec.go:167 → :187 + comment (drop the "multi-turn total budget" clause).
  - NO error-branch edits ANYWHERE (the branches quoted in call_sites §2.1-2.9 stay verbatim).

Task 2: EDIT pkg/stagecoach/stagecoach.go:569 — the 13th role site (library twin).
  - Add the resolution line above the loop (config already imported? verify; it imports
    internal/config for ResolveRoleModel — yes, mirror that), thread Deadlines{Idle, Hard}.
  - Its DeadlineExceeded → ErrTimeout branch stays verbatim.

Task 3: EDIT the 2 budget lines (generate.go:465-477; hook/exec.go:~255-262).
  - msgTimeout → msgHard in the totalMin product; append " (worst case)" to BOTH format strings.
  - Update the hook exec_test.go budget-line grep-guard string (~L315) to the new wording.

Task 4: EDIT internal/config/roles.go — ResolveRoleTimeout doc line only (zero production callers now;
  retained as the test-pinned hard-cap accessor). NO deletion, NO behavior change.

Task 5: ADD 5 per-family tests (idle knob reaches Execute):
  - internal/generate/invariants_test.go or a new generate_deadlines_test.go: one-shot — role
    IdleTimeout 150ms / global Timeout 3s / sleeping stub → DeadlineExceeded (mirror the Timeout row).
  - internal/generate/multiturn_test.go: TestRun_MessageRoleIdleBoundsTurn (mirror :246 test).
  - internal/generate/generate_workdesc_test.go: workdesc round variant (mirror :288 test).
  - internal/decompose/planner_test.go: planner variant (mirror :348) — Roles{"planner":{IdleTimeout:100ms}},
    Config.Timeout 3s.
  - internal/hook/exec_test.go: hook variant (mirror TestRun_MessageRoleTimeoutNeverBlock :298) —
    asserts the "timed out" error (never-block).
  - Use the recipe (small idle, bounded 3s global hard cap, silent sleeping stub); assert
    errors.Is(…, context.DeadlineExceeded) only (both kill kinds satisfy it; platform-safe per D4).

Task 6: DOCS (Mode A):
  - docs/how-it-works.md L346: rework "bounded by the **message role's** resolved timeout (/);
    total wall-clock ≈ message-timeout × (N+1)" → both deadlines bind EACH TURN individually (fresh
    idle clock per turn, FR-A4); the progress line reports the worst-case ceiling hard-cap × (N+1).
  - docs/cli.md L452: VERIFY the exit-124 row already reads "Deadline exceeded (idle or hard cap)" —
    it does (landed with P1.M1.T1.S3); edit nothing unless drifted.

Task 7: VERIFY — build, vet, focused suites, full test, lint, grep guards.
```

### Integration Points

```yaml
NO new types / config / flags / exit codes / rescue / CAS / lock changes. NO executor.go / activity*.go /
render.go edits (S1/S2/P2 domains). NO default_action.go edit.

CONSUMES: provider.Deadlines + Execute(ctx, spec, dl, vb) (S2, parallel); config.ResolveRoleDeadlines
  (P1.M1.T1.S1, Complete).
FEEDS: P1.M3.T1.S2-S4 pipeline parity tests (idle-kill e2e through these call sites).
RESOLUTION SEAM: internal/config/roles.go ResolveRoleDeadlines — the single ladder for both deadlines.
SCOPE FENCES: models.go (never); executor/activity/render (S1/S2/P2); docs/packaging.md (out of scope);
  the multiturn ErrRescue-vs-ErrTimeout asymmetry (explicitly preserved per the item contract).
```

---

## Validation Loop

### Level 1: Build & Vet

```bash
go build ./...            # all 13 sites threaded; ResolveRoleTimeout gone from production callers
go vet ./internal/... ./pkg/...
gofmt -l internal/ pkg/ docs/ 2>/dev/null | grep -v '\.md'   # empty
# Expected: clean. If Execute's signature is missing → S2 hasn't landed (prerequisite signal).
```

### Level 2: Unit Tests

```bash
# The threading gate: every touched package's existing suites pass with the new call shape.
go test ./internal/generate/ ./internal/decompose/ ./internal/hook/ ./internal/config/ ./pkg/stagecoach/ -v 2>&1 | tail -20
# Expected: green — the existing per-role TIMEOUT proofs (hard-cap knob) must pass UNCHANGED: they set
#           Roles[role].Timeout which still resolves through ResolveRoleDeadlines → Deadlines.Hard.

# The 5 new idle tests.
go test ./internal/generate/ -run 'Idle' -v ; go test ./internal/decompose/ -run 'Idle' -v ; go test ./internal/hook/ -run 'Idle' -v
# Expected: PASS on Linux (idle watcher armed via rchar); PASS (slower, via the 3s hard cap) on
#           darwin/windows print-mode — both satisfy errors.Is(…, DeadlineExceeded).
```

### Level 3: Full suite + lint

```bash
make test    # green
make lint    # green (ResolveRoleTimeout kept ⇒ exported ⇒ no unused flag; if the linter flags it,
             # the doc-line update + the test callers keep it referenced)
```

### Level 4: Grep guards

```bash
# 1. Zero production ResolveRoleTimeout callers remain.
grep -rn 'ResolveRoleTimeout' --include='*.go' . | grep -v _test.go | grep -v 'roles.go'
# Expected: empty (only the definition in roles.go + its tests).

# 2. All 13 role sites pass resolved Deadlines (Idle non-placeholder).
grep -rn 'provider.Execute(' --include='*.go' internal/ pkg/ cmd/ | grep -v _test.go | grep -v 'Deadlines{Idle:'
# Expected: exactly 1 hit — models.go (the permanent Idle: 0 site).

# 3. Budget lines updated (both twins).
grep -rn 'worst case' internal/generate/generate.go internal/hook/exec.go
# Expected: 2 hits (the Fprintf strings). And no stale shared-budget claim:
grep -rn 'totalMin := int((msgTimeout' internal/   # Expected: empty.

# 4. Error branches untouched (spot-check the invariants).
grep -c 'immediate rescue, NO retry' internal/generate/generate.go internal/decompose/message.go
grep -c 'graceful null' internal/decompose/arbiter.go
grep -c 'never-block' internal/hook/exec.go        # Expected: all ≥1, branches verbatim.

# 5. Scope: no executor/activity/render/models/default_action/exitcode changes.
git diff --name-only | grep -E 'executor|activity|render\.go|models\.go|default_action|exitcode'
# Expected: empty.

# 6. Docs.
grep -n 'worst case\|fresh idle clock\|idle deadline' docs/how-it-works.md   # ≥1 hit each of the reworked prose
grep -n 'Deadline exceeded (idle or hard cap)' docs/cli.md                   # 1 hit (verify-only)
```

---

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + `go vet` clean; `gofmt -l` empty on touched files
- [ ] `make test` + `make lint` green
- [ ] Existing hard-cap per-role tests pass unchanged (Timeout knob flows through the new ladder)

### Feature Validation
- [ ] 13 role sites pass `Deadlines{Idle: xIdle, Hard: xHard}` from ResolveRoleDeadlines
- [ ] Zero production `ResolveRoleTimeout` callers; function retained + doc updated
- [ ] Both budget lines: `msgHard × turns` + " (worst case)"; hook grep-guard updated
- [ ] 5 per-family idle tests pass (Linux fast-kill; non-Linux bounded via 3s hard cap)
- [ ] Stale FR-R7 comments rewritten (both deadlines + FR-A4); planner 480s narrative purged
- [ ] how-it-works.md reworked; cli.md verified

### Scope-Boundary Validation
- [ ] NO error-branch / exit-code / rescue / retry / CAS / lock edits (asymmetry preserved verbatim)
- [ ] models.go, executor.go, activity*.go, render.go, default_action.go, exitcode untouched
- [ ] No new deps; no new flags/config (P1.M1 owns that surface)

---

## Anti-Patterns to Avoid

- ❌ Don't "fix" the multiturn ErrRescue/3 asymmetry. The contract says DO NOT change it — FR-A5 keeps
  every existing mapping byte-identical; only the verbose cause line differs.
- ❌ Don't re-resolve per turn. One resolution per Run/function serves all its Executes; the fresh idle
  clock is per-Execute (FR-A4 structural), not per-resolution.
- ❌ Don't scale the idle deadline by turns in the budget line. Only the HARD cap × (N+1) is a
  meaningful worst case; each turn gets its own full idle window.
- ❌ Don't delete ResolveRoleTimeout. Its tests pin the accessor; update only the stale doc line.
- ❌ Don't touch models.go (`Idle: 0` is permanent — a probe, not a role call) or pkg/stagecoach's
   ERROR branch while threading :569.
- ❌ Don't assert `ErrIdleDeadline` in the per-family tests — the call-site contract is
  `context.DeadlineExceeded` (both kill kinds); platform-dependent assertions belong to S2's
  provider-level tests.
- ❌ Don't edit executor.go / activity*.go / render.go / config loaders — S2/S1/P2/P1.M1 domains.
- ❌ Don't anchor to line numbers (S2 lands in parallel). Locate via
  `grep -n 'ResolveRoleTimeout' internal/` and `grep -rn 'provider.Execute('`.
- ❌ Don't forget the hook exec_test.go budget-line grep-guard when rewording its format string.

---

## Confidence Score: 9/10

Grounded in grep-verified ground truth: the exact 13-site list (reconciling S2's count via the
pkg/stagecoach.go:569 library twin), verbatim before/after shapes, the verbatim budget lines (both
twins), the resolver API (read from source), the FR-A5 no-branch-edit rule with the explicit
asymmetry-preservation clause, the per-family test recipe with the D4 platform nuance worked out
(idle arms on Linux; bounded 3s hard cap keeps non-Linux green), and the docs state verified live
(cli.md already reworded; how-it-works.md stale). Residual (not 10): S2 is Implementing in parallel —
if it lands with any Deadlines-shape deviation, the call shapes here need mechanical adjustment (the
PRP pins the expected shape from S2's contract); and the 5 new tests carry mild timing sensitivity
(150ms idle vs 2s sleep = 13× margin — safe).
