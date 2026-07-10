# Issues 1 & 2: Bootstrap Pi Model Handling Bug

## Root Cause

`internal/config/role_defaults.go:56-73` stores pi's models as **bare** strings:

```go
"pi": {
    "planner": "gpt-5.4",      // bare — needs inference backend prefix
    "stager":  "gpt-5.4-mini", // bare
    "message": "gpt-5.4-nano", // bare
    "arbiter": "gpt-5.4-mini", // bare
},
```

The comment on line 59 says: "bare; sub-provider set separately via --provider" — but v3 (FR-R5b/FR-B7)
made a bare model on pi a **HARD ERROR**. The bootstrap has two code paths that emit these bare models
without applying the multi-backend rule.

## Issue 1: Stager-Fallback Path (bootstrap.go:165-181)

### The Bug

`buildBootstrapConfig` computes `piBlanked := target == "pi"` (line 165). This is ONLY true when
pi IS the target. When target is agy/opencode/qwen-code/codex/cursor (providers that cannot serve
as stager), `StagerFallback()` returns `("pi", "gpt-5.4-mini")` — a **fresh** bare model pulled
from `DefaultModelsForProvider("pi")`, bypassing any blanking.

The `if piBlanked { stagerModel = "" }` guard at line 175 does NOT fire because `target != "pi"`.

### Result

```toml
[role.stager]
provider = "pi"
model = "gpt-5.4-mini"    ← BARE: hard error under FR-R5b
```

This breaks decomposition (the DEFAULT action when nothing is staged + dirty tree) immediately
at role resolution, before the planner is even invoked.

### The Correct Path (for comparison: target=="pi")

When `target == "pi"`:
1. `piBlanked = true`
2. ALL roles blanked in the loop (lines 167-170)
3. `StagerFallback("pi", models)` returns `("pi", "")` — stager model already blanked
4. `if piBlanked { stagerModel = "" }` at line 175 — belt-and-suspenders re-blank
5. Multi-backend guidance comment emitted (lines 186-189)
6. Result: `model = ""` for all four roles — correct

### Fix Location

`internal/config/bootstrap.go`, `buildBootstrapConfig()`, after the `StagerFallback()` call (line 174).

Add a guard: when `stagerName == "pi" && stagerName != target`, blank `stagerModel` and emit
the multi-backend guidance comment on the stager block.

### Test That Pins the Bug

`internal/config/bootstrap_test.go:87`:
```go
assertContains(t, content, "[role.stager]", `model = "gpt-5.4-mini"`)
```
Must change to `model = ""`.

The negative guard at lines 41-43 (`if strings.Contains(content, "gpt-5.4")`) only runs for
`target=="pi"` — it should be extended to the stager-fallback test cases.

## Issue 2: Commented-Out Block Path (bootstrap.go:205-222)

### The Bug

The commented-block generation loop iterates over `preferredBuiltins`, skipping the target,
and writes commented `[role.*]` blocks for each OTHER installed provider. For pi, it calls
`DefaultModelsForProvider("pi")` and emits the bare models verbatim:

```go
other := DefaultModelsForProvider(name)  // for "pi": returns bare models
writeCommentedRoleBlock(&b, "planner", name, other["planner"])  // → # model = "gpt-5.4"
writeCommentedRoleBlock(&b, "stager", name, other["stager"])    // → # model = "gpt-5.4-mini"
writeCommentedRoleBlock(&b, "message", name, other["message"])  // → # model = "gpt-5.4-nano"
writeCommentedRoleBlock(&b, "arbiter", name, other["arbiter"])  // → # model = "gpt-5.4-mini"
```

These are COMMENTED so they don't error at load time — but FR-B1 says uncommenting should produce
a working config ("a one-line uncomment"). A user who uncomments the pi block hits:
```
provider render "pi": model "gpt-5.4-nano" on pi must be inference/model
```

Contrast: the commented-out opencode block correctly uses `openai/gpt-5.4` (prefixed, valid).
The bug is specific to pi.

### Fix Location

`internal/config/bootstrap.go`, the commented-block loop (lines 205-222).

For the pi provider specifically, either:
(a) Write the models blank with a guidance comment line, OR
(b) Write placeholder-prefixed example models (e.g. `zai/gpt-5.4`) clearly marked as examples.

Option (a) is consistent with how the target==pi path works (blank + guidance comment).

### `writeCommentedRoleBlock` (bootstrap.go:119-123)

```go
func writeCommentedRoleBlock(b *strings.Builder, role, prov, model string) {
    fmt.Fprintf(b, "# [role.%s]\n", role)
    fmt.Fprintf(b, "# provider = %q\n", prov)
    fmt.Fprintf(b, "# model = %q\n", model)
}
```

May need a variant that adds a guidance comment, or the loop can pre-process pi models.

## Post-Bootstrap ValidateModel Regression Net

The PRD recommends adding a post-bootstrap `ValidateModel` assertion on every active role model
so regressions like Issues 1 & 2 are caught automatically.

**Approach**: A test that, for every `(target, installed)` combination, generates the bootstrap
config, parses it into a `Config`, resolves each active role model, and calls
`Manifest.ValidateModel(model)` on it. Any non-nil error = test failure.

This would have caught Issue 1 immediately (the stager model `gpt-5.4-mini` on pi fails ValidateModel).

**Dependencies**: `internal/config` cannot import `internal/provider` (the config/provider decoupling
invariant). The test must live in a package that CAN import both (e.g., `internal/config` test
package that imports `internal/provider` via a test-only dependency, or a separate integration
test package). Looking at existing tests: `bootstrap_test.go` is in `package config` and does NOT
import `internal/provider`. The ValidateModel call may need to be in a separate test file or use
the provider's public API via a test binary.

Actually, `Manifest.ValidateModel` is a method on an exported type in an importable package.
A test in `package config_test` (external test package) can import both `config` and `provider`.
Or the test can construct a minimal Manifest and call ValidateModel directly.

**Key insight**: The existing `TestBuildBootstrapConfig_ValidTOML` test already iterates over
(target, installed) combinations and validates TOML parsing. The ValidateModel regression test
extends this pattern to also validate model format.
