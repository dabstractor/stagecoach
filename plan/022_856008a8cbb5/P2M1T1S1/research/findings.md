# Research Findings — P2.M1.T1.S1: manifest EventStream field + render insertion + ProgressiveStdout + no-extractor guard

## §0 Sources & scope

- **Authoritative**: `plan/022_856008a8cbb5/architecture/provider_subsystem.md` (§3.3 SessionMode precedent,
  §4 merge regimes, §5.3 THE INSERTION POINT, §7.4 where the extractor registry belongs) + the work-item
  contract + `docs_drift_inventory.md` L243-253 (the docs edit).
- **LIVE code verified** (provider_subsystem.md was written @ HEAD df98a9a6, BEFORE P1.M2 landed — line
  numbers shifted; the live numbers below are current):
  - `CmdSpec.ProgressiveStdout` EXISTS (P1.M2.T1.S2 landed): executor.go:81 comment, **executor.go:144**
    `if dl.Idle > 0 && (runtime.GOOS == "linux" || spec.ProgressiveStdout)` — the arming rule that consumes
    it. Render NEVER sets it (only tests hand-set it: executor_test.go:79,102). S1 is the producer side.
  - render.go: `func (m Manifest) Render` **:106**; mode switch `:158`, default case comment `:164`,
    `args = append(args, r.BareFlags...)` **:165**, switch closes ~:166, print_flag guard **:167**.
    `RenderMultiTurn` **:223**; `args = append(args, sessionArgs...)` **:281**, print_flag guard **:283**.
  - manifest.go SessionMode precedent UNCHANGED from doc: field **:67**, validate **:129-131**, Resolve
    →strPtr("") **:211-212**. Pointer-scalars design (header :28-36); slices stay plain (nil = absent).
  - merge.go: regime-2 block **:79-91** (TooledFlags example :89-90: `len(override.TooledFlags) > 0` ⇒
    wholesale replace). SessionMode regime-1 at :59-60.
  - builtin.go **:61** the dated `// VERIFIED 2026-07-05 via ...` SessionMode comment (the FR-S2 style).
  - docs/providers.md: "22 fields" **:13**; `session_mode` table row **:29**; capability section :40+ (S1
    does NOT add a subsection — table row + count only, per contract).
- **Parallel task P1.M3.T1.S4**: tests-only, ONE new file `internal/generate/idle_ladder_test.go`. It READS
  executor.go:144's arming rule but touches NO internal/provider file. **No conflict.** Its PRP confirms
  ProgressiveStdout semantics (`Idle>0 && (linux || ProgressiveStdout)`) — S1's arming is exactly what makes
  the pipe channel work off-Linux for event-mode providers.

## §1 Design decisions (grounded in the contract + codebase patterns)

### EventStream is a SLICE field, NOT a pointer (deliberate divergence from SessionMode)
Contract: `EventStream []string toml:"event_stream"` (nil ⇒ []). Slices in this codebase stay plain — nil is
the natural absent sentinel (manifest.go header design call; BareFlags/TooledFlags precedent). Consequences:
- **Validate()**: NO new enum check (it's a free token list, like TooledFlags — which has no Validate check;
  its capability gate is at render :155). "Validated like other optional capability fields" = nil-tolerant,
  gate at render. (Optionally: nothing. Do not invent an enum.)
- **Resolve()**: NO change (slices left as-is; nil stays nil; `len()`==0 for both nil and []). Consumers
  must use `len(m.EventStream) > 0`, never `!= nil`-style checks.
- **Merge (regime-2)**: `len(override.EventStream) > 0` ⇒ wholesale REPLACE; nil/empty override preserves
  base. This means a user layer can SET it on a provider that ships empty, or REPLACE a built-in's (future)
  list — but cannot CLEAR a non-empty base with `[]` (the documented regime-2 limitation, same as
  BareFlags/TooledFlags). Contract explicitly calls this regime.

### Render insertion (BOTH functions) + ProgressiveStdout arming + the no-extractor guard
```go
	// Render (after :165/:166 switch close, BEFORE :167 print_flag guard):
	if len(r.EventStream) > 0 {
		if LookupEventExtractor(m.Name) == nil {   // no registered extractor ⇒ hard config error (D8)
			return nil, fmt.Errorf("provider manifest %q: event_stream is set but no event extractor is registered for this provider (FR-S1; event_stream must be verified before use)", m.Name)
		}
		args = append(args, r.EventStream...)
	}
	// then the existing print_flag guard (ALWAYS LAST, §12.2) — untouched.
	// CmdSpec construction: spec.ProgressiveStdout = len(r.EventStream) > 0   (arm the executor's pipe channel)
```
Mirrored in RenderMultiTurn between :281 (sessionArgs append) and :283 (print_flag). print_flag stays LAST
(pi's trailing `-p` invariant). ProgressiveStdout set on the CmdSpec in BOTH paths (spec built at the
delivery switch in each).

### The Extractor registry type (new sibling file events.go) — Registry itself stays pure data
Per provider_subsystem.md §7.4: package provider, sibling of render.go — NOT a Registry method.
```go
// events.go (NEW)
type Extractor interface {
	// Extract scans one raw stdout line (NDJSON) and reports the terminal assistant
	// message. P2.M1.T1.S2 owns the concrete per-provider implementations + refinement.
	Extract(line string) (message string, terminal bool, err error)
}
// eventExtractors is the per-provider registry (mirrors BuiltinManifests' per-name map shape).
// EMPTY of real providers in S1 (FR-S2: populated only by P2.M2.T1.S2–S5 after dated verification).
var eventExtractors = map[string]Extractor{}
func LookupEventExtractor(name string) Extractor { return eventExtractors[name] }
func RegisterEventExtractor(name string, e Extractor) { eventExtractors[name] = e } // P2.M2 population seam
```
Unexported map + package seam (NOT a mutex — provider tests are same-package, sequential; tests mutate +
restore via t.Cleanup, the established seam-var idiom). D8's test registers a FAKE via the map/Register.

### No built-in declares event_stream (FR-S2: never speculative)
All six ship nil. builtin_test asserts it (loop over BuiltinManifests(): `len(m.EventStream) != 0` ⇒ fail).
Future population (P2.M2) adds the field + a dated `// VERIFIED <date> via <probe>` comment (builtin.go:61
style) in the provider's ctor — NOT this task.

## §2 Tests (contract-mandated; all stdlib `testing`, same-package, "got %q, want %q" style)

| File | Tests |
|---|---|
| manifest_test.go | Unmarshal `event_stream = ["--mode","json"]` → slice; absent → nil; Marshal omits nil/empty (pointer-omitempty parity is automatic for slices via go-toml); Validate passes with nil AND with tokens (no enum). Resolve: slice left nil (documented). |
| merge_test.go | Regime-2: non-empty override REPLACES wholesale; nil override preserves base; empty `[]string{}` override preserves base (not-overridden). Follow NonEmptySliceReplacesWholesale (:140) / EmptyOrNilSlicePreservesBase (:188) shapes. |
| render_test.go | (1) token position Render: manifest with EventStream + registered fake extractor + PrintFlag ⇒ tokens appear AFTER bare_flags, BEFORE print_flag (index assertions). (2) same for TOOLED mode (after tooled_flags). (3) RenderMultiTurn mirror (after sessionArgs, before print_flag). (4) ProgressiveStdout: true when set, false when nil — BOTH Render + RenderMultiTurn. (5) no-extractor hard error: EventStream set, no registration ⇒ error mentions provider name + extractor; (6) happy path w/ fake extractor (D8). (7) nil EventStream ⇒ NO lookup call, no error (all existing paths unaffected). |
| builtin_test.go | All six built-ins ship nil/empty EventStream (FR-S2). |

## §3 Docs (Mode A, surgical): docs/providers.md
- **:13** "Each manifest has 22 fields" → "23 fields".
- **After :29** (session_mode row) insert the event_stream row. Type column must match the REAL type
  (`[]string`, default `[]` (none)) — NOT the drift-inventory draft's "string/\"\"" (draft written before the
  slice decision). Wording (final, per this PRP):
  `| event_stream | list of string | [] (none) | Event-stream output capability (v3.5, FR-S1): the provider's verified JSON-event flags (e.g. pi --mode json, claude --output-format stream-json), appended after the mode flags and before print_flag. Non-empty arms the executor's progressive-stdout activity channel (idle detection off Linux) — and REQUIRES a registered per-provider event extractor: setting it for a provider without one is a hard configuration error. Empty (default) = no event stream; no built-in ships one yet (FR-S2: never speculative). |`
- NO new capability subsection (that ships with P2.M2 population); no other edits.

## §4 Scope fence
- EDIT: internal/provider/manifest.go, merge.go, render.go; NEW internal/provider/events.go;
  docs/providers.md (:13 + one row); tests: manifest_test.go, merge_test.go, render_test.go, builtin_test.go.
- NOT touched: executor.go (consumer already landed), registry.go (stays pure data — the extractor map is a
  sibling, not a Registry member), builtin.go (no provider declares the field; nothing to change),
  generate/decompose call sites (S2's execution path), docs capability section + other docs (Mode B/P2.M2),
  spec/, PRD/plan/tasks, the parallel task's idle_ladder_test.go.
- Gates: `go build ./...`; `go vet ./internal/provider/...`; gofmt; `go test ./internal/provider/... -race`
  (whole package — proves no existing render/merge/builtin test broke); scope guard `git status` == the 9 files.
