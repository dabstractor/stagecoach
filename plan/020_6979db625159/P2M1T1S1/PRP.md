name: "P2.M1.T1.S1 — Provider manifest + config code comments/strings: glm→anthropic/claude-haiku (pi.toml, config_init_interactive.go, default_action.go, config.go, bootstrap.go, builtin.go)"
description: >
  A NON-BEHAVIORAL comment/string/fixture cleanup across 6 files: refresh every stale `zai/glm-5.2`,
  `zai/glm-5-turbo`, and `glm-5-turbo` reference to `anthropic/claude-haiku` (spec §12.3 / §16.3 / FR-R5b's
  chosen illustrative token) in providers/pi.toml, internal/cmd/config_init_interactive.go,
  internal/cmd/default_action.go, internal/cmd/config.go, internal/config/bootstrap.go, and
  internal/provider/builtin.go. These illustrate pi's inference-backend prefix-fold (FR-R5b) in personal-
  override examples, wizard prompts, a FR-R5b bare-model-error comment, a config-upgrade migration example,
  bootstrap template comments written to users' files, and a historical comment. NO behavioral change, NO
  test-assertion change, NO `_test.go` edit (Category A only; S2 owns runtime error messages + docs, T2 owns
  test fixtures). VERIFIED test-safety: the only tests near these strings assert on STATIC substrings S1
  preserves ("include the inference backend as a prefix", "multi-backend provider") or use glm as their own
  test INPUT (T2's scope) — so `go test ./internal/cmd/... ./internal/config/... ./internal/provider/...`
  stays GREEN. Two coherent coupled edits: config.go L687+L688 (override-pi example pair) and config.go L94
  (migration illustration's 3 tokens in lockstep). builtin.go L59's DefaultModel VALUE stays `strPtr("")`
  (FR-D2 blank) — only the trailing comment changes. Two out-of-scope residuals flagged (bootstrap.go:261
  `zai/gpt-5.4` not-glm; config.go:698 bare-zai in a myagent template) — left untouched, don't match the
  acceptance grep. File-disjoint from siblings (S2 = manifest.go/render.go/roles.go + 4 docs; T2 = test
  fixtures) and the parallel task P1.M3.T1.S2 (docs/cli.md + README.md winget→chocolatey). Acceptance:
  `rg -n 'zai/glm|glm-5\.2|glm-5-turbo' <the 6 files>` → zero hits; `go test ./internal/cmd/...
  ./internal/config/... ./internal/provider/...` green; `git status` == the 6 files only.

---

## Goal

**Feature Goal**: Remove every stale `zai/glm-5.2` / `zai/glm-5-turbo` / `glm-5-turbo` reference from the 6
provider-manifest + config CODE files (Category A), replacing them with `anthropic/claude-haiku` (the spec's
chosen illustrative token), so the code's comments/strings/prompts no longer carry the author's personal
z.ai/GLM subscription as the example — they illustrate the FR-R5b prefix-fold with a realistic, neutral model.

**Deliverable**: Surgical comment/string edits in exactly 6 files (no behavior change, no test change):
`providers/pi.toml`, `internal/cmd/config_init_interactive.go`, `internal/cmd/default_action.go`,
`internal/cmd/config.go`, `internal/config/bootstrap.go`, `internal/provider/builtin.go`.

**Success Definition**:
- `rg -n 'zai/glm|glm-5\.2|glm-5-turbo' providers/pi.toml internal/cmd/config_init_interactive.go
  internal/cmd/default_action.go internal/cmd/config.go internal/config/bootstrap.go
  internal/provider/builtin.go` returns **zero hits**.
- `go test ./internal/cmd/... ./internal/config/... ./internal/provider/...` is GREEN (non-behavioral —
  verified: no test asserts on the changed text; see "Test Safety").
- `go build ./...` + `gofmt -l <6 files>` (empty) + `go vet ./internal/cmd/... ./internal/config/...
  ./internal/provider/...` all clean.
- `git status --porcelain` == the 6 files ONLY (no `_test.go`, no docs, no spec/, no PRD/plan/tasks).

## User Persona (if applicable)

**Target User**: A user reading `providers/pi.toml` (the shipped reference manifest), running `config init`
(the bootstrap comments are WRITTEN into their config file), or hitting the wizard/upgrade prompts — who
today sees the author's personal `zai/glm-5.2` subscription baked into the examples and wrongly infers z.ai
is the default/recommended backend.
**Use Case**: User opens `pi.toml` to learn how to override pi's model, or runs `config init` interactively
→ sees `anthropic/claude-haiku` as the illustrative override (a realistic, neutral model) instead of the
personal z.ai subscription.
**Pain Points Addressed**: the examples imply z.ai/GLM is the shipped default (it is NOT — FR-D2: pi ships
BLANK), conflating the author's subscription with the product's default. The cleanup makes the examples
neutral + accurate to FR-D2.

## Why

- **FR-D2 (pi ships decoupled/BLANK)**: the shipped pi `default_model` is `""`; the z.ai/GLM tokens in the
  comments/strings are a *personal override* example, but as written they read as the default. Refreshing to
  `anthropic/claude-haiku` (spec §12.3's chosen token) keeps the override concept while de-emphasizing the
  personal subscription.
- **Consistency with the spec + sibling tasks**: spec §12.3 intro, §16.3, and FR-R5b all use
  `anthropic/claude-haiku` as the illustrative multi-backend token; this task aligns the CODE's comments/
  strings with that token. S2 (runtime error messages) + T2 (test fixtures) do the parallel sweep in their
  scopes; S1 is the manifest + config CODE half.
- **Non-behavioral / low-risk**: every edit is a comment, docstring, wizard prompt, or written-config
  comment — no logic, no values (builtin.go L59's `strPtr("")` is unchanged), no test assertions. Verified
  test-safe (§ Test Safety).

## What

Comment/string/token refreshes across 6 files (15 edit sites; 2 are coupled pairs). No logic, no values, no
tests. Two out-of-scope residuals (`bootstrap.go:261 zai/gpt-5.4`, `config.go:698 bare zai`) are flagged and
left untouched (they don't match the acceptance grep and aren't in the contract's line list).

### Success Criteria
- [ ] **providers/pi.toml** (L18, 24, 53, 59, 61): all 5 glm/zai example comments refreshed to
      `anthropic/claude-haiku`; L59's backend list reordered so `zai` is not first.
- [ ] **internal/cmd/config_init_interactive.go** (L184, 200): wizard prompt `e.g. zai/glm-5.2` →
      `e.g. anthropic/claude-haiku` (the static prefix the test asserts on is preserved).
- [ ] **internal/cmd/default_action.go** (L234): `bare "glm-5.2"` → `bare "claude-haiku"` (BARE — the
      FR-R5b error example; no slash added).
- [ ] **internal/cmd/config.go** (L94, 687+688): migration example + override-pi template pair refreshed
      (3 tokens in lockstep on L94; the L687/L688 pair together).
- [ ] **internal/config/bootstrap.go** (L201, 205, 229): the 3 written-config NOTE strings refreshed;
      L261 (`zai/gpt-5.4`) UNCHANGED (out of scope — flagged).
- [ ] **internal/provider/builtin.go** (L34, 59): L34 personal-override example refreshed; L59 comment
      genericized (VALUE `strPtr("")` UNCHANGED).
- [ ] The 6-file scoped grep returns zero hits; the 3 package test suites stay green; scope guard passes.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — every edit site's exact current text + replacement (the §"Implementation Tasks" table), the
test-safety proof (which tests assert on which substrings), the 2 coupled edits, the 2 out-of-scope residuals,
the replacement strategy, and the verified validation gates.

### Documentation & References

```yaml
# MUST READ — the codebase findings (exact edits + test-safety + residuals).
- docfile: plan/020_6979db625159/P2M1T1S1/research/findings.md
  why: "§2 the EXACT current→new text for all 15 edit sites (the authoritative edit table); §3 the test-safety
        verification (which test asserts on which STATIC substring S1 preserves; which glm test data is T2's
        scope); §1 the replacement strategy + the 2 out-of-scope residuals; §4 the validation gates; §5 the
        flagged residuals (bootstrap.go:261, config.go:698, the spec §12.3 gap)."

# MUST READ — the cleanup scope (Category A = S1's 6 files).
- docfile: plan/020_6979db625159/architecture/model_cleanup_scope.md
  why: "Category A lists EXACTLY S1's 6 files + line numbers (pi.toml 18,24,53,61; config_init_interactive.go
        184,200; default_action.go 234; config.go 94,687; bootstrap.go 201,205,229; builtin.go 34,59). The
        replacement strategy + the acceptance grep. NOTE: it also lists manifest.go/render.go/roles.go (S2)
        and Category A-2 docs (S2) — those are NOT S1. Category B/C/D test fixtures = T2."

# MUST READ — the spec token + the prefix-fold contract (the authoritative replacement target).
- docfile: plan/020_6979db625159/prd_snapshot.md   # (or spec/02-providers.md §12.3 — READ-ONLY)
  section: "§12.3 (pi manifest: 'e.g. anthropic/claude-haiku'); §16.3 (git-config: model =
            anthropic/claude-haiku); §9.15 FR-R5b (bare 'claude-haiku' on pi = hard error; the prefix IS the
            field)."
  why: "anthropic/claude-haiku is the spec's chosen illustrative multi-backend token. NOTE: spec §12.3's
        personal-override PARAGRAPH still says zai/glm-5-turbo (a spec gap — the intro says
        anthropic/claude-haiku). Per AGENTS.md hard rule 1, spec/ is NEVER edited; S1 grounds its edits in
        the spec's INTENDED token."
  critical: "FR-R5b: a BARE model (no '/') on pi is a hard error. default_action.go L234 illustrates THAT
             error case — keep it BARE ('claude-haiku', no slash). Everywhere else, use the prefixed
             'anthropic/claude-haiku'."

# MUST READ — the 6 files under edit (re-read at impl to confirm line numbers haven't shifted).
- file: providers/pi.toml
  why: "L18 (override example), L24 (rendered personal-override), L53 (default_model comment), L59 (backend
        list), L61 (prefix-fold illustration)."
- file: internal/cmd/config_init_interactive.go
  why: "L184, L200 — the wizard prompt strings. CRITICAL: the test at L116 asserts on the STATIC prefix
        'include the inference backend as a prefix' (PRESERVED); only the trailing 'e.g. ...' changes."
- file: internal/cmd/default_action.go
  why: "L234 — a COMMENT illustrating the FR-R5b bare-model error. Keep BARE."
- file: internal/cmd/config.go
  why: "L94 (config-upgrade Long docstring migration illustration — 3 coupled tokens); L687+L688 (override-pi
        commented template — a coupled pair). L698 (myagent template bare-zai) UNCHANGED."
- file: internal/config/bootstrap.go
  why: "L201, L205, L229 — WriteString NOTE comments WRITTEN INTO users' config files. CRITICAL: bootstrap_test
        L131/L194 assert on 'multi-backend provider' (PRESERVED). L261 (zai/gpt-5.4) UNCHANGED."
- file: internal/provider/builtin.go
  why: "L34 (historical personal-override comment); L59 (DefaultModel comment). CRITICAL: L59's VALUE
        strPtr(\"\") is UNCHANGED (FR-D2 blank) — only the trailing comment changes."

# CONTEXT — the test-safety proof (read to CONFIRM S1 breaks nothing).
- file: internal/cmd/config_init_interactive_test.go
  why: "L116 asserts Contains(out, 'include the inference backend as a prefix') — STATIC, preserved by S1.
        L589/593 use zai/glm-5.2 as planner OVERRIDE INPUT (Category D / T2's scope) — NOT an assertion on the
        L184/200 prompt S1 changes. READ-ONLY — S1 edits NO test file."
- file: internal/config/bootstrap_test.go
  why: "L131/L194 assert Contains(content/piBlock, 'multi-backend provider') — STATIC, preserved by S1.
        READ-ONLY."
- file: internal/config/bootstrap_validate_test.go
  why: "L23 comment — references the FR-R5b bare-model validation. READ-ONLY context."

# CROSS-REFERENCE — sibling scopes (to AVOID overlap).
- file: internal/provider/manifest.go   # S2 (NOT S1) — error-message glm refs at L139,156
- file: internal/provider/render.go     # S2 (NOT S1) — error-message glm refs at L127,249
- file: internal/decompose/roles.go     # S2 (NOT S1) — error-message glm refs at L163,169
- file: plan/020_6979db625159/P1M3T1S2/PRP.md   # parallel task — docs/cli.md + README.md (winget→chocolatey); NO overlap
  why: "Confirms the parallel task edits docs/cli.md + README.md ONLY — file-disjoint from S1's 6 files."
```

### Current Codebase tree (relevant slice)

```bash
# EDIT (the 6 S1 files):
providers/pi.toml                         # L18,24,53,59,61 — example comments
internal/cmd/config_init_interactive.go   # L184,200 — wizard prompt strings
internal/cmd/default_action.go            # L234 — FR-R5b bare-model comment
internal/cmd/config.go                    # L94,687,688 — migration docstring + override-pi template pair
internal/config/bootstrap.go              # L201,205,229 — written-config NOTE strings
internal/provider/builtin.go              # L34,59 — historical comment + DefaultModel comment
# READ-ONLY (do NOT edit):
internal/provider/manifest.go, render.go, internal/decompose/roles.go   # S2's error-message scope
README.md, docs/cli.md, docs/configuration.md, docs/providers.md         # S2's docs scope
internal/cmd/*_test.go, internal/config/*_test.go, internal/provider/*_test.go  # T2's fixture scope
spec/02-providers.md                      # human-owned (AGENTS.md rule 1); §12.3 personal-override gap flagged
plan/020_6979db625159/architecture/model_cleanup_scope.md                # the scope doc (Category A = S1)
```

### Desired Codebase tree with files to be modified

```bash
# 6 files MODIFIED (comment/string refresh only — no logic, no values, no tests):
providers/pi.toml                         # 5 comment edits
internal/cmd/config_init_interactive.go   # 2 prompt-string edits
internal/cmd/default_action.go            # 1 comment edit
internal/cmd/config.go                    # 3 token edits (L94 x3, L687, L688)
internal/config/bootstrap.go              # 3 WriteString edits (L261 UNCHANGED)
internal/provider/builtin.go              # 2 comment edits (L59 VALUE unchanged)
# NOTHING ELSE.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (default_action.go L234 stays BARE): it illustrates the FR-R5b HARD-ERROR case — a bare model
// (no '/') on pi. Use "claude-haiku" (bare), NOT "anthropic/claude-haiku". Everywhere else uses the
// prefixed form.

// CRITICAL (builtin.go L59 VALUE unchanged): DefaultModel: strPtr("") stays "" (FR-D2 — pi ships BLANK).
// Only the trailing comment ("was glm-5-turbo" → "was a personal-override model") changes. Do NOT alter
// the strPtr("") value.

// CRITICAL (config.go L94 is a COUPLED migration illustration — 3 tokens in lockstep): v2 had
// model="glm-5.2" + default_provider="zai", folding to v3 model="zai/glm-5.2". Change all THREE together
// (claude-haiku / anthropic / anthropic/claude-haiku) or the fold illustration becomes incoherent.

// CRITICAL (config.go L687+L688 are a COUPLED override-pi pair): default_model + default_provider change
// together (claude-haiku / anthropic) so the example stays coherent. (L698's bare-zai myagent line is
// UNCHANGED — out of scope.)

// CRITICAL (test-safety — the asserted STATIC substrings are PRESERVED): config_init_interactive_test.go:116
// asserts "include the inference backend as a prefix"; bootstrap_test.go:131/194 assert "multi-backend
// provider". S1 changes only the trailing "e.g. ..." / inner model token — the asserted prefixes survive.
// Do NOT reword those prefixes.

// GOTCHA (the acceptance grep is SCOPED to S1's 6 files): full-repo zero-hit needs S2+T2 too. S1's gate is
// `rg -n 'zai/glm|glm-5\.2|glm-5-turbo' <the 6 files>` → zero. Do NOT chase hits in manifest.go/render.go/
// roles.go (S2), the docs (S2), or _test.go (T2) — those are out of S1's scope.

// GOTCHA (bootstrap.go L261 = "zai/gpt-5.4" is OUT OF SCOPE): it's z.ai+gpt (not glm); doesn't match the
// grep; the contract lists only L201/205/229. LEAVE it (flagged residual). Same for config.go L698
// (bare "zai" in a myagent template).

// GOTCHA (pi.toml L59 reorder, not removal): "zai | anthropic | google" → "anthropic | google | zai" so zai
// is not first (contract: "does not imply zai is a default"). zai IS a legitimate pi backend — keep it,
// just don't lead with it. (Bare "zai" here doesn't match the grep anyway; the reorder honors the contract.)
```

## Implementation Blueprint

### Data models and structure
None — comment/string/token refresh only. No types, no logic, no values, no tests.

### Implementation Tasks (ordered by dependencies — all independent; any order works)

> Every oldText below was verified by direct read of the current file. Re-read each file at impl time to
> confirm the line number hasn't shifted; the oldText TEXT (not the line number) is the edit-tool anchor.

```yaml
Task 1: providers/pi.toml — 5 comment edits (one edit call, 5 disjoint edits[])
  - L18:  oldText: '#       default_model = "glm-5.2"          # e.g. override only the model'
          newText: '#       default_model = "anthropic/claude-haiku"   # e.g. override only the model'
  - L24:  oldText: '#   (Personal-override example, NOT the shipped default: <backend>=zai, <m>=glm-5-turbo = commit-pi.)'
          newText: '#   (Personal-override example, NOT the shipped default: <backend>=anthropic, <m>=claude-haiku.)'
          (drop "= commit-pi" — no longer accurate once the model is anthropic/claude-haiku)
  - L53:  oldText: '                                    # The zai/glm-5-turbo setup is a personal override (NOT the shipped default).'
          newText: '                                    # An override like "anthropic/claude-haiku" is a personal choice (NOT the shipped default).'
  - L59:  oldText: 'provider_flag = "--provider"        # pi routes to backends: zai | anthropic | google | ...'
          newText: 'provider_flag = "--provider"        # pi routes to backends: anthropic | google | zai | ...'
  - L61:  oldText: '# (e.g. "zai/glm-5.2" → --provider zai --model glm-5.2). FR-R5b enforces this at Render.'
          newText: '# (e.g. "anthropic/claude-haiku" → --provider anthropic --model claude-haiku). FR-R5b enforces this at Render.'

Task 2: internal/cmd/config_init_interactive.go — 2 prompt-string edits
  - L184: oldText: '			prompt = fmt.Sprintf("%s model [%s]; include the inference/ prefix, e.g. zai/glm-5.2: ",'
          newText: '			prompt = fmt.Sprintf("%s model [%s]; include the inference/ prefix, e.g. anthropic/claude-haiku: ",'
  - L200: oldText: '				fmt.Fprintf(w, "multi-backend provider: include the inference backend as a prefix, e.g. zai/glm-5.2\n")'
          newText: '				fmt.Fprintf(w, "multi-backend provider: include the inference backend as a prefix, e.g. anthropic/claude-haiku\n")'
  - CRITICAL: preserve the leading STATIC text ("include the inference/ prefix" / "include the inference
    backend as a prefix") — config_init_interactive_test.go:116 asserts on the latter substring.

Task 3: internal/cmd/default_action.go — 1 comment edit (BARE model — the FR-R5b error case)
  - L234: oldText: '	// misconfiguration (e.g. bare "glm-5.2" on pi — FR-R5b) is rejected up front instead of'
          newText: '	// misconfiguration (e.g. bare "claude-haiku" on pi — FR-R5b) is rejected up front instead of'
  - BARE "claude-haiku" (no slash) — this illustrates the ERROR. Do NOT add a slash.

Task 4: internal/cmd/config.go — 2 edits (L94 coupled-3-tokens; L687+L688 coupled pair)
  - L94 (the config-upgrade Long docstring migration illustration). The current line is a Go string-concat;
    anchor on the unique glm/zai fragment and replace all 3 tokens in lockstep:
      oldText (unique fragment): '`model = \"glm-5.2\"`" + ` + ` + "`default_provider = \"zai\"`" + ` becomes ` + "`model = \"zai/glm-5.2\"`'
      newText:                   '`model = \"claude-haiku\"`" + ` + ` + "`default_provider = \"anthropic\"`" + ` becomes ` + "`model = \"anthropic/claude-haiku\"`'
    (VERIFY the exact surrounding concat quoting by reading config.go:92-96 at impl; the fragment above is
     the unique glm-bearing portion. The 3 tokens MUST change together or the fold is incoherent.)
  - L687+L688 (the override-pi commented template — a coupled pair, edit as one 2-line anchor):
      oldText: '# default_model    = "glm-5.2"\n# default_provider = "zai"'
      newText: '# default_model    = "claude-haiku"\n# default_provider = "anthropic"'
  - L698 (`# default_provider   = "zai"` in the myagent template): UNCHANGED — out of scope (bare zai;
    doesn't match the grep; not in the contract's line list).

Task 5: internal/config/bootstrap.go — 3 WriteString edits (L201, L205, L229)
  - L201: oldText: 'b.WriteString("# e.g. model = \"zai/glm-5.2\". A bare model (no \\'/') on pi is a config error (FR-R5b).\\n")'
          newText: 'b.WriteString("# e.g. model = \"anthropic/claude-haiku\". A bare model (no \\'/') on pi is a config error (FR-R5b).\\n")'
  - L205: oldText: 'b.WriteString("# slash-prefix (e.g. model = \"zai/glm-5.2\"). A bare model (no \\'/') on pi is a config\\n")'
          newText: 'b.WriteString("# slash-prefix (e.g. model = \"anthropic/claude-haiku\"). A bare model (no \\'/') on pi is a config\\n")'
  - L229: oldText: 'stagerAnnotation += " pi is a multi-backend provider — prefix the model with your inference backend, e.g. model = \"zai/glm-5.2\". A bare model (no \\'/') on pi is a config error (FR-R5b)."'
          newText: 'stagerAnnotation += " pi is a multi-backend provider — prefix the model with your inference backend, e.g. model = \"anthropic/claude-haiku\". A bare model (no \\'/') on pi is a config error (FR-R5b)."'
  - CRITICAL: preserve "multi-backend provider" / "A bare model (no '/') on pi is a config error (FR-R5b)" —
    bootstrap_test.go:131/194 assert on "multi-backend provider".
  - L261 (`zai/gpt-5.4`): UNCHANGED — out of scope (not glm; flagged residual).

Task 6: internal/provider/builtin.go — 2 comment edits
  - L34:  oldText: '// (provider=zai, model=glm-5-turbo) is a documented PERSONAL OVERRIDE, not the shipped default —'
          newText: '// (e.g. provider=anthropic, model=claude-haiku) is a documented PERSONAL OVERRIDE, not the shipped default —'
  - L59:  oldText: '		DefaultModel:      strPtr(""), // FR-D2: was glm-5-turbo; decoupled from any one subscription'
          newText: '		DefaultModel:      strPtr(""), // FR-D2: was a personal-override model; decoupled from any one subscription'
  - CRITICAL: L59's strPtr("") VALUE is UNCHANGED. Only the trailing comment changes. builtin_test.go
    asserts on the blank VALUE, not the comment.

Task 7: VERIFY — scoped grep (zero hits) + go test (green) + build/fmt/vet + scope guard (see Validation Loop).
```

### Implementation Patterns & Key Details

```go
// PATTERN (scoped grep gate): S1's acceptance is the 6-file grep, NOT the full-repo grep.
//   rg -n 'zai/glm|glm-5\.2|glm-5-turbo' providers/pi.toml internal/cmd/config_init_interactive.go \
//      internal/cmd/default_action.go internal/cmd/config.go internal/config/bootstrap.go \
//      internal/provider/builtin.go
// → zero hits. (manifest.go/render.go/roles.go + docs + _test.go still have hits — those are S2/T2.)

// PATTERN (test-safety): the changed strings' STATIC prefixes are what tests assert on. Keep them verbatim:
//   "include the inference backend as a prefix" (config_init_interactive_test.go:116)
//   "multi-backend provider"                      (bootstrap_test.go:131,194)

// PATTERN (coupled edits): config.go L94 (3 tokens) and L687+L688 (pair) MUST move in lockstep — a
// migration/override illustration whose input + folded output must stay coherent.

// PATTERN (the 2 UNCHANGED residuals): bootstrap.go:261 (zai/gpt-5.4) + config.go:698 (bare zai) are out
// of S1's scope (not glm; don't match the grep; not in the contract's line list). Leave them; flag them.
```

### Integration Points

```yaml
NO logic / value / config-schema / CLI / test integration. Comment/string refresh only.
CONSUMES (READ-ONLY):
  - spec §12.3 / §16.3 / FR-R5b — anthropic/claude-haiku is the chosen illustrative token.
  - model_cleanup_scope.md Category A — S1's exact file+line list.
PRODUCES (consumed by siblings):
  - S1's 6 cleaned files are the manifest+config-CODE half of the full sweep. S2 (error messages + docs) +
    T2 (test fixtures) finish the full-repo zero-hit. S1's scoped grep is the gate; the full-repo grep lands
    when S2+T2 land.
FLAGGED (not fixed — out of scope):
  - bootstrap.go:261 (zai/gpt-5.4), config.go:698 (bare zai), spec §12.3 personal-override paragraph
    (still zai/glm-5-turbo — spec is human-owned, AGENTS.md rule 1).
NO conflict with the parallel task P1.M3.T1.S2 (docs/cli.md + README.md, winget→chocolatey).
```

## Validation Loop

> **Non-behavioral comment/string cleanup.** The gates are the scoped grep (zero hits) + the 3 package test
> suites (green — proving the edits didn't break the assertions on static substrings) + build/fmt/vet + scope.

### Level 1: Build & format (Immediate Feedback)

```bash
go build ./...                                                    # compiles (comment/string changes — no build impact)
gofmt -l providers/pi.toml internal/cmd/config_init_interactive.go internal/cmd/default_action.go internal/cmd/config.go internal/config/bootstrap.go internal/provider/builtin.go
# Expected: empty (no formatting deltas). gofmt doesn't check .toml — that line is a harmless no-op for pi.toml.
go vet ./internal/cmd/... ./internal/config/... ./internal/provider/...
# Expected: clean.
```

### Level 2: The scoped zero-hit grep (the contract's OUTPUT criterion)

```bash
# S1's 6 files must have ZERO glm/zai-glm hits after the edits.
rg -n 'zai/glm|glm-5\.2|glm-5-turbo' providers/pi.toml internal/cmd/config_init_interactive.go internal/cmd/default_action.go internal/cmd/config.go internal/config/bootstrap.go internal/provider/builtin.go
# Expected: NO output (zero hits). If any line prints, that edit was missed or mistyped — fix it.

# Confirm the new token is present (sanity — the edits landed):
rg -c 'anthropic/claude-haiku' providers/pi.toml internal/cmd/config_init_interactive.go internal/cmd/config.go internal/config/bootstrap.go internal/provider/builtin.go
# Expected: hits in all 5 .go/.toml files that got edits (default_action.go uses bare "claude-haiku" — check that separately):
rg -n 'bare "claude-haiku"' internal/cmd/default_action.go   # Expected: 1 hit (L234)
```

### Level 3: Test suites stay GREEN (the non-behavioral proof)

```bash
# The 3 packages whose files S1 touched. MUST stay green (proves no test asserted on the changed text).
go test ./internal/cmd/... ./internal/config/... ./internal/provider/...
# Expected: PASS (all green). If a test FAILS, it asserts on a string S1 changed — re-read the failure:
#   - config_init_interactive_test.go should still pass (it asserts on the STATIC prefix, preserved).
#   - bootstrap_test.go should still pass (it asserts on "multi-backend provider", preserved).
#   - If a test used glm as its OWN fixture (Category C/D), that failure is T2's scope — DO NOT fix the test
#     in S1; flag it. (Verified: the near-S1 tests do NOT do this — see findings §3.)
```

### Level 4: Scope guard

```bash
# ONLY the 6 S1 files changed.
git status --porcelain
# Expected: exactly 6 files — providers/pi.toml, internal/cmd/config_init_interactive.go,
#   internal/cmd/default_action.go, internal/cmd/config.go, internal/config/bootstrap.go,
#   internal/provider/builtin.go.
git status --porcelain | grep -E '_test\.go|^spec/|README\.md|docs/|manifest\.go|render\.go|roles\.go|PRD\.md|plan/|tasks\.json' && echo "FAIL: out-of-scope file" || echo "OK: scope clean"
# Expected: "OK: scope clean" (NO test files, NO docs, NO spec/, NO S2 error-message files, NO PRD/plan/tasks).
```

## Final Validation Checklist

### Technical Validation
- [ ] Level 1: `go build ./...` + `gofmt -l` (empty) + `go vet` clean
- [ ] Level 2: the 6-file scoped grep returns ZERO hits; `anthropic/claude-haiku` present; `bare "claude-haiku"` at default_action.go L234
- [ ] Level 3: `go test ./internal/cmd/... ./internal/config/... ./internal/provider/...` GREEN
- [ ] Level 4: `git status` == the 6 S1 files ONLY; no `_test.go` / docs / spec / PRD / plan / tasks / S2-files

### Feature Validation
- [ ] pi.toml: 5 example comments refreshed; L59 backend list reordered (zai not first)
- [ ] config_init_interactive.go: 2 wizard prompts refreshed (static prefixes preserved)
- [ ] default_action.go: L234 BARE "claude-haiku" (FR-R5b error case — no slash)
- [ ] config.go: L94 3-token migration illustration + L687/L688 override-pi pair refreshed in lockstep
- [ ] bootstrap.go: 3 NOTE strings refreshed (L261 zai/gpt-5.4 UNCHANGED)
- [ ] builtin.go: L34 example refreshed; L59 comment genericized (strPtr("") VALUE unchanged)
- [ ] A reader of pi.toml / a `config init` user / a wizard user now sees anthropic/claude-haiku (not the personal z.ai/GLM subscription)

### Scope-Boundary Validation
- [ ] NO `_test.go` file edited (test fixtures = T2)
- [ ] NO manifest.go / render.go / roles.go edit (runtime error messages = S2)
- [ ] NO README.md / docs/*.md edit (user-facing docs = S2)
- [ ] NO spec/ edit (human-owned; AGENTS.md hard rule 1)
- [ ] bootstrap.go:261 + config.go:698 UNCHANGED (flagged residuals — out of scope)

---

## Anti-Patterns to Avoid

- ❌ Don't add a slash to default_action.go L234's model. It illustrates the FR-R5b BARE-model HARD ERROR
  ("bare claude-haiku on pi"). A prefixed "anthropic/claude-haiku" there would defeat the example.
- ❌ Don't change builtin.go L59's `strPtr("")` value. It's FR-D2 (pi ships BLANK). Only the trailing comment
  ("was glm-5-turbo" → "was a personal-override model") changes.
- ❌ Don't split the coupled edits. config.go L94's 3 tokens and L687+L688's pair MUST change together or the
  migration/override illustration becomes incoherent (e.g. zai backend folding a claude model).
- ❌ Don't reword the STATIC prefixes the tests assert on ("include the inference backend as a prefix",
  "multi-backend provider"). S1's test-safety DEPENDS on those surviving verbatim.
- ❌ Don't chase full-repo zero-hit in S1. The grep gate is SCOPED to the 6 files. manifest.go/render.go/
  roles.go (S2), the docs (S2), and `_test.go` (T2) still have hits — those are their scopes.
- ❌ Don't edit any `_test.go` file. Even if a test's glm fixture looks stale, it's Category B/C/D = T2's
  scope. S1 is Category A (code comments/strings) ONLY.
- ❌ Don't touch bootstrap.go:261 (`zai/gpt-5.4`) or config.go:698 (bare `zai`). They're not glm, don't match
  the grep, and aren't in the contract's line list. Flagged residuals — leave them.
- ❌ Don't edit `spec/02-providers.md` (even its stale §12.3 personal-override paragraph). AGENTS.md hard
  rule 1 forbids it. Ground the edits in the spec's intended token (anthropic/claude-haiku).
- ❌ Don't remove "zai" from pi.toml L59's backend list — just reorder it (not first). zai IS a legitimate pi
  backend; the contract says "does not imply zai is a default," not "remove zai."

---

## Confidence Score: 9.5/10

This is a mechanical, non-behavioral comment/string refresh with every edit site's exact current→new text
verified by direct file read, a proven test-safety analysis (the only near-S1 tests assert on STATIC
substrings S1 preserves), explicit coupled-edit handling, two flagged out-of-scope residuals, and a
deterministic scoped-grep gate. The 0.5 deduction is for the config.go L94 Go string-concat anchor (the
exact quoting must be confirmed against the live file at impl time — the fragment is unique but the
surrounding backtick/quote nesting is fiddly) and the general risk that a line number shifted between
research and impl (mitigated: the edit anchors are the TEXT, not the line numbers; re-read at impl).