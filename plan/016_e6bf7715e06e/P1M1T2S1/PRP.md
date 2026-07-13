name: "P1.M1.T2.S1 — Chrome-disable contract assertions in builtin_test.go (FR-C2 / FR-C4b)"
description: >
  Add focused, order-independent (contains-style) test(s) to internal/provider/builtin_test.go asserting
  the FR-C2 chrome-disable contract (pi: --no-extensions/--no-skills/--no-prompt-templates/
  --no-context-files; claude: --tools/--setting-sources) and the FR-C4(b) read-only-constraint contract
  (codex --sandbox read-only + --ephemeral; cursor --mode ask + --trust; agy --mode plan; qwen-code
  --approval-mode default; opencode empty-by-design) on every built-in provider's BareFlags. Reuses the
  EXISTING containsToken/containsPair helpers from render_test.go (same package). Test-only — no
  production code, no docs. Complements (does not duplicate) the existing order-pinned DeepEqual checks.

---

## Goal

**Feature Goal**: Lock the chrome-disable + read-only-constraint contract (PRD §9.28 FR-C2, FR-C4(b);
§12.7.1) as a permanent, order-independent regression guard on every built-in provider's `BareFlags`.
The existing per-provider tests (`TestBuiltinManifests_PiFields`, `_ClaudeFields`, etc.) assert the
EXACT ordered `BareFlags` slice via `reflect.DeepEqual` — they break on any reordering, even a benign
one, and they bury the chrome/constraint INVARIANT inside a 6-element literal. This task adds focused
contains-style assertions that (a) survive future flag reordering and (b) name the FR-C2/FR-C4(b)
contract explicitly, so a future change that drops a chrome-disable or read-only flag is caught with a
clear, contract-named failure — not a noisy whole-slice diff.

**Deliverable**: ONE new test function `TestBuiltinManifests_ChromeDisableContract` in
`internal/provider/builtin_test.go` (`package provider`), with `t.Run` subtests for: pi chrome surfaces
(FR-C2), claude chrome surfaces (FR-C2), and the 5 read-only-constrained providers' constraint flags
(FR-C4b, table-driven, incl. opencode's empty-by-design case). Reuses the existing `containsToken` and
`containsPair` helpers from `render_test.go`.

**Success Definition**:
- pi `BareFlags` contains `--no-extensions`, `--no-skills`, `--no-prompt-templates`, `--no-context-files`
  (the 4 chrome surfaces pi exposes switches for) — order-independent.
- claude `BareFlags` contains `--tools` and `--setting-sources` (the chrome-covering flags).
- codex `BareFlags` contains the adjacent pair `--sandbox read-only` AND the token `--ephemeral`.
- cursor `BareFlags` contains the adjacent pair `--mode ask` AND `--trust`.
- agy `BareFlags` contains the adjacent pair `--mode plan`.
- qwen-code `BareFlags` contains the adjacent pair `--approval-mode default`.
- opencode `BareFlags` is empty (`len == 0`) by design.
- All assertions use the ACTUAL landed `BareFlags` values (not the stale PRD/item prose — see Gotchas).
- `go test ./internal/provider/ -count=1` passes (the item's exact command); `make test` + `make lint` pass.
- No production file touched; no docs touched; existing tests unchanged.

## User Persona (if applicable)

**Target User**: Stagecoach maintainers / CI (regression guard; no user-facing surface).

**Use Case**: A maintainer reorders `BareFlags` (e.g. groups the `--no-*` flags) or re-derives a
provider's flags from a fresh `--help`. The order-pinned `DeepEqual` tests would fail noisily on the
reorder even though the CONTRACT is intact; this test passes (contains, not order) and continues to
guard the invariant. Conversely, if a flag is DROPPED, this test names the missing contract flag clearly.

**User Journey**: PR drops `--no-context-files` from pi (a chrome regression) → the
`pi_chrome_surfaces_FR-C2` subtest fails with "pi BareFlags missing chrome-disable flag --no-context-files
(FR-C2)" → maintainer restores it before merge.

**Pain Points Addressed**: Today the chrome/constraint invariant is implicit in order-pinned literals.
A reorder forces a noisy whole-slice diff that obscures whether the CONTRACT still holds; a dropped flag
is caught only by a human reading the diff. This test makes the contract explicit and reorder-tolerant.

## Why

- **FR-C2 / §9.28**: Each built-in provider's `bare_flags` MUST include the agent's literal disable flag
  for every chrome surface the agent CLI exposes a switch for. pi sets the bar (4 `--no-*` chrome flags);
  claude covers chrome via `--tools ""` + `--setting-sources ""`. This test asserts those flag TOKENS are
  present so the FR-C2 contract is machine-checked, not just documented in a CHROME-DISABLE note.
- **FR-C4(b) / §12.7.1**: The 5 read-only-constrained providers keep their never-mutate constraint flag.
  This test confirms the constraint flag (the mutation-safety guarantee that "stays") is in fact present —
  FR-C4(b) explicitly says the constraint is NOT a chrome substitute, and this test asserts the constraint
  side of that statement.
- **Complementary, not duplicative**: the existing `TestBuiltinManifests_*Fields` tests pin exact order
  (fragile, noisy); T2.S1 adds the contract-named, order-independent view. Both belong.
- **Bounded, test-only**: no production code (S1 owns builtin.go — Complete), no docs (P1.M2.T1), no
  reference files (S2, parallel).

## What

**User-visible behavior**: None (test-only; item point 5: "DOCS: none").

**Technical change (one new test function, reusing existing helpers):**
- `TestBuiltinManifests_ChromeDisableContract` uses `BuiltinManifests()` (the public map) and the EXISTING
  `containsToken` / `containsPair` helpers (render_test.go:762/772, same package). Three `t.Run` subtests:
  (1) pi chrome flags, (2) claude chrome flags, (3) a table over the 5 read-only-constrained providers
  (constraint pair + extras, opencode empty-by-design).

### Success Criteria
- [ ] pi contains all 4 chrome flags (`--no-extensions`, `--no-skills`, `--no-prompt-templates`, `--no-context-files`)
- [ ] claude contains `--tools` and `--setting-sources`
- [ ] codex contains adjacent `--sandbox read-only` + token `--ephemeral`
- [ ] cursor contains adjacent `--mode ask` + token `--trust`
- [ ] agy contains adjacent `--mode plan`
- [ ] qwen-code contains adjacent `--approval-mode default`
- [ ] opencode `BareFlags` is empty (`len == 0`)
- [ ] All 7 assertions based on the LANDED builtin.go values (esp. codex uses `--ephemeral`, NOT `--ask-for-approval`)
- [ ] `go test ./internal/provider/ -count=1` passes; `make test`/`make lint` pass; no production/docs touched

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact landed BareFlags per provider, the existing helpers to reuse (with signatures), the test
file's style/precedent, the codex-stale-flag drift (the #1 failure mode), the FR-C2 vs FR-C4(b) split, and
the scope fences against S1/S2/P1.M2.T1 are all enumerated below.

### Documentation & References

```yaml
- file: internal/provider/builtin.go
  why: "THE source of truth for the BareFlags values being asserted. S1 (Complete) added the 7
        CHROME-DISABLE notes + the flags. The ACTUAL landed BareFlags: pi (:63-69, 6 tokens incl. the 4
        chrome flags); claude (:135-138, --tools/--setting-sources + 2 empty value tokens); codex
        (:380-382, --sandbox read-only --ephemeral); cursor (:424-426, --mode ask --trust); agy
        (:229-230, --mode plan); qwen-code (:281-282, --approval-mode default); opencode (:328, []string{})."
  pattern: "Manifest{... BareFlags: []string{...} ...} per builtinXxx()."
  critical: "BASE EVERY ASSERTION ON THESE LANDED VALUES. The item description's codex flag
             ('--sandbox + read-only ... or --sandbox') is satisfied, but its PRD-era side
             ('--ask-for-approval never') is NOT in the landed slice — builtin.go:340-342 explains
             --ask-for-approval was replaced by --ephemeral. Assert --ephemeral, NEVER --ask-for-approval."

- file: internal/provider/render_test.go
  why: "REUSE these existing helpers — do NOT invent a new scan. containsPair(args, flag, val) @762 returns
        true iff args[i]==flag && args[i+1]==val (adjacent). containsToken(args, token) @772 returns true
        iff token appears anywhere. Same `package provider` → directly callable from builtin_test.go."
  pattern: >
    containsPair(m["codex"].BareFlags, "--sandbox", "read-only") // adjacent flag+value
    containsToken(m["pi"].BareFlags, "--no-extensions")          // unordered token presence
  critical: "Use containsPair for value-taking constraint flags (--sandbox read-only, --mode ask/plan,
             --approval-mode default) — the flag+value adjacency is semantically load-bearing (a reordered
             'read-only --sandbox' would render wrong). Use containsToken for standalone chrome flags.
             For pi's 4-flag set, chain containsToken (or a thin wrapper that names the missing token)."

- file: internal/provider/builtin_test.go
  why: "THE change site. `package provider`. Use BuiltinManifests() (the public map) — the cross-cutting
        precedent is TestBuiltinManifests_KeysAndCount (:209, asserts len==7) and _NameMatchesKey (:226).
        The per-provider _*Fields tests use builtinXxx() constructors + reflect.DeepEqual for ORDER-PINNED
        BareFlags (PiFields :256, ClaudeFields :323, CodexFields :539, CursorFields :583, AgyFields :655,
        QwenCodeFields :715) — T2.S1 COMPLEMENTS these with an order-INDEPENDENT contains-check."
  pattern: "m := BuiltinManifests(); ... containsToken(m[\"pi\"].BareFlags, ...) ..."
  critical: "Do NOT duplicate the order-pinned DeepEqual checks — add the contains-style contract check.
             assertStr/assertNilStr (manifest_test.go:523/532) are *string helpers — NOT for BareFlags
             ([]string); use containsToken/containsPair."

- docfile: plan/016_e6bf7715e06e/architecture/external_deps.md
  why: "The per-provider chrome surface inventory + verification sources/dates the CHROME-DISABLE notes
        cite. Confirms which flags are the chrome surfaces (pi 4 flags; claude 2 flags) vs the read-only
        constraint (codex/cursor/agy/qwen-code/opencode)."
  section: "Per-provider chrome surface inventory"

- docfile: plan/016_e6bf7715e06e/P1M1T1S1/PRP.md
  why: "S1 is the CONTRACT (Complete): it added the CHROME-DISABLE notes and confirmed ZERO bare_flags
        additions were needed (FR-C2 verification). T2.S1 consumes S1's landed output. Read it to confirm
        the flags the notes claim are exactly the tokens asserted here."

- docfile: plan/016_e6bf7715e06e/P1M1T1S2/PRP.md
  why: "S2 (Implementing in parallel) edits providers/*.toml (reference files) — a DIFFERENT file from
        builtin_test.go. No file-level conflict. Read it to confirm the non-overlap."
```

### Current Codebase tree (relevant slice)

```bash
internal/provider/
  builtin.go            # S1's landed CHROME-DISABLE notes + BareFlags (read-only — the values asserted)
  builtin_test.go       # THE change site (package provider) — add TestBuiltinManifests_ChromeDisableContract
  render_test.go        # containsPair (:762) + containsToken (:772) — REUSE (do not redefine)
  manifest_test.go      # assertStr (:523) / assertNilStr (:532) — *string helpers (NOT for BareFlags)
```

### Desired Codebase tree with files to be added

```bash
internal/provider/builtin_test.go   # MODIFY (additive): +TestBuiltinManifests_ChromeDisableContract
# (no new files; no production code; no docs)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (codex stale flag — the #1 one-pass failure mode): the LANDED codex BareFlags are
//   ["--sandbox","read-only","--ephemeral"]. builtin.go:340-342 explains --ask-for-approval was REPLACED
//   by --ephemeral (it's not a `codex exec` flag). The item description's prose ("--sandbox + read-only
//   ... or --sandbox present") is satisfied, but its PRD-era neighbor "--ask-for-approval never" is NOT
//   in the slice. Assert containsPair(--sandbox, read-only) AND containsToken(--ephemeral). NEVER assert
//   --ask-for-approval or "never" — the test would fail. Base EVERY assertion on the landed values.

// CRITICAL (reuse containsToken/containsPair — do NOT reinvent): render_test.go:762/772 already define
//   these helpers in `package provider`. Redefining them in builtin_test.go is a compile error (duplicate
//   symbol in the same package). Call them directly. For pi's 4-flag set, chain containsToken calls or
//   add a tiny local wrapper that names the missing flag in the error — but delegate the scan to
//   containsToken; do not reimplement it.

// CRITICAL (containsPair checks ADJACENCY, not just presence): containsPair(args, flag, val) is true iff
//   args[i]==flag && args[i+1]==val. That is the RIGHT check for value-taking constraint flags
//   (--sandbox read-only, --mode ask, --mode plan, --approval-mode default) because flag+value adjacency
//   is semantically load-bearing in the rendered command. Do NOT use two separate containsToken calls
//   for these — that would pass on a reordered ("read-only ... --sandbox") slice that renders wrong.

// CRITICAL (claude's "" value tokens): claude BareFlags = ["--tools","","--setting-sources","",
//   "--no-session-persistence"]. containsToken(--tools) and containsToken(--setting-sources) both PASS
//   (the flag tokens are present). Do NOT try to assert the "" value tokens via containsToken("") — ""
//   is present but asserting it is meaningless and brittle. The flag-token presence is the FR-C2 contract.

// CRITICAL (opencode empty-by-design): opencode BareFlags = []string{} (NON-NIL empty, builtin.go:328).
//   Assert len == 0 (NOT m["opencode"].BareFlags == nil — it is a non-nil empty slice). The comment at
//   :328 says "do NOT omit" the empty slice — `run` is inherently read-only, so no flags are needed.

// GOTCHA (complement, don't duplicate): the existing TestBuiltinManifests_*Fields tests pin EXACT order
//   via reflect.DeepEqual. T2.S1's value-add is the ORDER-INDEPENDENT contains-check + the contract-named
//   failure. Both kinds of test belong; do not "replace" the DeepEqual checks.

// GOTCHA (FR-C4(b) is mutation safety, NOT chrome): the read-only-constraint assertions confirm the
//   never-mutate flag is present (FR-C4(b)) — they do NOT claim chrome is disabled. FR-C4 is explicit
//   that the constraint is not a chrome substitute. Reflect this in the subtest/comments so a future
//   reader does not mistake "constraint present" for "chrome off".

// SCOPE: do NOT modify builtin.go (S1 — Complete), providers/*.toml (S2 — parallel), docs/*.md
//   (P1.M2.T1), or render_test.go/manifest_test.go (the helpers live there; reuse, don't edit).
//   T2.S1 is builtin_test.go ONLY.
```

## Implementation Blueprint

### Data models and structure
None. Pure test addition. No types, no production code. Reuses `Manifest.BareFlags []string`, the
`BuiltinManifests()` map, and the existing `containsToken`/`containsPair` helpers.

### Implementation Tasks (ordered by dependencies)

> **Prerequisite**: S1 (P1.M1.T1.S1) is Complete — the CHROME-DISABLE notes + landed BareFlags are in
> builtin.go. CONFIRM: `grep -c CHROME-DISABLE internal/provider/builtin.go` == 7. (Verified during research.)

```yaml
Task 1: MODIFY internal/provider/builtin_test.go — add TestBuiltinManifests_ChromeDisableContract
  - FILE: `package provider` (same as all tests there). No new imports needed (containsToken/containsPair
    are in-package; BuiltinManifests is in-package; testing is already imported).
  - PLACE: near the other TestBuiltinManifests_* tests (e.g. after TestBuiltinManifests_QwenCodeFields or
    alongside the cross-cutting KeysAndCount/NameMatchesKey tests).
  - ADD (verbatim shape — reuse containsToken/containsPair):
        // TestBuiltinManifests_ChromeDisableContract asserts the FR-C2 chrome-disable and FR-C4(b)
        // read-only-constraint contracts on every built-in provider's BareFlags, ORDER-INDEPENDENTLY.
        // The existing TestBuiltinManifests_*Fields tests pin exact order via reflect.DeepEqual (fragile
        // under benign reordering); this complements them with a contains-style contract check that names
        // the missing flag clearly. Values verified against the landed builtin.go (S1).
        //
        // FR-C2 (§9.28): pi/claude expose per-surface chrome-disable switches → assert those flag TOKENS
        //   are present. FR-C4(b) (§12.7.1): the 5 read-only-constrained providers keep their never-mutate
        //   constraint flag → assert it (mutation safety, NOT chrome — FR-C4 is explicit).
        func TestBuiltinManifests_ChromeDisableContract(t *testing.T) {
            m := BuiltinManifests()

            t.Run("pi_chrome_surfaces_FR-C2", func(t *testing.T) {
                want := []string{"--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files"}
                for _, flag := range want {
                    if !containsToken(m["pi"].BareFlags, flag) {
                        t.Errorf("pi BareFlags missing chrome-disable flag %s (FR-C2): %v", flag, m["pi"].BareFlags)
                    }
                }
            })

            t.Run("claude_chrome_surfaces_FR-C2", func(t *testing.T) {
                for _, flag := range []string{"--tools", "--setting-sources"} {
                    if !containsToken(m["claude"].BareFlags, flag) {
                        t.Errorf("claude BareFlags missing chrome-disable flag %s (FR-C2): %v", flag, m["claude"].BareFlags)
                    }
                }
            })

            t.Run("readonly_constraint_FR-C4b", func(t *testing.T) {
                // FR-C4(b): the read-only, never-mutate constraint flag is present for each read-only-
                // constrained provider. (Mutation safety, NOT chrome.) Values verified against landed builtin.go.
                // NOTE: codex uses --ephemeral (NOT --ask-for-approval — that is not a `codex exec` flag).
                cases := []struct {
                    name        string
                    provider    string
                    pair        [2]string // adjacent flag+value; empty if none
                    extraTokens []string  // additional standalone tokens
                    wantEmpty   bool      // opencode: empty BareFlags by design
                }{
                    {"codex", "codex", [2]string{"--sandbox", "read-only"}, []string{"--ephemeral"}, false},
                    {"cursor", "cursor", [2]string{"--mode", "ask"}, []string{"--trust"}, false},
                    {"agy", "agy", [2]string{"--mode", "plan"}, nil, false},
                    {"qwen-code", "qwen-code", [2]string{"--approval-mode", "default"}, nil, false},
                    {"opencode", "opencode", [2]string{}, nil, true},
                }
                for _, tc := range cases {
                    tc := tc
                    t.Run(tc.name, func(t *testing.T) {
                        flags := m[tc.provider].BareFlags
                        if tc.wantEmpty {
                            if len(flags) != 0 {
                                t.Errorf("%s BareFlags = %v, want empty (opencode `run` is inherently read-only; FR-C4b)", tc.provider, flags)
                            }
                            return
                        }
                        if tc.pair[0] != "" && !containsPair(flags, tc.pair[0], tc.pair[1]) {
                            t.Errorf("%s BareFlags missing adjacent pair %q %q (FR-C4b): %v", tc.provider, tc.pair[0], tc.pair[1], flags)
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
  - VERIFY every asserted value against the landed builtin.go table (Gotchas §1): codex extraTokens is
    ["--ephemeral"] (NOT "--ask-for-approval"); opencode wantEmpty=true; etc.
  - DEPENDENCIES: S1 landed (it is). No new helpers (reuse containsToken/containsPair).

Task 2: VERIFY build + vet + format + the item's exact command + full package
  - go build ./...
  - go vet ./internal/provider/...
  - gofmt -l internal/provider/builtin_test.go   # must list nothing
  - go test ./internal/provider/ -count=1         # the item's EXACT command (all tests pass)
  - go test ./internal/provider/ -v -run TestBuiltinManifests_ChromeDisableContract
  - make test && make lint
  - REGRESSION-CHECK (by reasoning): confirm the new test would FAIL if a flagged token were dropped
    (e.g. remove "--no-context-files" from pi in builtin.go → the pi subtest fails naming that flag).
    This is the test's reason to exist. (Optional empirical check via a temporary local edit.)
```

### Implementation Patterns & Key Details

```go
// PATTERN: reuse the existing helpers (render_test.go:762/772) — do NOT redefine
//   containsPair(args, flag, val) — adjacent flag+value (semantically load-bearing for value-taking flags)
//   containsToken(args, token)     — unordered token presence (for standalone chrome flags)

// PATTERN: the cross-provider table uses the PUBLIC map (precedent: KeysAndCount, NameMatchesKey)
m := BuiltinManifests()
containsToken(m["pi"].BareFlags, "--no-extensions")               // FR-C2 chrome
containsPair(m["codex"].BareFlags, "--sandbox", "read-only")      // FR-C4b constraint (adjacency matters)
containsToken(m["codex"].BareFlags, "--ephemeral")                // FR-C4b extra (NOT --ask-for-approval)
len(m["opencode"].BareFlags) == 0                                 // FR-C4b empty-by-design (NON-NIL empty)

// PATTERN: order-independent contains-check (the gap T2 fills vs the existing DeepEqual order-pinning)
for _, flag := range []string{"--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files"} {
    if !containsToken(m["pi"].BareFlags, flag) {
        t.Errorf("pi BareFlags missing chrome-disable flag %s (FR-C2): %v", flag, m["pi"].BareFlags)
    }
}
```

### Integration Points

```yaml
NO production / struct / runtime / public-API / docs changes. One new test function.

TEST FILE:
  - internal/provider/builtin_test.go (package provider) — +TestBuiltinManifests_ChromeDisableContract

CONSUMED (read-only):
  - BuiltinManifests() (builtin.go) — the public map of 7 providers.
  - containsToken / containsPair (render_test.go:762/772) — existing in-package helpers, REUSED.

RELATION TO SIBLINGS:
  - S1 (Complete): produced the landed BareFlags + CHROME-DISABLE notes this test asserts against.
  - S2 (Implementing, parallel): edits providers/*.toml — DIFFERENT file, no conflict.
  - P1.M2.T1 (Planned): docs/providers.md Chrome-disable column etc. — not touched here.

UNCHANGED (do NOT touch): builtin.go (S1); providers/*.toml (S2); docs/*.md (P1.M2.T1);
  render_test.go / manifest_test.go (the helpers — reuse, don't edit); the existing _*Fields tests
  (complement, don't duplicate).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build (test file compiles — reuses containsToken/containsPair from the same package)
go build ./...
# Vet the package
go vet ./internal/provider/...
# Format check
gofmt -l internal/provider/builtin_test.go
# Expected: nothing listed. If listed: gofmt -w it.
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new contract test (targeted)
go test ./internal/provider/ -v -run TestBuiltinManifests_ChromeDisableContract
# Expected: all subtests PASS — pi/claude chrome flags present; codex/cursor/agy/qwen-code constraint
#           pairs + extras present; opencode empty.

# The item's EXACT command — the whole provider package, all tests incl. existing _*Fields (unchanged)
go test ./internal/provider/ -count=1
# Expected: ALL pass.

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# Test-only subtask — no runtime behavior change. The within-scope proof is the unit test.
# Optional regression-property check (temporary local edit, then revert): remove "--no-context-files"
# from pi's BareFlags in builtin.go, re-run the new test, observe the pi_chrome_surfaces_FR-C2 subtest
# FAIL with "missing chrome-disable flag --no-context-files (FR-C2)", then revert. (Proves the test is a
# real guard, not a no-op. Reasoning suffices; the empirical check is optional.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: the new test exists
grep -n 'TestBuiltinManifests_ChromeDisableContract' internal/provider/builtin_test.go
# Expected: one hit (the func).

# Grep guard: REUSED the existing helpers (did not redefine them)
grep -n 'containsToken\|containsPair' internal/provider/builtin_test.go
# Expected: calls to containsToken/containsPair inside the new test (NOT `func containsToken`/`func containsPair`
#           definitions — those stay in render_test.go).

# Grep guard: codex asserts --ephemeral, NOT --ask-for-approval (the stale-flag drift)
grep -n 'ephemeral\|ask-for-approval' internal/provider/builtin_test.go
# Expected: "--ephemeral" present in the codex row; NO "--ask-for-approval".

# Scope-boundary guard: NO production file touched
git diff --stat -- internal/provider/builtin.go internal/provider/render_test.go internal/provider/manifest_test.go providers/ docs/
# Expected: empty (T2.S1 is builtin_test.go only).

# Scope-boundary guard: the existing _*Fields tests are UNCHANGED (complement, not edit)
git diff -- internal/provider/builtin_test.go | grep -E '^-' | grep -iE 'DeepEqual|wantBare|Fields'
# Expected: empty (no deletions in the existing tests — T2 only ADDED a new function).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/provider/...` clean
- [ ] `gofmt -l internal/provider/builtin_test.go` empty
- [ ] `make lint` zero errors
- [ ] `go test ./internal/provider/ -count=1` passes (the item's exact command); `make test` passes

### Feature Validation
- [ ] pi contains all 4 chrome flags (--no-extensions/--no-skills/--no-prompt-templates/--no-context-files)
- [ ] claude contains --tools and --setting-sources
- [ ] codex contains adjacent `--sandbox read-only` + token `--ephemeral`
- [ ] cursor contains adjacent `--mode ask` + token `--trust`
- [ ] agy contains adjacent `--mode plan`
- [ ] qwen-code contains adjacent `--approval-mode default`
- [ ] opencode BareFlags is empty (len == 0)
- [ ] All assertions use the LANDED builtin.go values (codex = --ephemeral, NOT --ask-for-approval)

### Scope-Boundary Validation
- [ ] NO production file touched (builtin.go, render_test.go, manifest_test.go unchanged)
- [ ] NO providers/*.toml touched (S2, parallel)
- [ ] NO docs/*.md touched (P1.M2.T1)
- [ ] NO redefinition of containsToken/containsPair (reused from render_test.go)
- [ ] NO edit to the existing _*Fields tests (complemented, not duplicated)

### Code Quality
- [ ] Reuses containsToken/containsPair (anti-pattern: don't create new patterns)
- [ ] Order-independent contains-checks (the value-add over the order-pinned DeepEqual tests)
- [ ] Failure messages name the missing flag + the FR contract (FR-C2 / FR-C4b) + the provider
- [ ] Comments distinguish FR-C2 (chrome) from FR-C4(b) (mutation safety, NOT chrome)

---

## Anti-Patterns to Avoid

- ❌ Don't assert `--ask-for-approval` or `"never"` for codex — the LANDED codex BareFlags are `["--sandbox","read-only","--ephemeral"]`. `--ask-for-approval` was replaced by `--ephemeral` (builtin.go:340-342: it's not a `codex exec` flag). Assert `containsPair(--sandbox, read-only)` + `containsToken(--ephemeral)`. Base EVERY assertion on the landed builtin.go table, not the item/PRD prose. (This is the #1 failure mode.)
- ❌ Don't redefine `containsToken`/`containsPair` in builtin_test.go — they already exist in render_test.go (same `package provider`); redefining is a duplicate-symbol compile error. Reuse them.
- ❌ Don't use two `containsToken` calls for a value-taking constraint flag (e.g. `containsToken(--sandbox)` && `containsToken(read-only)`) — that passes on a reordered `["read-only", ... "--sandbox"]` slice that renders wrong. Use `containsPair(--sandbox, read-only)` — adjacency is semantically load-bearing.
- ❌ Don't duplicate the existing order-pinned `reflect.DeepEqual` BareFlags checks — T2.S1's value-add is the ORDER-INDEPENDENT contains-check with a contract-named failure. Complement, don't duplicate.
- ❌ Don't use `assertStr`/`assertNilStr` for BareFlags — those are `*string` helpers (manifest_test.go:523/532); BareFlags is `[]string`. Use containsToken/containsPair.
- ❌ Don't assert opencode's BareFlags is `== nil` — it is a NON-NIL empty slice (`[]string{}`, builtin.go:328). Assert `len(...) == 0`.
- ❌ Don't assert claude's `""` value tokens — they're present but meaningless to check; the flag TOKENS (`--tools`, `--setting-sources`) are the FR-C2 contract.
- ❌ Don't modify builtin.go (S1 — Complete), providers/*.toml (S2 — parallel), render_test.go/manifest_test.go (the helpers — reuse, don't edit), or docs/*.md (P1.M2.T1). T2.S1 is builtin_test.go ONLY.
- ❌ Don't conflate FR-C4(b) (mutation safety) with chrome-disable — FR-C4 is explicit the constraint is NOT a chrome substitute. The read-only-constraint subtest confirms the never-mutate flag is present; it does NOT claim chrome is off. State this in the comments.
- ❌ Don't write a test that passes against a contract-violating manifest — verify (by reasoning or a temporary local edit) that dropping a flagged token fails the relevant subtest. A test that can't fail is not a guard.

---

## Confidence Score: 9/10

One-pass success is very high: the task is ONE new test function, the values to assert are enumerated
verbatim from the landed builtin.go, the helpers to reuse (containsToken/containsPair) already exist with
verified signatures, and the test style (BuiltinManifests() + t.Run subtests) mirrors existing cross-
cutting tests. The -1 is for the one stale-flag drift this PRP foregrounds: the item description's codex
prose inherits the PRD-era `--ask-for-approval never` which is NOT in the landed slice (replaced by
`--ephemeral`). An implementer who copies the item's codex wording without reading the landed builtin.go
will assert a flag that isn't there and fail. The PRP's Gotchas make the `--ephemeral` requirement
unmissable (it is the #1 CRITICAL gotcha + a Level-4 grep guard), which is the single thing that unlocks
one-pass success. Everything else is a mechanical, reorder-safe mirror of an existing pattern.
