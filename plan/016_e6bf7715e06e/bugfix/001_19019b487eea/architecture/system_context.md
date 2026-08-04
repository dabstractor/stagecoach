# System Context — Bugfix 001: Documentation Drift Remediation

## Scope

Two documentation drift defects introduced by the v2.9 chrome-disable feature's
commits (`b8f081d` rewrote the providers.md table rows to add the Chrome-disable
column; `71e57c7a` rewrote the how-it-works.md safety paragraph to append the
chrome sentence). Both are **documentation-only** — no code, no manifest, no test
logic changes. The chrome-disable feature itself is functionally correct and
fully tested.

## Defects to fix

### Defect 1 (Major): docs/providers.md opencode Delivery column

**File:** `docs/providers.md`, line 82
**Field:** Delivery column, opencode row

**Current (stale):**
```
| `opencode` | positional | (none) | `-m` | ... |
```

**Correct (matches binary):**
```
| `opencode` | stdin | (none) | `-m` | ... |
```

**Authoritative sources (all agree on `stdin`):**
| Source | Value | Location |
|--------|-------|----------|
| Binary (`stagecoach providers show opencode`) | `prompt_delivery = 'stdin'` | verified 2026-07-13 |
| `internal/provider/builtin.go` | `PromptDelivery: strPtr("stdin")` | line 327 (inside `builtinOpenCode()`) |
| `providers/opencode.toml` | `prompt_delivery = "stdin"` | line 45 |
| `internal/provider/builtin_test.go` decode-parity TOML | `prompt_delivery = "stdin"` | line 87 |

**Root cause:** Commit `b8f081d` ("Add Chrome-disable column to providers.md table")
rewrote every table row to insert the Chrome-disable column. The opencode row's
Delivery value `positional` was carried forward unchanged from the PRD's original
§12.6 text, even though the delivery method was revised to `stdin` in commit
`010ecee` (2026-07-08). The rewrite touched this exact line (confirmed via
`git blame -L 82,82 docs/providers.md`) and missed the stale value.

**Why it matters:** The file's contract (stated in `providers/*.toml` headers and
implied by `docs/README.md` line 8: "the binary is authoritative") requires docs
to mirror the shipped binary. Documenting `positional` hides the stdin-delivery
behavior and would mislead anyone debugging an opencode large-diff failure
(stdin was specifically chosen to avoid the 128 KB `MAX_ARG_STRLEN` ceiling on
~300 KB diffs).

### Defect 2 (Minor): docs/how-it-works.md safety paragraph provider enumeration

**File:** `docs/how-it-works.md`, line 197
**Section:** `### Safety invariant`

**Current (incomplete):**
```
… either via explicit tool-disable flags (pi, claude) or read-only constraint flags (codex, cursor). …
```

This makes a universal claim ("Every built-in manifest constrains the agent to a
read-only mode") but names only 4 of 7 providers. The sentence reads as
exhaustive (no "e.g." qualifier), implying agy/qwen-code/opencode don't fit
either bucket — which is incorrect. All three are read-only-constrained:

| Provider | Constraint mechanism |
|----------|---------------------|
| codex | `--sandbox read-only --ephemeral` |
| cursor | `--mode ask --trust` |
| agy | `--mode plan` |
| qwen-code | `--approval-mode default` |
| opencode | `run` subcommand (read-only by design) |

**Root cause:** Commit `71e57c7a` rewrote the entire safety paragraph to append
the chrome-less sentence. The stale parenthetical `(codex, cursor)` was carried
through the rewrite rather than refreshed to include all 5 constrained providers.

## Full 7-provider cross-check (binary vs docs table)

Verified 2026-07-13 via `/tmp/stagecoach_verify providers show <name> | grep prompt_delivery`:

| Provider | Binary `prompt_delivery` | docs/providers.md Delivery | Match? |
|----------|-------------------------|---------------------------|--------|
| pi | `stdin` | `stdin` | ✅ |
| claude | `stdin` | `stdin` | ✅ |
| opencode | `stdin` | `positional` | ❌ **Defect 1** |
| codex | `stdin` | `stdin` | ✅ |
| cursor | `positional` | `positional` | ✅ |
| agy | `stdin` | `stdin` | ✅ |
| qwen-code | `stdin` | `stdin` | ✅ |

**opencode is the only mismatch.** Fixing Defect 1 restores 7/7 consistency.

## Existing test coverage (NOT changing)

- `TestBuiltinManifests_ChromeDisableContract` (builtin_test.go:754) — asserts
  FR-C2 (pi/claude chrome-disable flag presence) and FR-C4(b) (read-only
  constraint flags for 5 constrained providers). All pass.
- `TestBuiltinManifests_OpenCodeFields` and decode-parity test — assert
  `PromptDelivery: "stdin"` in the Go struct and TOML. All pass.
- **No docs-binary consistency test exists** — the docs table is not asserted
  against binary field values in any Go test. This is why the drift was not
  caught by CI. (Out of scope for this bugfix — noted as a future improvement.)

## Files to touch

| File | Change | Defect |
|------|--------|--------|
| `docs/providers.md` | Line 82: change opencode Delivery cell from `positional` to `stdin` | 1 |
| `docs/how-it-works.md` | Line 197: expand read-only constraint flags parenthetical to name all 5 providers (or add "e.g.") | 2 |

## What is NOT changing

- No source code changes (`internal/provider/builtin.go`, `manifest.go`, etc.)
- No test changes (`builtin_test.go`, `manifest_test.go`, etc.)
- No `providers/*.toml` changes (they are already correct)
- No manifest schema or field changes
- No CLI flags, env vars, or config keys
