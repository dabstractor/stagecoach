name: "P2.M1.T2.S1 — Provider subsystem test-fixture token refresh (zai/glm-5.2 → anthropic/claude-haiku) across 9 test files"
description: |
  A mechanical TEST-FIXTURE rename across 9 test files in 4 packages (internal/provider, internal/decompose,
  internal/generate, internal/ui): refresh every `glm-5.2` / `glm-5-turbo` / `zai/glm-*` / standalone-`zai`
  (paired with a glm model) token to `claude-haiku` / `anthropic/claude-haiku` / `anthropic`, so the test
  fixtures use the same canonical example token the runtime code+docs now use (P2.M1.T1.S1/S2 landed
  `anthropic/claude-haiku` as the FR-R5b multi-backend illustration). Files: render_test.go, manifest_test.go,
  merge_test.go, builtin_test.go, registry_test.go (internal/provider); roles_test.go (internal/decompose);
  realagent_test.go, multiturn_test.go (internal/generate); output_test.go (internal/ui). The 5 mappings:
  `zai/glm-5.2`→`anthropic/claude-haiku`; bare `glm-5.2`→bare `claude-haiku`; `zai/glm-5-turbo`→
  `anthropic/claude-haiku`; bare `glm-5-turbo`→bare `claude-haiku`; standalone `zai` (a `--provider`/table
  value paired with a glm model)→`anthropic`. CRITICAL SCOPE GUARD: roles_test.go ALSO has `zai/gpt-5.4*`
  tokens (L102/122/123/408/419/420/504/522) — a DIFFERENT model family that the acceptance grep does NOT match;
  those are OUT OF SCOPE and must NOT be touched (a blind global `zai`→`anthropic` would corrupt them). The
  rename is TARGETED to glm tokens only. Input + token-bearing assertions change IN LOCKSTEP (e.g. an input
  `zai/glm-5.2` and its `containsPair(...,"--provider","zai")` assertion both move to anthropic/claude-haiku).
  Test-safe: no scoped test asserts on the RENAMED error-message tail — only the STATIC "must be
  inference/model" substring (preserved by P2.M1.T1.S2) — so all suites stay green. DOCS = Mode A: none
  (test-fixture hygiene; no user/config/API surface). Validates via go test (4 packages), go build, gofmt,
  and the zero-hit acceptance grep.

---

## Goal

**Feature Goal**: Every provider-subsystem test fixture uses the canonical `anthropic/claude-haiku` /
`claude-haiku` example token (matching the runtime code+docs refreshed by P2.M1.T1.S1/S2), so the P2.M1
acceptance grep over these test files returns zero hits and the FR-R5b prefix-fold illustration is
consistent across code, docs, AND tests.

**Deliverable**: Token-replacement edits to exactly 9 existing test files (no new files):
`internal/provider/render_test.go`, `internal/provider/manifest_test.go`, `internal/provider/merge_test.go`,
`internal/provider/builtin_test.go`, `internal/provider/registry_test.go`, `internal/decompose/roles_test.go`,
`internal/generate/realagent_test.go`, `internal/ui/output_test.go`, `internal/generate/multiturn_test.go`.

**Success Definition**: `go test ./internal/provider/... ./internal/decompose/... ./internal/generate/...
./internal/ui/...` is green; `go build ./...` exit 0; `gofmt -l` clean on the 9 files; and the acceptance
grep `rg -n "zai/glm|glm-5\.2|glm-5-turbo" internal/provider/*_test.go internal/decompose/*_test.go
internal/generate/*_test.go internal/ui/*_test.go` returns **zero hits**. The `zai/gpt-5.4*` lines in
roles_test.go are UNTOUCHED (out of scope — different model family).

## User Persona (if applicable)

**Target User**: A contributor/maintainer reading or extending the provider tests — they should see the same
canonical example token (`anthropic/claude-haiku`) the runtime code + docs use, not the author's stale
`zai/glm-5.2` personal-subscription token.
**Use Case**: A contributor adds a new fold test and copies an existing one as a template; the template
should use the current canonical token so the new test is consistent with the spec (PRD §12.3).
**Pain Points Addressed**: Stale, account-specific fixture tokens that imply a default nobody inherits
(PRD §12.3 / FR-D2) and that keep the P2.M1 acceptance grep from reaching zero hits.

## Why

- **P2.M1 consistency sweep**: P2.M1.T1 (S1+S2) refreshed the runtime code + user-facing docs from
  `zai/glm-5.2` to `anthropic/claude-haiku`. The TEST fixtures still use the old token — this task closes
  the loop so code, docs, and tests all use the same example.
- **Zero-hit acceptance gate**: the model_cleanup_scope.md acceptance criterion requires the grep to return
  zero hits repo-wide (excl. spec/plan). Category B (provider-subsystem tests) is this task's slice.
- **FR-R5b mechanism unchanged**: `anthropic/claude-haiku` demonstrates the IDENTICAL slash-prefix fold
  (`--provider anthropic --model claude-haiku`) that `zai/glm-5.2` did. Only the example token moves; the
  fold logic the tests exercise is byte-for-byte the same.

## What

Pure mechanical text-replacement edits across 9 test files. The 5 mappings (see §1 of findings):
`zai/glm-5.2`→`anthropic/claude-haiku`; bare `glm-5.2`→bare `claude-haiku`; `zai/glm-5-turbo`→
`anthropic/claude-haiku`; bare `glm-5-turbo`→bare `claude-haiku`; standalone `zai` (paired with a glm
model)→`anthropic`. Input + token-bearing assertions change in lockstep. The STATIC `"must be
inference/model"` error-substring assertions are UNCHANGED.

### Success Criteria
- [ ] All 9 files renamed per the mappings; every token-bearing line in the findings §3 inventory is updated.
- [ ] `go test ./internal/provider/... ./internal/decompose/... ./internal/generate/... ./internal/ui/...` green.
- [ ] `go build ./...` exit 0; `gofmt -l <9 files>` clean.
- [ ] Acceptance grep (zero-hit gate) over the 9 files → zero hits.
- [ ] `zai/gpt-5.4*` lines in roles_test.go (L102/122/123/408/419/420/504/522) UNTOUCHED.
- [ ] Scope clean: only the 9 test files modified; no code, no docs, no config-subsystem tests (T2.S2), no spec/.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the complete per-file/per-line hit inventory (findings §3) with the exact old→new token for each
line, the 5 replacement mappings, the CRITICAL out-of-scope guard (`zai/gpt-5.4*`), the in-lockstep
assertion rule, the test-safety proof (which assertions are static vs renamed), and the verified Go
validation commands.

### Documentation & References

```yaml
# MUST READ — the complete hit inventory + mappings + the out-of-scope guard + test-safety proof.
- docfile: plan/020_6979db625159/P2M1T2S1/research/findings.md
  why: "§1 the 5 mappings; §2 the CRITICAL zai/gpt-5.4* OUT-OF-SCOPE guard; §3 the per-file/per-line
        inventory with old→new token (every token-bearing line); §4 test-safety (static vs renamed
        assertions); §5 validation; §6 scope fence."

# MUST READ — the sibling task (CONTRACT) that produced the new runtime error-message tail.
- docfile: plan/020_6979db625159/P2M1T1S2/PRP.md
  why: "Establishes the replacement mappings this task reuses (zai/glm-5.2→anthropic/claude-haiku; bare
        glm→bare claude-haiku) + the TEST-SAFETY guarantee: the FR-R5b error tests assert ONLY on the STATIC
        'must be inference/model' substring (preserved by S2), NOT the glm tail — so renaming fixtures keeps
        those error tests green. S2's scope (3 code files + 4 docs) is DISJOINT from this test-only task."

# MUST READ — the scope doc (Categories B = this task; C/D = sibling T2.S2, out of scope).
- docfile: plan/020_6979db625159/architecture/model_cleanup_scope.md
  why: "Category B (provider-subsystem test fixtures) = this task's exact 9 files. Category C/D (config
        migration + generic config tests) = P2.M1.T2.S2 (DO NOT TOUCH). The Replacement Strategy + Acceptance
        Criterion sections define the mappings + the zero-hit grep gate."

# PRD authority for the canonical example + the FR-R5b mechanism the tests exercise.
- url: PRD.md#12.3   # §12.3 — canonical multi-backend example is anthropic/claude-haiku
  why: "Establishes anthropic/claude-haiku as the canonical token the fixtures should use."
- url: PRD.md#9.15   # §9.15 FR-R5b — the inference/model prefix-fold the render/fold tests exercise
  why: "The tests assert the fold (--provider <prefix> --model <rest>); renaming the token keeps exercising
        the SAME mechanism with a different example."

# The files under edit (READ to confirm exact text + lockstep pairs before each edit).
- file: internal/provider/render_test.go
  why: "The biggest file (19 token-bearing lines). Many are INPUT+COMMENT only (the assertion checks
        reasoning tokens, not the model). The fold assertions (L154/314/572/610) + the wantArgs (L91) +
        builtin (L438/439/889/890) reference the token and rename IN LOCKSTEP with their input."
  pattern: "Each fold test: input 'zai/glm-5.2' (or zai/glm-5-turbo) → its containsPair/wantArgs assertion
            '--provider','zai' + '--model','glm-5.2' both move to anthropic/claude-haiku. Bare-model error
            tests (L298) only check err!=nil — safe to rename the bare input."
  gotcha: "L478/L488/L641/L659/L685/L726/L747 assertions do NOT reference the model token (they check
           --thinking / return values) — change ONLY the input + its inline comment there."
- file: internal/provider/manifest_test.go
  why: "L329 asserts the STATIC 'must be inference/model' — UNCHANGED. L18/L63 (glm-5-turbo TOML+assert),
        L323/L325/L338/L340/L350/L383 (ValidateModel fixtures+inputs) all rename per the bare-vs-prefixed rule."
  gotcha: "L325 + L350 are BARE (the error case) → bare claude-haiku. L323/L338/L340/L383 are prefixed →
           anthropic/claude-haiku. Do NOT prefix the bare ones (they must stay the no-slash error illustration)."
- file: internal/provider/merge_test.go
  why: "5 lines, ALL bare glm (L19 glm-5-turbo; L42/325 glm-5.2 fixture; L46/47 the assertion). → bare claude-haiku."
- file: internal/provider/builtin_test.go
  why: "L436 renderArgs(provider,model) + L438/439 the wantArgs; L884 Render(zai/glm-5-turbo) + L889/890 wantArgs.
        provider 'zai'→'anthropic', model 'glm-5-turbo'→'claude-haiku', input 'zai/glm-5-turbo'→'anthropic/claude-haiku'."
- file: internal/provider/registry_test.go
  why: "7 lines, ALL bare glm-5.2 (DefaultModel fixtures L65/215/315 + assertions L70/71/222/341). → bare claude-haiku."
- file: internal/decompose/roles_test.go
  why: "ONLY L318 (comment) + L323 (fixture glm-5-turbo→claude-haiku bare). The error assertions L332/333/391/392
        are STATIC ('must be inference/model' + 'planner') — UNCHANGED."
  gotcha: "CRITICAL — L102/122/123/408/419/420/504/522 use zai/gpt-5.4* (a DIFFERENT family). OUT OF SCOPE. Do NOT
           touch them. The acceptance grep does not match gpt — a blind zai→anthropic would corrupt them."
- file: internal/generate/realagent_test.go
  why: "L40 table row {'glm-5-turbo','zai'} → {'claude-haiku','anthropic'} (model + provider both rename)."
- file: internal/ui/output_test.go
  why: "L250 table row: BOTH the input 'zai/glm-5.2' AND the expected 'Generating with zai/glm-5.2 in pi…' carry
        the token — rename BOTH to anthropic/claude-haiku in lockstep."
- file: internal/generate/multiturn_test.go
  why: "L218/244/268/285/306: the 7th arg 'zai/glm-5.2' to Run(...) → 'anthropic/claude-haiku'. Assertions check
        msg/ok/cause (not the token) — input-only rename."
```

### Current Codebase tree (relevant slice)

```bash
# EDIT (9 test files — Category B):
internal/provider/render_test.go        # 19 token-bearing lines (fold tests; biggest file)
internal/provider/manifest_test.go      # 8 lines (ValidateModel fixtures/inputs; 1 STATIC assertion preserved)
internal/provider/merge_test.go         # 5 lines (all bare glm)
internal/provider/builtin_test.go       # 6 lines (provider+model pairs)
internal/provider/registry_test.go      # 7 lines (all bare glm)
internal/decompose/roles_test.go        # 2 lines ONLY (L318/L323); gpt-5.4* lines OUT OF SCOPE
internal/generate/realagent_test.go     # 1 line (table row)
internal/ui/output_test.go              # 1 line (input + expected both carry token)
internal/generate/multiturn_test.go     # 5 lines (Run model arg)
# DO NOT TOUCH:
#   P2.M1.T1.S1/S2 code+docs (manifest.go, render.go, roles.go, pi.toml, builtin.go, README, docs/*) — runtime surface
#   P2.M1.T2.S2 config-subsystem tests (config_test.go, migrate_test.go, file_test.go, git_test.go, load_test.go,
#       default_action_test.go, config_init_interactive_test.go, providers_test.go) — Category C/D sibling
#   zai/gpt-5.4* lines in roles_test.go (L102/122/123/408/419/420/504/522) — different model family
#   spec/** (human-owned — AGENTS.md rule 1), PRD.md, plan/, tasks.json
```

### Desired Codebase tree with files to be added/edited

```bash
# NONE added. Edits to the 9 existing test files above. No new files, no code, no docs.
```

### Known Gotchas of our codebase & Library Quirks

```text
# CRITICAL (zai/gpt-5.4* in roles_test.go is OUT OF SCOPE): roles_test.go L102/122/123/408/419/420/504/522
# use zai/gpt-5.4, zai/gpt-5.4-mini, zai/gpt-5.4-nano — a DIFFERENT model family. The acceptance grep
# (zai/glm|glm-5\.2|glm-5-turbo) does NOT match them. Edit ONLY L318 (comment) + L323 (fixture) in roles_test.go.
# A blind global zai→anthropic / glm→claude replace would corrupt the gpt fixtures. TARGET glm tokens only.

# CRITICAL (bare vs prefixed — do not cross them): bare 'glm-5.2'/'glm-5-turbo' → bare 'claude-haiku' (they
# are the no-slash error case or an incidental fixture); prefixed 'zai/glm-5.2'/'zai/glm-5-turbo' →
# 'anthropic/claude-haiku'. Prefixing a bare one would break the FR-R5b "no slash = error" test.

# CRITICAL (preserve the STATIC 'must be inference/model' assertions): manifest_test.go:329 + roles_test.go
# L332/333/391/392 assert on the STATIC error substring (preserved by P2.M1.T1.S2), NOT the glm tail. Do NOT
# change those assertion strings — only the fixture INPUT + the token-bearing structural assertions change.

# GOTCHA (lockstep): where an assertion references the SAME token as its input (render_test.go fold tests:
# containsPair '--provider'/'zai' + '--model'/'glm-5.2'; builtin_test wantArgs; output_test expected string),
# BOTH the input AND the assertion must move to the new token together, or the test fails.

# GOTCHA (input-only lines): render_test.go L478/488/641/659/685/726/747 + multiturn_test.go L218/244/268/
# 285/306 — the assertions do NOT reference the model token (they check --thinking / msg / cause). Change
# ONLY the input (+ inline comment, if present). Do not hunt for a non-existent assertion to change.

# GOTCHA (line numbers can drift): the contract/findings cite specific line numbers. RE-ANCHOR with the
# acceptance grep (Task 0) before editing — use TEXT anchors for the actual edits, not bare line numbers.

# GOTCHA (gofmt): the edits are within existing string/comment literals — gofmt -l should report nothing
# changed. If it does, run gofmt -w on that file (likely a stray space from the edit).
```

## Implementation Blueprint

### Data models and structure
None — mechanical text-token rename in test fixtures. No production code, no schema, no behavior change.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: RE-ANCHOR (do this FIRST — line numbers drift)
  - RUN: rg -n "zai/glm|glm-5\.2|glm-5-turbo" internal/provider/render_test.go internal/provider/manifest_test.go \
            internal/provider/merge_test.go internal/provider/builtin_test.go internal/provider/registry_test.go \
            internal/decompose/roles_test.go internal/generate/realagent_test.go internal/ui/output_test.go \
            internal/generate/multiturn_test.go
  - EXPECT: ~54 token-bearing LINES across the 9 files (see findings §3). If line numbers differ from the
    findings, trust the rg output — the TEXT anchors are what matter. Confirm ZERO hits for 'glm' on the
    roles_test.go gpt-5.4 lines (those are 'gpt', not 'glm' — correctly absent from this grep).

Task 1: EDIT internal/provider/render_test.go (the biggest file — fold tests)
  - Apply the mappings to each token-bearing line in findings §3 (render_test.go block). Key pairs:
    * L87 input "zai/glm-5-turbo"→"anthropic/claude-haiku" + comment; L91 wantArgs "zai"→"anthropic",
      "glm-5-turbo"→"claude-haiku".
    * L152 comment + L153 input + L154 containsPair assertion: all three tokens move together.
    * L298 bare input "glm-5.2"→"claude-haiku" (assertion is err==nil — input only).
    * L306 bare DefaultModel "glm-5.2"→"claude-haiku".
    * L312 comment + L313 input + L314 containsPair assertion: lockstep.
    * L478/L488 input+comment only (assertions check --thinking); L567 input + L572 wantArgs; L605 input +
      L610 wantArgs; L641/L659/L685/L726/L747 input only (structural assertions).
  - USE the edit tool with unique oldText anchors (each fold-test line is unique via its surrounding context).
  - VALIDATE: go test ./internal/provider/ -run 'Render|FR5b|Reasoning|MultiTurn' -v

Task 2: EDIT internal/provider/manifest_test.go
  - L18 TOML "glm-5-turbo"→"claude-haiku" (bare); L63 assertion "glm-5-turbo"→"claude-haiku".
  - L323/L338 DefaultModel strPtr("zai/glm-5.2")→strPtr("anthropic/claude-haiku").
  - L325 input "glm-5.2"→"claude-haiku" (bare); L340 input "zai/glm-5.2"→"anthropic/claude-haiku".
  - L350 DefaultModel strPtr("glm-5.2")→strPtr("claude-haiku") (bare); L383 input "zai/glm-5.2"→"anthropic/claude-haiku".
  - DO NOT touch L329 (STATIC 'must be inference/model' assertion).
  - VALIDATE: go test ./internal/provider/ -run TestValidateModel -v

Task 3: EDIT internal/provider/merge_test.go (5 lines, all bare)
  - L19 "glm-5-turbo"→"claude-haiku"; L42/L325 "glm-5.2"→"claude-haiku"; L46/L47 assertions "glm-5.2"→"claude-haiku".
  - VALIDATE: go test ./internal/provider/ -run Merge -v

Task 4: EDIT internal/provider/builtin_test.go (provider+model pairs)
  - L436 renderArgs "zai"→"anthropic", "glm-5-turbo"→"claude-haiku"; L438 "--provider","zai"→"anthropic";
    L439 "--model","glm-5-turbo"→"claude-haiku".
  - L884 Render "zai/glm-5-turbo"→"anthropic/claude-haiku"; L889 "--provider","zai"→"anthropic";
    L890 "--model","glm-5-turbo"→"claude-haiku".
  - VALIDATE: go test ./internal/provider/ -run 'builtinPi|Builtin' -v

Task 5: EDIT internal/provider/registry_test.go (7 lines, all bare glm-5.2)
  - L65/L215/L315 DefaultModel/default_model "glm-5.2"→"claude-haiku"; L70/L71/L222/L341 assertions→"claude-haiku".
  - VALIDATE: go test ./internal/provider/ -run Registry -v

Task 6: EDIT internal/decompose/roles_test.go (ONLY L318 + L323 — TARGET glm, NOT gpt)
  - L318 comment "glm-5-turbo"→"claude-haiku"; L323 fixture {Model: "glm-5-turbo"}→{Model: "claude-haiku"} (bare).
  - DO NOT touch L102/122/123/408/419/420/504/522 (zai/gpt-5.4* — different family, out of scope).
  - DO NOT touch L332/333/391/392 (STATIC 'must be inference/model' + 'planner' assertions).
  - VALIDATE: go test ./internal/decompose/ -run 'Role|FR5b' -v

Task 7: EDIT internal/generate/realagent_test.go (1 line)
  - L40 {"glm-5-turbo", "zai"} → {"claude-haiku", "anthropic"} (model + provider both rename).

Task 8: EDIT internal/ui/output_test.go (1 line — input AND expected both carry token)
  - L250 row → {"slash-prefixed model", "Generating", "anthropic/claude-haiku", "pi",
    "Generating with anthropic/claude-haiku in pi…"} (BOTH occurrences on the line).
  - VALIDATE: go test ./internal/ui/ -run 'Label|Output' -v

Task 9: EDIT internal/generate/multiturn_test.go (5 lines — Run model arg)
  - L218/L244/L268/L285/L306: the "zai/glm-5.2" arg to Run(...) → "anthropic/claude-haiku".
  - VALIDATE: go test ./internal/generate/ -run Multiturn -v

Task 10: VALIDATE (see Validation Loop) — scoped tests green + go build + gofmt + acceptance grep + scope guard.
```

### Implementation Patterns & Key Details

```text
# PATTERN (the fold-test lockstep — input + assertion move together):
#   INPUT:  builtinPi().Render("zai/glm-5.2", ...)
#   ASSERT: containsPair(s.Args, "--provider", "zai") && containsPair(s.Args, "--model", "glm-5.2")
#   → BOTH become anthropic/claude-haiku:
#   INPUT:  builtinPi().Render("anthropic/claude-haiku", ...)
#   ASSERT: containsPair(s.Args, "--provider", "anthropic") && containsPair(s.Args, "--model", "claude-haiku")
# The fold MECHANISM (split on first '/', emit --provider/--model) is unchanged; only the example token moves.

# PATTERN (bare stays bare — the error case):
#   INPUT:  pi.Render("glm-5.2", ...)        → expects err (no slash on a provider_flag provider)
#   →      pi.Render("claude-haiku", ...)    (still bare, still the no-slash error case)

# PATTERN (input-only lines — don't hunt for an assertion):
#   L478  m.Render("zai/glm-5.2", "", "", lvl)   // the next line asserts --thinking, NOT the model
#   →     m.Render("anthropic/claude-haiku", "", "", lvl)   (change the input + the inline comment ONLY)

# PATTERN (output_test.go — expected string carries the token too):
#   {"...", "Generating", "zai/glm-5.2", "pi", "Generating with zai/glm-5.2 in pi…"}
#   → {"...", "Generating", "anthropic/claude-haiku", "pi", "Generating with anthropic/claude-haiku in pi…"}
```

### Integration Points

```yaml
# NONE (code/build/config/migration/behavior). This is a test-fixture token rename; no production change.
CONSUMES:
  - P2.M1.T1.S2's new runtime error-message tail (anthropic/claude-haiku) — the FR-R5b error tests assert on
    the STATIC 'must be inference/model' substring (preserved by S2), so they stay green after the fixture rename.
  - model_cleanup_scope.md Category B (the 9 files) + the replacement mappings.
PRODUCES:
  - The 9 provider-subsystem test files use anthropic/claude-haiku; the P2.M1 acceptance grep hits zero on them.
PARALLEL (non-conflicting):
  - P2.M1.T1.S2 (code+docs) and P2.M1.T2.S2 (config-subsystem tests, Category C/D) are on DISJOINT files — no merge conflict.
```

## Validation Loop

> **Go-repo test-fixture rename, NOT Python/ruff/mypy.** Use go test (scoped), go build, gofmt, and the
> acceptance grep. Run commands from the repo root.

### Level 1: Build & format (Immediate Feedback)

```bash
# Tests must still compile (the edits are string literals, but confirm no stray quote broke a file).
go build ./...
# Expected: exit 0, no output.

# Formatting check — must print NOTHING.
gofmt -l internal/provider/render_test.go internal/provider/manifest_test.go internal/provider/merge_test.go \
  internal/provider/builtin_test.go internal/provider/registry_test.go internal/decompose/roles_test.go \
  internal/generate/realagent_test.go internal/ui/output_test.go internal/generate/multiturn_test.go
# Expected: empty. If a file is listed, run gofmt -w on it and re-check.
```

### Level 2: Scoped unit tests (the green gate)

```bash
# The 4 packages whose tests were edited.
go test ./internal/provider/... ./internal/decompose/... ./internal/generate/... ./internal/ui/...
# Expected: all PASS, exit 0. (The rename is fixture-input + lockstep-assertion; the fold mechanism + the
# STATIC 'must be inference/model' assertions are unchanged → no behavioral change → green.)
```

### Level 3: Whole-repo test (System Validation)

```bash
# Confirm nothing else broke (the edits are isolated test literals, so this is a sanity gate).
go test ./... 2>&1 | tail -30
# Expected: no FAIL package. (If a package OUTSIDE the 4 edited fails, it is unrelated to this task — flag it,
# do not fix here. The config-subsystem tests are P2.M1.T2.S2's scope.)
```

### Level 4: Acceptance grep (the contract's zero-hit gate) + scope guard

```bash
# THE acceptance criterion: zero hits across the 9 edited test files.
rg -n "zai/glm|glm-5\.2|glm-5-turbo" internal/provider/*_test.go internal/decompose/*_test.go \
   internal/generate/*_test.go internal/ui/*_test.go
# Expected: NO output (zero hits). If any line remains, edit it per findings §3 and re-run.

# Scope guard: ONLY the 9 test files modified; no code/docs/config-tests/spec touched.
git status --porcelain
# Expected: M on the 9 test files ONLY.
git status --porcelain | grep -vE '_test\.go' | grep -E '\.go$|\.md$|\.toml$|spec/|PRD|plan/|tasks\.json' \
  && echo "FAIL: out-of-scope file" || echo "OK: only test files touched"
# Expected: "OK: only test files touched".

# Critical: the gpt-5.4* lines in roles_test.go are UNTOUCHED (different family — out of scope).
rg -n "zai/gpt-5\.4" internal/decompose/roles_test.go
# Expected: the SAME 8 lines (L102/122/123/408/419/420/504/522) still present — proving they were NOT renamed.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` exit 0
- [ ] `go test ./internal/provider/... ./internal/decompose/... ./internal/generate/... ./internal/ui/...` green
- [ ] `gofmt -l <9 files>` prints nothing
- [ ] Acceptance grep (Level 4) over the 9 test files → zero hits

### Feature Validation
- [ ] All 9 files renamed per the 5 mappings; every token-bearing line in findings §3 updated
- [ ] Input + token-bearing assertions changed IN LOCKSTEP (fold tests, builtin wantArgs, output expected string)
- [ ] Bare tokens stayed bare (`glm-5.2`→`claude-haiku`); prefixed stayed prefixed (`zai/glm-5.2`→`anthropic/claude-haiku`)
- [ ] The STATIC `must be inference/model` assertions (manifest_test.go:329, roles_test.go L332/333/391/392) UNCHANGED

### Scope-Boundary Validation
- [ ] `git status --porcelain` shows ONLY the 9 test files modified
- [ ] `zai/gpt-5.4*` lines in roles_test.go (L102/122/123/408/419/420/504/522) UNTOUCHED (Level 4 grep proves it)
- [ ] NO P2.M1.T1 code/docs file touched (manifest.go, render.go, roles.go, pi.toml, builtin.go, README, docs/*)
- [ ] NO P2.M1.T2.S2 config-subsystem test touched (config_test.go, migrate_test.go, file_test.go, git_test.go,
      load_test.go, default_action_test.go, config_init_interactive_test.go, providers_test.go)
- [ ] NO spec/**, PRD.md, plan/, tasks.json touched (human/orchestrator-owned)

### Documentation & Deployment
- [ ] Mode A: none required — this is test-fixture hygiene with no user/config/API surface change

---

## Anti-Patterns to Avoid

- ❌ Don't run a blind global `zai`→`anthropic` or `glm`→`claude` replace. roles_test.go's `zai/gpt-5.4*`
  lines (L102/122/123/408/419/420/504/522) are a DIFFERENT model family and OUT OF SCOPE. Target glm tokens
  only; the acceptance grep (`zai/glm|glm-5\.2|glm-5-turbo`) defines the boundary.
- ❌ Don't prefix a BARE token. `glm-5.2`→`claude-haiku` (bare); `zai/glm-5.2`→`anthropic/claude-haiku`
  (prefixed). Crossing them breaks the FR-R5b "no slash = error" test illustration.
- ❌ Don't change the STATIC `must be inference/model` assertion strings (manifest_test.go:329,
  roles_test.go L332/333/391/392). Those are test-asserted substrings preserved by P2.M1.T1.S2; only the
  fixture INPUT + token-bearing structural assertions change.
- ❌ Don't break the lockstep. Where an input and its assertion reference the same token (fold tests,
  builtin wantArgs, output expected string), BOTH must move — change one and the test fails.
- ❌ Don't hunt for a non-existent assertion on input-only lines (render_test L478/488/641/659/685/726/747,
  multiturn L218/244/268/285/306). Their assertions check `--thinking`/`msg`/`cause`, not the model — change
  the input (+ inline comment) only.
- ❌ Don't edit any P2.M1.T1 code/docs file or any P2.M1.T2.S2 config-subsystem test — those are disjoint
  sibling scopes. This task owns ONLY the 9 provider-subsystem test files.
- ❌ Don't edit `spec/**`, `PRD.md`, `plan/`, or `tasks.json` — human/orchestrator-owned.
- ❌ Don't trust bare line numbers — re-anchor with the acceptance grep (Task 0) and use TEXT anchors.
- ❌ Don't run ruff/mypy/pytest — this is a Go repo. The gates are go test, go build, gofmt, and the grep.

---

## Confidence Score

**One-pass success likelihood: 9/10.** This is a deterministic test-fixture token rename: every edit is
pinned to a verified token-bearing line (the grep inventory is complete — it catches every `zai`, `glm-5.2`,
`glm-5-turbo` occurrence), the 5 mappings are unambiguous, the in-lockstep rule is spelled out per site, and
the test-safety guarantee (static vs renamed assertions) is proven. The 1-point deduction is for the single
real trap — the `zai/gpt-5.4*` lines in roles_test.go that a careless global replace would corrupt — which
the Critical Gotcha + Task 6 + the Level-4 guard (re-grep `zai/gpt-5.4` to prove those lines survived)
explicitly defend against. No behavior, schema, or API change; the FR-R5b fold mechanism the tests exercise
is unchanged.