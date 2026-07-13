# Research: P1.M1.T2.S1 — Chrome-disable contract assertions in builtin_test.go

**Scope**: Add focused test(s) to internal/provider/builtin_test.go asserting the FR-C2 (chrome-disable)
and FR-C4(b) (read-only-constraint) contract on every built-in provider's `BareFlags`. Test-only — no
production code, no docs. All BareFlags values verified against the LANDED builtin.go (S1 Complete).

## 1. S1 has LANDED — the 7 CHROME-DISABLE notes + the BareFlags are in builtin.go

`grep -c CHROME-DISABLE internal/provider/builtin.go` == 7. The actual landed `BareFlags` per provider
(verified by grep — THESE are the values to assert, not the stale PRD §12.7 / item-description values):

| Provider | BareFlags (landed) | FR category |
|---|---|---|
| **pi** | `["--no-tools","--no-extensions","--no-skills","--no-prompt-templates","--no-context-files","--no-session"]` | FR-C2 chrome |
| **claude** | `["--tools","","--setting-sources","","--no-session-persistence"]` | FR-C2 chrome |
| **codex** | `["--sandbox","read-only","--ephemeral"]` | FR-C4(b) read-only |
| **cursor** | `["--mode","ask","--trust"]` | FR-C4(b) read-only |
| **agy** | `["--mode","plan"]` | FR-C4(b) read-only |
| **qwen-code** | `["--approval-mode","default"]` | FR-C4(b) read-only |
| **opencode** | `[]` (NON-NIL empty slice, builtin.go:328) | FR-C4(b) read-only (inherently read-only `run`) |

## 2. ⚠️ CRITICAL DRIFT: the item description's codex flag is STALE — use the landed value

The item description says codex's read-only constraint is "`--sandbox` + read-only as adjacent pair,
OR `--sandbox` present" — and the PRD §12.7 shows `["--sandbox","read-only","--ask-for-approval","never"]`.
The **LANDED** builtin.go codex BareFlags are `["--sandbox","read-only","--ephemeral"]`. builtin.go:340-342
explains: "`--ask-for-approval` is NOT a `codex exec` flag (it lives on interactive `codex`); `--ephemeral`
keeps the run session-clean". So:
- DO assert `containsPair(--sandbox, read-only)` (present, adjacent ✓) AND `containsToken(--ephemeral)`.
- DO NOT assert `--ask-for-approval` or `never` — they are NOT in the landed slice → the test would FAIL.

The item's fallback ("or --sandbox present") is satisfied, but the implementer MUST base every assertion
on the ACTUAL landed BareFlags (above), not the item/PRD prose. This is the #1 one-pass failure mode.

## 3. REUSE existing helpers — do NOT invent a new pattern

`internal/provider/render_test.go` (same `package provider`, accessible from builtin_test.go) ALREADY has:
```go
// render_test.go:762
func containsPair(args []string, flag, val string) bool       // args[i]==flag && args[i+1]==val (adjacent)
// render_test.go:772
func containsToken(args []string, token string) bool          // token appears anywhere in args
```
Use these directly:
- `containsToken(m["pi"].BareFlags, "--no-extensions")` — pi/claude single-flag chrome checks.
- `containsPair(m["codex"].BareFlags, "--sandbox", "read-only")` — adjacent flag+value for the
  read-only-constrained providers (codex/cursor/agy/qwen-code).

For pi's 4-flag chrome set, chain `containsToken` calls (or a tiny local wrapper that names the missing
token in the error). The item suggests a `containsAll` helper — if you add one, make it a thin wrapper
over `containsToken` (don't reimplement the scan). Prefer reusing `containsToken`/`containsPair` verbatim.

## 4. Test-file style to mirror (builtin_test.go)

- `package provider` (internal test package — same as all existing tests there).
- Per-provider tests use `builtinPi()` / `builtinClaude()` / etc. (the constructors). Cross-cutting tests
  use `BuiltinManifests()` (the map): see `TestBuiltinManifests_KeysAndCount` (builtin_test.go:209, asserts
  len==7) and `TestBuiltinManifests_NameMatchesKey` (:226). **Use `BuiltinManifests()`** for the
  cross-provider table — it tests the PUBLIC API and gives all 7 in one call.
- Existing BareFlags assertions use `reflect.DeepEqual(m.BareFlags, wantBare)` (EXACT slice, order-
  dependent) — e.g. PiFields (:256), ClaudeFields (:323), CodexFields (:539), CursorFields (:583),
  AgyFields (:655), QwenCodeFields (:715). These are ORDER-PINNED. **T2.S1's value-add is the
  ORDER-INDEPENDENT contains-check** (survives future flag reordering) — that is the gap the item calls out.
- `assertStr`/`assertNilStr` (defined in manifest_test.go:523/532) are `*string` helpers — NOT used for
  BareFlags (a `[]string`). Use containsToken/containsPair for BareFlags.

## 5. The test design (one focused function, t.Run subtests per FR category)

```go
func TestBuiltinManifests_ChromeDisableContract(t *testing.T) {
    m := BuiltinManifests()

    t.Run("pi_chrome_surfaces_FR-C2", func(t *testing.T) {
        // FR-C2: pi disables every chrome surface it exposes a switch for.
        for _, flag := range []string{"--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files"} {
            if !containsToken(m["pi"].BareFlags, flag) {
                t.Errorf("pi BareFlags missing chrome-disable flag %s (FR-C2): %v", flag, m["pi"].BareFlags)
            }
        }
    })

    t.Run("claude_chrome_surfaces_FR-C2", func(t *testing.T) {
        // FR-C2: --tools "" disables all tools (MCP surfaces as tools); --setting-sources "" blocks the
        // settings files where MCP/skills/extensions are configured.
        for _, flag := range []string{"--tools", "--setting-sources"} {
            if !containsToken(m["claude"].BareFlags, flag) {
                t.Errorf("claude BareFlags missing chrome-disable flag %s (FR-C2): %v", flag, m["claude"].BareFlags)
            }
        }
    })

    t.Run("readonly_constraint_FR-C4b", func(t *testing.T) {
        // FR-C4(b): the read-only, never-mutate constraint flag is present for each read-only-constrained
        // provider. (Mutation safety, NOT chrome — FR-C4 is explicit that the constraint is not a chrome
        // substitute.) Values verified against the LANDED builtin.go.
        cases := []struct {
            name                                      string
            provider                                  string
            pair                                      [2]string // flag, value (adjacent); empty if none
            extraTokens                               []string  // additional standalone tokens
            wantEmpty                                 bool      // opencode: empty BareFlags by design
        }{
            {"codex",     "codex",     [2]string{"--sandbox", "read-only"}, []string{"--ephemeral"}, false},
            {"cursor",    "cursor",    [2]string{"--mode", "ask"},          []string{"--trust"},     false},
            {"agy",       "agy",       [2]string{"--mode", "plan"},         nil,                     false},
            {"qwen-code", "qwen-code", [2]string{"--approval-mode", "default"}, nil,                 false},
            {"opencode",  "opencode",  [2]string{},                         nil,                     true},
        }
        for _, tc := range cases {
            tc := tc
            t.Run(tc.name, func(t *testing.T) {
                flags := m[tc.provider].BareFlags
                if tc.wantEmpty {
                    if len(flags) != 0 {
                        t.Errorf("%s BareFlags = %v, want empty (opencode `run` is inherently read-only)", tc.provider, flags)
                    }
                    return
                }
                if tc.pair[0] != "" && !containsPair(flags, tc.pair[0], tc.pair[1]) {
                    t.Errorf("%s BareFlags missing adjacent pair %s %s (FR-C4b): %v", tc.provider, tc.pair[0], tc.pair[1], flags)
                }
                for _, tok := range tc.extraTokens {
                    if !containsToken(flags, tok) {
                        t.Errorf("%s BareFlags missing token %s (FR-C4b): %v", tc.provider, tok, flags)
                    }
                }
            })
        }
    })
}
```

## 6. Why this design satisfies the item (point-by-point)

- **pi 4 chrome flags** (contains, reorder-safe): ✓ `pi_chrome_surfaces_FR-C2` subtest.
- **claude --setting-sources + --tools**: ✓ `claude_chrome_surfaces_FR-C2` subtest.
- **Read-only-constrained providers**: ✓ `readonly_constraint_FR-C4b` subtest covers codex/cursor/agy/
  qwen-code/opencode, asserting the constraint pair + extras (or empty for opencode). Confirms FR-C4(b).
- **Note/flag consistency (optional)**: the contains-assertions ARE the consistency check — the flags the
  CHROME-DISABLE doc-comments name ("disabled by --no-extensions (bare_flags)" etc.) are exactly the tokens
  asserted present. (The notes are Go doc-COMMENTS, not runtime-parseable; the contains-check is the
  machine-enforceable proxy. No separate mechanism needed.)
- **Cheap, table-driven, no agent invoked**: ✓ pure in-process struct inspection.

## 7. Anchors (verified)

| Symbol | Location | Notes |
|---|---|---|
| `containsPair(args, flag, val)` | render_test.go:762 | REUSE — adjacent flag+value check |
| `containsToken(args, token)` | render_test.go:772 | REUSE — unordered token presence |
| `BuiltinManifests()` | builtin.go (returns map[name]Manifest) | the public API; len 7 (KeysAndCount asserts) |
| `TestBuiltinManifests_PiFields` | builtin_test.go:238 | ORDER-PINNED BareFlags via DeepEqual (the gap T2 fills) |
| `TestBuiltinManifests_KeysAndCount` | builtin_test.go:209 | cross-cutting map-use precedent |
| pi BareFlags | builtin.go:63-69 | 6 tokens incl. the 4 chrome flags |
| claude BareFlags | builtin.go:135-138 | --tools/--setting-sources + 2 "" value tokens |
| codex BareFlags | builtin.go:380-382 | --sandbox read-only --ephemeral (NOT --ask-for-approval) |
| cursor BareFlags | builtin.go:424-426 | --mode ask --trust |
| agy BareFlags | builtin.go:229-230 | --mode plan |
| qwen-code BareFlags | builtin.go:281-282 | --approval-mode default |
| opencode BareFlags | builtin.go:328 | `[]string{}` NON-NIL empty |

## 8. What this task does NOT do (scope fences)

- Does NOT modify builtin.go (S1 owns the notes/flags — Complete).
- Does NOT modify providers/*.toml (S2, parallel — the reference-file notes).
- Does NOT modify docs/*.md (P1.M2.T1).
- Does NOT assert ORDER (DeepEqual order-pinned checks already exist per-provider; T2's value-add is the
  reorder-safe contains-check, complementing not duplicating them).
- Does NOT invoke any agent / do end-to-end verification (FR-C5's "verified vs --help" is a doc-claim in
  the CHROME-DISABLE note; T2 only asserts the FLAG TOKENS are present in the manifest).

## 9. Validation commands

- `go build ./...` (test file compiles — reuses containsToken/containsPair from the same package).
- `go vet ./internal/provider/...`.
- `gofmt -l internal/provider/builtin_test.go` → empty.
- `go test ./internal/provider/ -count=1` (the item's exact command — all tests pass incl. the new one).
- `go test ./internal/provider/ -v -run TestBuiltinManifests_ChromeDisableContract`.
- `make test && make lint`.
- Grep guard: `grep -n 'TestBuiltinManifests_ChromeDisableContract' internal/provider/builtin_test.go` (one hit);
  reuse guard: `grep -n 'containsToken\|containsPair' internal/provider/builtin_test.go` (the reused helpers).
