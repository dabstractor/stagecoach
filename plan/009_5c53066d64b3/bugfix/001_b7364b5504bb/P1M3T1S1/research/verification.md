# Verification — P1.M3.T1.S1 (bind `resolved` + hoist `payload` in hook.Run)

> Live-tree confirmation that the contract's 3 edits are exact, safe, and conflict-free. Numbered for
> cross-reference from the PRP. (2026-07-04)

## §1 — The 3 edits match the current code EXACTLY (line-number-accurate)

`internal/hook/exec.go`, the `Run` generate→parse→dedupe loop (research_hook_exec.md §2):

| Edit | Anchor (current) | Contract / resolution_strategy.md | Verified |
|---|---|---|---|
| 1 — bind `resolved` | L151 `retryInstr := *deps.Manifest.Resolve().RetryInstruction` | replace with `resolved := deps.Manifest.Resolve()` + `retryInstr := *resolved.RetryInstruction` | ✅ line & text match |
| 2 — hoist `payload` | L154 `var rejected []string` / L155 `var parseFail bool` | add `var payload string // hoisted: survives the loop for the FR-T1 gate` AFTER L155 | ✅ anchors match |
| 3 — `:=` → `=` | L158 `payload := prompt.BuildUserPayload(diff, cfg.Context, rejected)` | change `:=` to `=` (payload now function-scoped) | ✅ line & text match |

(Loop body context confirmed: L157 `for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++ {`;
L160 `payload = retryInstr + "\n\n" + payload`; L163 `deps.Manifest.Render(msgModel, sysPrompt, payload, msgReasoning)`.)

## §2 — ZERO shadowing risk (the load-bearing safety check)

`grep -n "payload\|resolved" internal/hook/exec.go` returns ONLY:
```
158:  payload := prompt.BuildUserPayload(diff, cfg.Context, rejected)
160:    payload = retryInstr + "\n\n" + payload
163:  spec, rerr := deps.Manifest.Render(msgModel, sysPrompt, payload, msgReasoning)
```
- `payload` appears NOWHERE else in `Run` ⇒ hoisting it to function scope (Edit 2) + switching L158 to `=`
  (Edit 3) turns the loop's `payload` from a per-iteration declaration into an assignment to the hoisted
  var. L160 (assign) and L163 (read) behave identically either way. No shadow, no redeclaration.
- `resolved` is NOT used anywhere in the file ⇒ the new `resolved :=` (Edit 1) cannot clash.
- `go vet ./internal/hook/` is the deterministic confirmation (the contract requires it).

## §3 — `Manifest.Resolve()` returns the type the gate needs

`internal/provider/manifest.go:150` `func (m Manifest) Resolve() Manifest` — returns a `Manifest` with:
- `RetryInstruction *string` (L88; Resolve guarantees non-nil → default at L193) ⇒ `*resolved.RetryInstruction` is safe (Edit 1).
- `SessionMode *string` (L66; Resolve guarantees non-nil → `strPtr("")` at L177-178) ⇒ `resolved.SessionMode` is available for the FR-T1 gate (P1.M3.T1.S2 reads it to decide multi-turn eligibility). **This is the reason `resolved` must be bound** (the inline `*deps.Manifest.Resolve().RetryInstruction` discarded the manifest, so SessionMode was unreachable).

## §4 — No conflict with the parallel work item (P1.M2.T1.S2)

P1.M2.T1.S2 (dry-run runPipeline gate) is **pkg/stagehand ONLY**. Its PRP explicitly excludes
`internal/hook/exec.go` as P1.M3's scope — verified at lines 64, 242 ("NOT this task ← NO edit"),
297-298 ("do NOT edit ... internal/hook/exec.go ... This task is pkg/stagehand ONLY"), 476, 606. This task
(P1.M3.T1.S1) edits `internal/hook/exec.go` ONLY. **Zero file overlap ⇒ no merge conflict.** (P1.M2.T1.S1,
already Complete, did the analogous `payload` hoist in `runPipeline` — the precedent for this hook.Run hoist.)

## §5 — Pure refactor; no behavioral change

The 3 edits change ONLY variable binding + scoping:
- Edit 1: `resolved` is a new name for the already-computed `deps.Manifest.Resolve()` result (it was
  inline-discarded; now bound). `retryInstr` is byte-identical.
- Edit 2/3: `payload` moves from loop scope to function scope; its value at every loop iteration is
  unchanged (same `BuildUserPayload` call, same assignment semantics).
No control-flow, Render/Execute/ParseOutput call, or return changes. ⇒ existing `go test ./internal/hook/...`
stays green (the contract requires confirming this). The hoisted `payload` + bound `resolved` become
readable AFTER the loop by the FR-T1 gate (P1.M3.T1.S2) — that is the sole purpose of this refactor.
