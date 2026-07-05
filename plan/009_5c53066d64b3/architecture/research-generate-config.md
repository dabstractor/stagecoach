# Research: Generate core + config surface for multi-turn fallback (PRD §9.24, FR-T1–T12)

Scout deliverable for R2 (config keys) + R3 (generate trigger gate / chunking). Validates every
line reference in `plan/009_5c53066d64b3/delta_prd.md`. **No files modified** — research only.

## Delta line-reference validation (verdict)

| Delta claim | Verdict |
|---|---|
| One-shot exhaustion → `&RescueError{…}` "around line 288" | ✅ EXACT: `internal/generate/generate.go:288` |
| `CommitStaged` retry loop at "line 226" | ✅ EXACT: `internal/generate/generate.go:226` (`for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++`) |
| `git.EstimateTokens` at "tokens.go:25" | ✅ EXACT: `internal/git/tokens.go:25` (`func EstimateTokens(s string) int`) |
| "the `merge` function" (R2) | ⚠️ **MISNAMED.** There is NO `merge` in `internal/config/`. The field-by-field merge is `overlay(dst, src *Config)` at `file.go:294`. (`materialize` at `file.go:193` is the file→Config copy.) The delta's "loadTOML overlay + merge" maps to `materialize` + `overlay`. |
| New keys "next to TokenLimit / MaxDuplicateRetries" in resolved Config | ✅ config.go:81 (`TokenLimit`) + :83 (`MaxDuplicateRetries`) |
| `Manifest.Render` at "render.go:89" | ✅ EXACT (`internal/provider/render.go:89`) — R1 scope, cross-checked |
| `SessionMode == "append"` (FR-T1d) | ⚠️ **`SessionMode` does NOT exist yet** anywhere in `internal/`. grep for `SessionMode`/`session_mode` returns nothing. This is R1's deliverable; the R3 trigger gate (FR-T1d) depends on R1 landing first. |

---

## 1. `internal/generate/generate.go` — `CommitStaged` orchestrator

### 1a. Rescue boundary — the trigger-gate insertion point (FR-T1)

The one-shot retry loop is `internal/generate/generate.go:226–284`. Exhaustion falls through to a
**single** rescue return at **`generate.go:288`**. Exact context (lines 284–292):

```go
284		msg = m
285		success = true
286		break // SUCCESS — accept the message
287	}
288	if !success {
289		return Result{}, &RescueError{
290			Kind: ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA,
291			Candidate: candidate, Cause: lastCause,
292		}
293	}
```

**Insertion point for FR-T1 (a–d):** the multi-turn trigger gate goes between the `}` closing the
loop (line 287) and the `if !success` check (line 288). I.e. the gate replaces the bare
`if !success { return … }` with:

```
if !success {
    if /* FR-T1 a–d all hold */ { /* run multi-turn; on failure fall through */ }
    return Result{}, &RescueError{ Kind: ErrRescue, TreeSHA: treeSHA, … }   // line 289 unchanged
}
```

All four FR-T1 conditions evaluate to the existing `!success` state, so the literal `return` line
content is preserved byte-for-byte on the fall-through path (FR-T7 "existing rescue unchanged").

### 1b. ⚠️ CRITICAL — `payload` is loop-scoped (the "do not recompute" tension)

The delta R3 says *"Reuse the existing payload-construction in CommitStaged; do not recompute."*
**This is in tension with the current code structure.** The user-payload is built INSIDE the loop
at **`generate.go:228`** with `:=` (loop-scoped):

```go
226	for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++ {
227		// Build user payload each attempt (rejection list / retry_instruction change).
228		payload := prompt.BuildUserPayload(diff, cfg.Context, rejected)
229		if parseFail {
230			payload = retryInstr + "\n\n" + payload // FR29 corrective preamble
231		}
...
```

At the rescue boundary (line 288), the **in-scope variables** are:

| Var | Declared | Scope at :288 | Notes |
|---|---|---|---|
| `sysPrompt` | `generate.go:170` (`sysPrompt, err := buildSystemPrompt(...)`) | ✅ in scope | Built ONCE before the loop. **This IS the system prompt the multi-turn turn-1 needs.** |
| `diff` | `generate.go:175` (StagedDiff result) | ✅ in scope | Raw diff body — the multi-turn capture-once payload source |
| `cfg`, `treeSHA`, `parentSHA`, `isUnborn`, `rejected`, `candidate`, `parseFail`, `lastCause`, `msg`, `success` | various | ✅ in scope | |
| **`payload`** | `generate.go:228` (`:=` inside loop) | ❌ **NOT in scope** | Loop-local; cannot be reused at :288 |
| `spec` | `generate.go:237` (`:=` inside loop) | ❌ not in scope | |

**Implication for R3:** to honor "do not recompute," the implementation must either:
1. **Hoist** a function-scoped `var payload string` before the loop and assign it inside (so the last
   loop-built instance survives to :288), OR
2. **Capture-once at :288**: call `prompt.BuildUserPayload(diff, cfg.Context, nil)` once after the
   loop. This is one deterministic call with the same `diff` input → produces the same payload bytes
   (modulo the `rejected` list, which is irrelevant for a fresh multi-turn priming). This is
   arguably a *recomputation* (tension with the delta), but produces identical content.

Option (1) is the cleaner literal reading of "do not recompute." Flag for the implementer.

### 1c. `EstimateTokens` usage in CommitStaged

`git.EstimateTokens` is currently called **only indirectly** at `generate.go:174`:

```go
170	sysPrompt, err := buildSystemPrompt(ctx, deps.Git, cfg, isUnborn)
...
174	reserve := prompt.MessageReserveTokens(sysPrompt, cfg.MaxDuplicateRetries, cfg.SubjectTargetChars, cfg.Context, git.EstimateTokens)
```

It is passed as a **function value** to `prompt.MessageReserveTokens` (the reserve seam). It is NOT
called directly on the diff/payload in `CommitStaged` today. The R3 chunk-sizing helper
(`git.EstimateTokens(payload)` for `N = ceil(...)`) and the FR-T1b gate
(`EstimateTokens(payload) > cfg.MultiTurnChunkTokens`) would be the **first direct `EstimateTokens`
calls on the payload** in this file.

### 1d. Other in-scope seam: `RetryInstruction`, `msgModel`, `msgReasoning`

```go
222	resolved := deps.Manifest.Resolve()
223	retryInstr := *resolved.RetryInstruction // resolved default: "Output ONLY the commit message…"
...
224	_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)
```

All function-scoped, available at :288. The multi-turn final-turn instruction ("Now write the
commit message…") is a NEW literal (FR-T4), not `retryInstr`.

---

## 2. `internal/config/config.go` — resolved `Config` struct + `Defaults()`

### 2a. The generation block in `Config` (where the two new keys go)

`type Config struct` is at **`config.go:63`**. The generation scalars live at lines 78–96. The two
anchor fields for placement:

```go
81	TokenLimit          int  `toml:"token_limit"`           // FR3d holistic token cap (0 = unset ⇒ legacy caps); consumed by S2/S4
82	DiffContext         *int `toml:"diff_context"`          // FR3f reduced context ...
83	MaxDuplicateRetries int  `toml:"max_duplicate_retries"` // re-gen attempts on duplicate subject
84	SubjectTargetChars  int  `toml:"subject_target_chars"`  // target subject length for truncation
```

**R2 placement:** add the two new fields immediately after line 83 (next to
`TokenLimit`/`MaxDuplicateRetries`), per the delta:

```go
MaxDuplicateRetries int  `toml:"max_duplicate_retries"` // re-gen attempts on duplicate subject
MultiTurnFallback   bool `toml:"multi_turn_fallback"`   // §9.24 FR-T1c: lossless multi-turn fallback on one-shot exhaustion (default true)
MultiTurnChunkTokens int `toml:"multi_turn_chunk_tokens"` // §9.24 FR-T3: per-turn chunk budget in tokens (default 32000)
SubjectTargetChars  int  `toml:"subject_target_chars"`  // target subject length for truncation
```

The struct is **flat + plain-typed + resolved** (per its docstring at config.go:47–61); no nested
sub-struct, no pointers for these scalars. The TOML tags are snake_case leaf names (the resolved
Config is never directly decoded from §16.2 — `fileConfig` is — but the tags are kept for
documentation parity, see config.go:53–54).

### 2b. `Defaults()` — where to add the two defaults

`func Defaults() Config` is at **`config.go:161`**. The relevant block:

```go
174		TokenLimit:          0,         // FR3d: 0 = unset ⇒ legacy per-section caps ...
175		DiffContext:         intPtr(1), // FR3f: -U1 default ...
176		MaxDuplicateRetries: 3,
177		SubjectTargetChars:  50,
```

**R2 additions** (per delta: `multi_turn_fallback` default `true`, `multi_turn_chunk_tokens` default `32000`):

```go
MaxDuplicateRetries:   3,
MultiTurnFallback:     true,     // §9.24 FR-T1c default (feature on by default)
MultiTurnChunkTokens:  32000,    // §9.24 FR-T3 default per-turn budget
SubjectTargetChars:    50,
```

---

## 3. `internal/config/file.go` — file struct + `materialize` + `overlay` (the "merge")

### 3a. `fileGeneration` struct — where the TOML tags live

`fileGeneration` is at **`file.go:44`** (the FILE decode twin; only `file.go` decodes into it).
The anchor fields:

```go
52	TokenLimit          int      `toml:"token_limit"`  // FR3d — plumbed in S2 (materialize/overlay)
53	DiffContext         *int     `toml:"diff_context"` // FR3f — *int (0-vs-unset); nil ⇒ user omitted ...
54	MaxDuplicateRetries int      `toml:"max_duplicate_retries"`
55	SubjectTargetChars  int      `toml:"subject_target_chars"`
```

**R2 additions** after line 54:

```go
MaxDuplicateRetries  int  `toml:"max_duplicate_retries"`
MultiTurnFallback    bool `toml:"multi_turn_fallback"`     // §9.24 FR-T1c (default true)
MultiTurnChunkTokens int  `toml:"multi_turn_chunk_tokens"` // §9.24 FR-T3 (default 32000)
SubjectTargetChars   int  `toml:"subject_target_chars"`
```

### 3b. `materialize` — file→Config copy (the delta's "loadTOML overlay")

`func materialize(fc *fileConfig, timeout time.Duration) *Config` is at **`file.go:193`**. The
int-guard template and the bool-guard template the delta references:

**Int guard template** (the `>0`/`!=0` pattern — for `MultiTurnChunkTokens`):

```go
218	// FR3d: TokenLimit is a plain int; 0 = unset ⇒ legacy caps (no meaningful "explicit 0").
219	if g.TokenLimit != 0 {
220		c.TokenLimit = g.TokenLimit
221	}
...
229	if g.MaxDuplicateRetries != 0 {
230		c.MaxDuplicateRetries = g.MaxDuplicateRetries
231	}
```

For a positive-default int like `MultiTurnChunkTokens` (default 32000), `!= 0` and `> 0` are
**equivalent for all valid user input** (a file value of `0` is treated as "unset" → keeps the
32000 default — acceptable, matches every other int field). The delta's "`> 0` guard" wording is
satisfied by `!= 0`; either is fine.

**Bool guard template** (the unset-sentinel problem — for `MultiTurnFallback`):

```go
213	if d.AutoStageAll {
214		c.AutoStageAll = true // v1 limitation: cannot set false via file
215	}
...
274	// §9.22 FR-P1 — push from file (mirrors AutoStageAll/Verbose bool pattern).
275	if g.Push {
276		c.Push = true
277	}
```

> **⚠️ DESIGN TENSION (flag for parent):** `MultiTurnFallback` defaults to **`true`** in
> `Defaults()`. With the existing bool pattern (`if g.X { c.X = true }` — only `true` propagates
> from a file), **a user can NEVER disable multi-turn fallback via the config file.** This is the
> same documented "v1 limitation" that `AutoStageAll` (also default-true) carries. The delta
> explicitly accepts "follow whatever pattern auto_stage_all uses" — so the limitation is inherited
> intentionally. But because `multi_turn_fallback = false` in a file would be silently ignored,
> consider: (a) accept the limitation (matches delta); (b) make it `*bool` like `DiffContext` is
> `*int` (nil = unset → default true; explicit false honored) — but this widens scope and diverges
> from the delta's "follow auto_stage_all" instruction. **Recommend (a)** per the delta, and surface
> the limitation in `docs/configuration.md` (R2 Mode A doc).

### 3c. `overlay` — field-by-field merge (the delta's "merge")

`func overlay(dst, src *Config)` is at **`file.go:294`**. There is **NO `merge` function** — the
delta's "merge" = `overlay`. Templates:

**Int overlay** (TokenLimit, `!= 0`):

```go
331	// FR3d: TokenLimit plain int + != 0 (0 IS its unset sentinel ...).
332	if src.TokenLimit != 0 {
333		dst.TokenLimit = src.TokenLimit
334	}
...
343	if src.MaxDuplicateRetries != 0 {
344		dst.MaxDuplicateRetries = src.MaxDuplicateRetries
345	}
```

**Bool overlay** (AutoStageAll, only-true-propagates):

```go
318	if src.AutoStageAll {
319		dst.AutoStageAll = true
320	}
...
326	// §9.22 FR-P1 — push
327	if src.Push {
328		dst.Push = true
329	}
```

**R2 additions** to `overlay` (after the MaxDuplicateRetries block, ~line 345):

```go
// §9.24 FR-T1c — multi_turn_fallback (bool; only-true-propagates, mirrors AutoStageAll/Push —
// cannot disable via file, same v1 limitation).
if src.MultiTurnFallback {
    dst.MultiTurnFallback = true
}
// §9.24 FR-T3 — multi_turn_chunk_tokens (int; != 0, mirrors TokenLimit/MaxCommits).
if src.MultiTurnChunkTokens != 0 {
    dst.MultiTurnChunkTokens = src.MultiTurnChunkTokens
}
```

`overlay` is called by the precedence resolver (load.go, global→repo→git-config) — adding the two
fields here is sufficient to thread them through all file-based layers. Env/flag layers (R2 does NOT
add CLI flags — the delta says "No new ... CLI flags") are out of scope.

---

## 4. `internal/git/tokens.go` — `EstimateTokens`

**`internal/git/tokens.go:25`** — CONFIRMED EXACT:

```go
25	func EstimateTokens(s string) int {
26		return ceilDiv(utf8.RuneCountInString(s), 4)
27	}
```

Signature `EstimateTokens(s string) int` ✅. Formula is **`ceil(runeCount / 4)`** (rune-based, NOT
byte-based — multi-byte UTF-8/CJK does not over-count). The delta's "chars/4" wording is
approximate; the precise contract is runes/4 (documented at tokens.go:18–24). Also available:
`EstimateTokensBytes(b []byte) int` at `tokens.go:32` (same formula, `[]byte` form — useful if the
chunker holds a byte buffer). `ceilDiv` at tokens.go:37.

Use this SINGLE estimator for both FR-T3 (chunk sizing, `N = ceil(payload_tokens / chunk_tokens)`)
and FR-T1b (gate, `EstimateTokens(payload) > cfg.MultiTurnChunkTokens`). Do not introduce a second
estimator (delta R3 anchors, line 87 of delta_prd.md).

---

## 5. `RescueError` type — location & multi-turn failure contract (FR-T7)

⚠️ **The `RescueError` type lives in `internal/generate/generate.go` (lines 82–99), NOT in
`rescue.go`.** `internal/generate/rescue.go` is **purely the `FormatRescue` / `FormatRescueMulti`
string assemblers** (PRD §18.3 message text) — it contains NO error type and NO triggering logic.

### 5a. The error sentinels and type (all in `generate.go`)

```go
60	var ErrTimeout = errors.New("stagehand: generation timed out")
...
65	var ErrRescue = errors.New("stagehand: commit generation failed after retries")
...
82	type RescueError struct {
83		Kind      error  // ErrTimeout or ErrRescue — Unwrap() returns this (enables errors.Is)
84		TreeSHA   string // the frozen snapshot (always non-empty — rescue fires only after WriteTree)
85		ParentSHA string // "" on a root commit (FormatRescue omits -p)
86		Candidate string // the last generated message ("" if none)
87		Cause     error  // underlying: context.DeadlineExceeded / *exec.ExitError / nil
88	}
...
99	func (e *RescueError) Unwrap() error { return e.Kind }
```

### 5b. Multi-turn failure → identical rescue (FR-T7 confirmed feasible)

The existing one-shot-exhaustion rescue (the very return the gate sits in front of, `generate.go:288–292`):

```go
288	if !success {
289		return Result{}, &RescueError{
290			Kind: ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA,
291			Candidate: candidate, Cause: lastCause,
292		}
293	}
```

**FR-T7 confirmation:** any multi-turn failure (mid-turn non-zero exit, turn timeout, final-turn
parse/dedupe failure) can `return Result{}, &RescueError{ Kind: ErrRescue, TreeSHA: treeSHA,
ParentSHA: parentSHA, Candidate: candidate, Cause: <multi-turn cause> }` **byte-identically** to the
one-shot path. The CLI's `errors.As(err, &re)` + `FormatRescue(re.TreeSHA, …)` plumbing is unchanged;
the rescue message is unchanged; exit code stays 3. The only thing the implementer must do is set
`Candidate` to the multi-turn final-turn stdout (if any) and `Cause` to the turn's error.

There is also a **`RescueError{Kind: ErrTimeout}`** precedent for per-turn timeout at
`generate.go:244–249` (the DeadlineExceeded branch) — but the delta says a multi-turn turn-timeout
should abort to `ErrRescue` (FR-T7 "any turn ... timeout → existing rescue"), NOT `ErrTimeout`.
`ErrTimeout` is reserved for the one-shot kill. Use `ErrRescue` for multi-turn turn-timeout to keep
exit code 3 (not 124).

---

## Architecture summary — how the pieces connect for R2+R3

```
Defaults() [config.go:161]
   │ sets MultiTurnFallback=true, MultiTurnChunkTokens=32000
   ▼
loadTOML→materialize [file.go:193]   ──► overlay [file.go:294] (global→repo→git-config)
   │ fileGeneration.MultiTurnFallback/ChunkTokens [file.go:54+]      │ if src.X { dst.X = … }
   ▼                                                                  ▼
resolved config.Config [config.go:63, fields ~:84]
   │
   ▼ passed to generate.CommitStaged(ctx, deps, cfg) [generate.go:139]
   │
   ├─ sysPrompt built ONCE [generate.go:170]   ◄── reused by multi-turn turn 1 (system_prompt_flag)
   ├─ diff captured ONCE [generate.go:175]      ◄── multi-turn payload source (FR-T2 capture-once)
   ├─ snapshot (treeSHA, parentSHA) [generate.go:185]   ◄── RescueError.{TreeSHA,ParentSHA}
   │
   ├─ one-shot retry loop [generate.go:226–284]
   │     payload := prompt.BuildUserPayload(diff, cfg.Context, rejected) [:228]  ◄── LOOP-SCOPED ⚠️
   │
   └─ !success → [generate.go:288]  ◄── FR-T1 TRIGGER GATE inserts here
         │ FR-T1 a–d: exhausted && EstimateTokens(payload) > cfg.MultiTurnChunkTokens
         │            && cfg.MultiTurnFallback && manifest.SessionMode=="append"
         │   (SessionMode is R1's deliverable — does NOT exist yet)
         ├─ all hold → multi-turn N+1 turns (new multiturn.go) → success | failure
         │     chunk sizing: N = ceil(EstimateTokens(payload) / cfg.MultiTurnChunkTokens)  [tokens.go:25]
         │     failure → fall through to rescue (FR-T7)
         └─ else → existing rescue return [generate.go:289] (unchanged bytes)
```

---

## Start here (for the R2/R3 implementer)

1. **`internal/config/config.go`** — add the two fields after `MaxDuplicateRetries` (line 83) and
   the two `Defaults()` entries after line 176. Smallest, lowest-risk change; lands R2's resolved-config half.
2. **`internal/config/file.go`** — add to `fileGeneration` (after line 54), `materialize` (after
   line 231), `overlay` (after line 345). Use the `AutoStageAll` bool template for
   `MultiTurnFallback` (only-true-propagates — accepted limitation) and the `TokenLimit`/`MaxCommits`
   `!= 0` int template for `MultiTurnChunkTokens`. Lands R2's file/overlay half.
3. **`internal/generate/generate.go`** — insert the FR-T1 trigger gate between lines 287 and 288.
   Resolve the `payload` loop-scope issue first (§1b above): either hoist `var payload string`
   before the loop, or call `prompt.BuildUserPayload(diff, cfg.Context, nil)` once at the gate.
   Depends on R1 (`manifest.SessionMode`) for condition (d) — gate defensively on
   `deps.Manifest.Resolve().SessionMode == "append"` (nil-safe if R1 uses a pointer/string field).
4. **`internal/generate/multiturn.go`** (NEW) — chunk-sizing helper + N+1 turn protocol. Uses
   `git.EstimateTokens` (`tokens.go:25`) for both sizing and the gate.

---

## Residual risks / open questions for the parent

1. **`payload` is loop-scoped (§1b).** The delta's "do not recompute" is in tension with the current
   loop structure. The implementer must choose: hoist to function scope (cleaner) or accept one
   deterministic `BuildUserPayload` re-call at the gate (produces identical bytes, same `diff`
   input). Flag for explicit decision.
2. **`MultiTurnFallback` bool-sentinel limitation (§3b).** Default-true bool with the
   only-true-propagates pattern means `multi_turn_fallback = false` in a file is silently ignored.
   Delta accepts this ("follow auto_stage_all"). Surface in `docs/configuration.md`. Alternative
   (`*bool` like `DiffContext`) widens scope — not recommended.
3. **Delta misnames `overlay` as `merge` (validation table).** No `merge` exists in `internal/config/`.
4. **R1 dependency:** `SessionMode` does not exist yet. The R3 trigger gate's condition (d)
   (`FR-T1d`) cannot compile until R1 lands the manifest field. R3 should be sequenced after R1, or
   gated behind a nil-safe accessor.
5. **`MultiTurnChunkTokens` `> 0` vs `!= 0`:** delta says `> 0`; existing int fields use `!= 0`. For
   a positive default (32000) and positive user values they are equivalent. `!= 0` matches house
   style; `> 0` is also correct. Either is acceptable.
