# System Context — Multi-turn generation fallback (PRD §9.24, FR-T1–T12)

Synthesized from three parallel codebase-research passes (see sibling `research-*.md` files) plus
direct reads of `manifest.go`, `render.go`, `generate.go`, and a live `pi` FR-T9 verification run.
This is the authoritative handoff for the downstream PRP/implementation agents. All line numbers
verified against the current tree (2026-07-05).

## 1. What this delta is (and is not)

**In scope:** a *lossless* multi-turn generation fallback for the **message role only** (single-commit
path, §13.1–§13.5). When a one-shot generation of a large diff fails (provider per-request
unreliability below the advertised context window), re-deliver the FULL diff across request-sized
provider session turns so a single commit message can still be produced — without truncation and
without decomposing into multiple commits.

**Out of scope (explicit, FR-T10):** planner/stager/arbiter roles; decompose path (§13.6); any change
to commit/rescue-message/CAS/run-lock/signal logic; lossy map-reduce chunking (permanently rejected).
`token_limit` (FR3d) continues to govern ONLY the one-shot path (FR-T12).

**Sizing:** medium feature. New generation path (`multiturn.go`) + one manifest field + two config
knobs + a render variant + tests + docs. One phase, one milestone, four implementing tasks + a Mode B
doc sync.

## 2. The four insertion points (validated against source)

### R1 — Provider surface: `session_mode` field + pi value + multi-turn render variant

| Artifact | Location | Change |
|---|---|---|
| `Manifest` struct | `internal/provider/manifest.go:60–64` | Add `SessionMode *string` between `ProviderFlag` and `BareFlags` (new `// --- session continuation (multi-turn fallback, §9.24) ---` block, mirroring PRD §12.1 TOML ordering at `PRD.md:726–733`). |
| `Resolve()` | `manifest.go:110–164` | Add `if out.SessionMode == nil { out.SessionMode = strPtr("") }` (mirrors `ProviderFlag`/`PrintFlag`). |
| `Validate()` | `manifest.go:88–106` | Optionally reject values other than `""`/`"append"` (enum enforcement). |
| `MergeManifest` | `internal/provider/merge.go:28` (regime-1 scalars, ~:80) | Add `if override.SessionMode != nil { out.SessionMode = override.SessionMode }`. **A plain `*string` merges identically to every other scalar** — confirmed. |
| `builtinPi()` | `internal/provider/builtin.go:30–96` | Add `SessionMode: strPtr("append")` with inline `// VERIFIED 2026-07-05 ... FR-T9` comment. `BareFlags` (line 60) contains `"--no-session"` — the flag the render variant must DROP. All other builtins ship absent (Resolve→`""`). |
| `providers/pi.toml` | line ~34 | Add `session_mode = "append"` with the matching VERIFIED comment. |
| Render variant | `internal/provider/render.go:89` | **Preferred: a sibling `RenderMultiTurn` method**, NOT a widened `Render` signature (24+ call sites across generate/hook/stagehand/decompose/stubtest would break). It reuses the §12.2 token pipeline but swaps the bare-flags block: filter out `"--no-session"`, append `"--session-id", sessionID`, keep `-p`, emit the system-prompt flag on turn 1 ONLY. Error if `*r.SessionMode != "append"` (capability gate, FR-T8/T9). |

**CRITICAL — no existing "drop a specific bare_flag" mechanism.** The renderer treats `BareFlags` as
an opaque verbatim slice (`args = append(args, r.BareFlags...)`, render.go:135). The only precedent
for flag-set *substitution* is the bare/tooled mode ternary. The multi-turn variant must introduce
flag-filtering by exact token (`"--no-session"`, pi-only-shipped value). The PRD does not name a
`SessionKillFlag` field; hardcoding the `"--no-session"` filter is the simplest correct path.

### R2 — Config surface: `multi_turn_fallback` + `multi_turn_chunk_tokens`

| Artifact | Location | Change |
|---|---|---|
| `Config` struct | `internal/config/config.go:63` (after `MaxDuplicateRetries` :83) | Add `MultiTurnFallback bool` (TOML `multi_turn_fallback`) + `MultiTurnChunkTokens int` (TOML `multi_turn_chunk_tokens`). Flat resolved struct; plain types. |
| `Defaults()` | `config.go:161` (after :176) | `MultiTurnFallback: true`, `MultiTurnChunkTokens: 32000`. |
| `fileGeneration` struct | `internal/config/file.go:44` (after :54) | Same two fields with TOML tags. |
| `materialize` (the delta's "loadTOML overlay") | `file.go:193` (after :231) | `MultiTurnChunkTokens`: `if g.X != 0 { c.X = g.X }` (int template, mirrors `TokenLimit`/`MaxCommits`). `MultiTurnFallback`: `if g.X { c.X = true }` (bool template, mirrors `AutoStageAll`/`Push`). |
| `overlay` (the delta's "merge") | `file.go:294` (after :345) | Same two guards. |

**⚠️ Bool-sentinel limitation (accepted):** `MultiTurnFallback` defaults `true` and the existing bool
pattern only propagates `true` from a file. A user setting `multi_turn_fallback = false` in a file
will be silently ignored — the same documented "v1 limitation" `AutoStageAll` carries. The delta
explicitly accepts "follow whatever pattern auto_stage_all uses." Surface in `docs/configuration.md`.
(Alternative `*bool` like `DiffContext` widens scope — not recommended.)

**No CLI flags, no env vars** for these two keys (delta: "No new ... CLI flags"). They are config-file-only.

### R3 — Generate core: chunking + N+1 turns + trigger gate + failure→rescue + progress + verbose

| Artifact | Location | Change |
|---|---|---|
| Trigger gate | `internal/generate/generate.go:288` (the `if !success { return … &RescueError }` boundary) | Insert FR-T1 (a–d) gate BETWEEN the loop close (`:287`) and `if !success` (`:288`). All four conditions: (a) loop exhausted on empty/unparseable output (true at that point); (b) `EstimateTokens(payload) > cfg.MultiTurnChunkTokens`; (c) `cfg.MultiTurnFallback`; (d) `deps.Manifest.Resolve().SessionMode == "append"`. Any false → fall through to the EXISTING rescue (byte-identical). |
| Chunker + protocol | NEW `internal/generate/multiturn.go` | `N = ceil(EstimateTokens(payload) / cfg.MultiTurnChunkTokens)`; split into N consecutive chunks ≤ budget; anchor each boundary FORWARD to the next newline (no fractured diff line); prefix each `"PART i/N:"` OUTSIDE the budget. |
| N+1 turn protocol | `multiturn.go` | Mint session id `stagehand-<run-uuid>`. Turn 1: sys prompt (via flag) + priming preamble (verbatim w/ N) + chunk 1. Turns 2..N: `"PART i/N:"` + chunk i. Turn N+1: `"Now write the commit message…Output ONLY the message."`. Discard intermediate `ok`. Final stdout → existing `ParseOutput` + dedupe unchanged. |
| Per-turn execution | `provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)` UNCHANGED | One `Execute` per turn; same `cfg.Timeout` per turn; total budget `timeout × (N+1)`. |
| Failure handling (FR-T7) | `generate.go:288` | Any turn non-zero-exit (not timeout) / turn timeout / final-turn parse|dedupe failure → `return Result{}, &RescueError{Kind: ErrRescue, TreeSHA, ParentSHA, Candidate, Cause}` byte-identical to one-shot. Use `ErrRescue` (exit 3), NOT `ErrTimeout` (124) — `ErrTimeout` is reserved for the one-shot kill. |
| Progress (FR-T5) | new one-liner at fallback time | `"falling back to multi-turn: N+1 turns, ~Mm total"` to stderr. `CommitStaged` does NOT print progress itself today (only `Deps.Progress` hook callback) — emit via `deps.Verbose` is insufficient (verbose is opt-in); a new stderr progress write or a new `Deps` callback is needed. |
| Verbose (FR-T11) | FREE via `provider.Execute` | `executor.go:71–92` already emits `VerboseCommand`/`VerbosePayload`/`VerboseRawOutput`/`VerboseStderr` per call. A multi-turn loop calling `Execute` per turn inherits per-turn payload-size + raw stdout/stderr logging automatically. Add a trigger/fallback verbose line at the gate. |

**⚠️ CRITICAL — `payload` is loop-scoped (`generate.go:228`, `:=` inside the loop).** At the rescue
boundary (`:288`) `payload` is NOT in scope; `sysPrompt` (`:170`) and `diff` (`:175`) ARE. FR-T2 says
"reuse the existing payload-construction; do not recompute." Two options: (1) hoist
`var payload string` before the loop and assign inside (last instance survives — cleaner); or
(2) call `prompt.BuildUserPayload(diff, cfg.Context, nil)` once at the gate (one deterministic call,
same `diff` input → identical bytes modulo the irrelevant `rejected` list). **Recommend (1)** (hoist)
for the literal "do not recompute" reading. The multi-turn payload is the UNTRUNCATED payload —
`token_limit` does NOT apply (FR-T12).

### R4 — Tests (stub + integration)

The `internal/stubtest` stub-agent (`cmd/stubagent/main.go`) is env-var-driven and **already varies
output per-invocation** via `NewScript(t, bin, []string{"ok","ok","feat: msg"})` (call-order indexed,
clamps to last). `STAGEHAND_STUB_ARGSFILE` captures the rendered argv (NUL-joined) so a test can
ASSERT `--session-id <value>` is present and `--no-session` is absent on every turn. **This is
sufficient** — the stub need not echo prior-turn content; the orchestrator's prompt builder
re-sends it. `TestCommitStaged_DedupeRetryThenSuccess` (`generate_test.go:118`) is the canonical
per-turn-output integration template.

## 3. Shared utilities (reuse, do NOT re-create)

- **`git.EstimateTokens(s string) int`** (`internal/git/tokens.go:25`) — `ceil(runeCount / 4)` (rune-
  based, NOT byte — multi-byte UTF-8/CJK does not over-count). The SINGLE token estimator; use for
  both chunk sizing (FR-T3) and the gate (FR-T1b). Also `EstimateTokensBytes([]byte) int` (:32).
- **`provider.ParseOutput(raw, m)`** (`parse.go:41`) — parse the FINAL turn's stdout only; unchanged.
- **`provider.Execute(ctx, spec, timeout, vb)`** (`executor.go:44`) — one call per turn; returns
  `(stdout, stderr, err)`. Already does per-turn verbose.
- **`*RescueError`** (`generate.go:82–99`) — multi-turn failure returns it byte-identically to one-shot.
- **Config patterns** (`TokenLimit`/`MaxCommits` int `!= 0`; `AutoStageAll`/`Push` bool only-true) are
  the exact template for the two new keys.
- **Render's variadic `mode`** pattern (how `RenderTooled` was added without breaking callers) is the
  template for the multi-turn render variant — but a sibling `RenderMultiTurn` method is preferred
  over a 7th positional param given the session-id threading.

## 4. Data flow (where multi-turn inserts)

```
CommitStaged (generate.go)
  sysPrompt built ONCE (:170)  ──┐
  diff captured ONCE (:175)     ──┤  both in scope at :288
  WriteTree snapshot (:185)     ──┤
  recent subjects (:...)         │
  one-shot retry loop (:226–287) │  payload := BuildUserPayload(diff, cfg.Context, rejected)  ← LOOP-SCOPED
  │
  └─ !success → :288  ◄── FR-T1 TRIGGER GATE inserts here
        │ (a) exhausted  (b) EstimateTokens(payload) > MultiTurnChunkTokens
        │ (c) cfg.MultiTurnFallback  (d) manifest.SessionMode == "append"
        │
        ├─ all hold ─▶ multiturn.Run(payload, sysPrompt, msgModel, msgReasoning, manifest, cfg, deps)
        │                 │ chunk: N = ceil(EstimateTokens(payload)/MultiTurnChunkTokens), newline-anchored
        │                 │ for turn in 1..N+1:
        │                 │   spec = manifest.RenderMultiTurn(msgModel, sys?, chunkPrompt, msgReasoning, sessionID, turn)
        │                 │          (drops --no-session, adds --session-id; sys flag on turn 1 only)
        │                 │   out,_,err = provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)
        │                 │   on err/timeout → return false (→ rescue)
        │                 │ final out → ParseOutput → dedupe → return (msg, ok)
        │                 │
        │                 ├─ ok & not dup → SUCCESS (msg assigned, success=true)
        │                 └─ else → fall through
        └─ → existing &RescueError{Kind: ErrRescue, ...}  (:289, UNCHANGED)
```

## 5. Risks / decisions for the implementer

1. **FR-T9 is DONE** (see `fr-t9-verification.md`) — pi `"append"` ships verified. No longer blocking.
2. **`payload` loop-scope** — hoist to function scope (recommended) or re-call `BuildUserPayload` once.
3. **`MultiTurnFallback` bool-sentinel** — `false` in a file is silently ignored (accepted limitation).
4. **No `--no-session` drop mechanism exists** — the render variant introduces one (filter by exact token).
5. **Progress emission** — `CommitStaged` prints no progress today; FR-T5 needs a new stderr write or `Deps` callback.
6. **Per-turn timeout compounds** — `120s × (N+1)` can be many minutes; the FR-T5 progress line is mandatory.
