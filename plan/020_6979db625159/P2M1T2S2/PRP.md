name: "P2.M1.T2.S2 — Config-subsystem test-fixture token refresh (zai/glm-5.2 → anthropic/claude-haiku) across 8 test files"
description: |
  A mechanical TEST-FIXTURE rename across 8 test files in 2 packages (internal/cmd, internal/config): refresh every
  `glm-5.2` / `zai/glm-5.2` token AND every bare `default_provider = "zai"` (the v2→v3 fold INPUT) to `claude-haiku`
  / `anthropic/claude-haiku` / `default_provider = "anthropic"`, so the config migration + generic-config test
  fixtures use the canonical example token the runtime code+docs now use (P2.M1.T1.S1/S2 landed
  `anthropic/claude-haiku`). Files: internal/config/migrate_test.go, internal/config/file_test.go,
  internal/config/git_test.go, internal/config/load_test.go, internal/cmd/config_test.go,
  internal/cmd/default_action_test.go, internal/cmd/config_init_interactive_test.go, internal/cmd/providers_test.go.
  The 3 mappings: M1 bare `glm-5.2`→bare `claude-haiku`; M2 `zai/glm-5.2`→`anthropic/claude-haiku`; M3 bare
  `default_provider = "zai"`→`default_provider = "anthropic"` (the v2→v3 fold INPUT — 6 in migrate_test.go, 8 in
  config_test.go). CRITICAL: the contract's acceptance grep (`zai/glm|glm-5\.2|glm-5-turbo`) does NOT match bare
  `default_provider = "zai"`; M3 is MANDATORY (the fold concatenates default_provider+'/'+model, so leaving "zai"
  produces `zai/claude-haiku` ≠ the `anthropic/claude-haiku` assertion → test FAILS). A `rg 'zai'` over the two
  fold-test files (config_test.go + migrate_test.go) is the additional correctness gate. CRITICAL SCOPE GUARDS
  (a blind global `zai`/`glm` replace corrupts these): (a) file_test.go L187/204/205 — a provider-table MERGE test
  using `default_provider:"zai"` + model `"A"` (sentinel, not glm; grep doesn't match) — OUT OF SCOPE; (b)
  config_init_interactive_test.go L105/107/119/120 — `zai/gpt-5.4*` (a DIFFERENT model family; grep doesn't match)
  — OUT OF SCOPE; (c) migrate_test.go case 6 + config_test.go single-backend claude sub-test — use `claude` provider
  with pre-existing `default_provider:"anthropic"`/`"opus"` (no glm/zai token) — OUT OF SCOPE. Input + token-bearing
  assertions change IN LOCKSTEP (e.g. a `default_provider="zai"`+`default_model="glm-5.2"` input and its
  `model="anthropic/claude-haiku"` assertion both move together; the FR51b stderr label + its assertion both move).
  The bare/no-fold cases STAY bare (FR-B7 no-invent illustration). The STATIC `must be inference/model` assertion
  (default_action_test.go:1500) is UNCHANGED (preserved by P2.M1.T1.S2). DOCS = Mode A: none (test-fixture hygiene;
  no user/config/API surface). Validates via go test (2 packages), go build, gofmt, the contract zero-hit grep, and
  the `rg 'zai'`=zero correctness grep.

---

## Goal

**Feature Goal**: Every config-subsystem test fixture (Category C migration tests + Category D generic-config
tests) uses the canonical `anthropic/claude-haiku` / `claude-haiku` / `default_provider = "anthropic"` example
token (matching the runtime code+docs refreshed by P2.M1.T1.S1/S2), so the v2→v3 fold illustrations produce
`anthropic/claude-haiku` (the spec's canonical pi example, PRD §16.3) and the P2.M1 acceptance grep over these
test files returns zero hits.

**Deliverable**: Token-replacement edits to exactly 8 existing test files (no new files):
`internal/config/migrate_test.go`, `internal/config/file_test.go`, `internal/config/git_test.go`,
`internal/config/load_test.go`, `internal/cmd/config_test.go`, `internal/cmd/default_action_test.go`,
`internal/cmd/config_init_interactive_test.go`, `internal/cmd/providers_test.go`.

**Success Definition**: `go test ./internal/cmd/... ./internal/config/...` is green; `go build ./...` exit 0;
`gofmt -l` clean on the 8 files; the contract acceptance grep `rg -n "zai/glm|glm-5\.2|glm-5-turbo" <8 files>`
returns **zero hits**; AND the correctness grep `rg -n 'zai' internal/cmd/config_test.go
internal/config/migrate_test.go` returns **zero hits** (proves the bare-zai `default_provider` fold inputs were
renamed). The out-of-scope guards survive: `zai/gpt-5.4*` in config_init_interactive_test.go (L105/107/119/120)
and `default_provider:"zai"` in file_test.go (L187/204/205) are UNTOUCHED.

## User Persona (if applicable)

**Target User**: A contributor/maintainer reading or extending the config migration / parse tests — they should see
the same canonical example token (`anthropic/claude-haiku`) the runtime code + docs use, not the author's stale
`zai/glm-5.2` personal-subscription token.
**Use Case**: A contributor adds a new v2→v3 fold test and copies an existing one as a template; the template uses
the current canonical token so the new test stays consistent with the spec (PRD §9.17 FR-B7, §16.3).
**Pain Points Addressed**: Stale, account-specific fixture tokens that imply a default nobody inherits (PRD §12.3 /
FR-D2) and that keep the P2.M1 acceptance grep from reaching zero hits.

## Why

- **P2.M1 consistency sweep**: P2.M1.T1 (S1+S2) refreshed the runtime code + user-facing docs from `zai/glm-5.2` to
  `anthropic/claude-haiku`. The config TEST fixtures still use the old token — this task closes the loop so code,
  docs, and tests all use the same example.
- **Zero-hit acceptance gate**: `model_cleanup_scope.md`'s acceptance criterion requires the grep to return zero
  hits repo-wide (excl. spec/plan). Category C (config migration tests) + Category D (generic config tests) is this
  task's slice.
- **FR-B7 / FR-R5b mechanism unchanged**: `default_provider="anthropic"` + `default_model="claude-haiku"` → folded
  `model="anthropic/claude-haiku"` demonstrates the IDENTICAL prefix-fold that `zai`+`glm-5.2` did. Only the example
  token moves; the fold logic the tests exercise is byte-for-byte the same.

## What

Pure mechanical text-replacement edits across 8 test files. The 3 mappings (findings §1):
- **M1** bare `glm-5.2` → bare `claude-haiku` (generic fixtures + bare/no-fold migration cases);
- **M2** `zai/glm-5.2` (already-folded output or idempotent input) → `anthropic/claude-haiku`;
- **M3** bare `default_provider = "zai"` / `"default_provider": "zai"` (the fold INPUT) → `... = "anthropic"` /
  `... : "anthropic"`. **M3 is mandatory** — without it the fold yields `zai/claude-haiku` and the
  `anthropic/claude-haiku` assertion fails.

Input + token-bearing assertions change in lockstep. The bare/no-fold migration cases (FR-B7 no-invent) STAY bare.
The STATIC `must be inference/model` assertion (default_action_test.go:1500) is UNCHANGED.

### Success Criteria
- [ ] All 8 files renamed per the 3 mappings; every token-bearing line in findings §4 is updated.
- [ ] `go test ./internal/cmd/... ./internal/config/...` green; `go build ./...` exit 0; `gofmt -l <8 files>` clean.
- [ ] Contract acceptance grep (zero-hit gate) over the 8 files → zero hits.
- [ ] **Correctness grep** `rg -n 'zai' internal/cmd/config_test.go internal/config/migrate_test.go` → zero hits.
- [ ] Out-of-scope guards UNTOUCHED: `zai/gpt-5.4*` (config_init_interactive_test.go L105/107/119/120) and
      `default_provider:"zai"` field-merge sentinel (file_test.go L187/204/205) and the claude-provider cases
      (migrate_test.go case 6, config_test.go single-backend sub-test) all survive.
- [ ] Scope clean: only the 8 test files modified; no code, no docs, no provider-subsystem tests (T2.S1), no spec/.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the complete per-file/per-line hit inventory (findings §4) with the exact old→new token for each line,
the 3 replacement mappings, the CRITICAL M3 rule (bare-zai default_provider must change or folds break), the
CRITICAL correctness-grep gap (the contract grep misses M3), the three out-of-scope guards, the in-lockstep rule,
the test-safety proof (static vs renamed assertions), and the verified Go validation commands.

### Documentation & References

```yaml
# MUST READ — the complete hit inventory + mappings + the M3 rule + the out-of-scope guards + test-safety proof.
- docfile: plan/020_6979db625159/P2M1T2S2/research/findings.md
  why: "§1 the 3 mappings; §2 the CRITICAL M3/correctness-grep gap (contract grep misses bare default_provider=zai);
        §3 the OUT-OF-SCOPE guards (file_test.go L187/204/205 sentinel; config_init_interactive L105/107/119/120
        gpt-5.4 family; claude-provider cases); §4 the per-file/per-line inventory (every token-bearing line, old→new);
        §5 test-safety; §6 validation; §7 scope fence."

# MUST READ — the sibling task (CONTRACT) that produced the new runtime token + the STATIC error tail.
- docfile: plan/020_6979db625159/P2M1T1S2/PRP.md
  why: "Establishes anthropic/claude-haiku as the canonical token + preserves the STATIC 'must be inference/model'
        error tail (asserted by default_action_test.go:1500). S2's scope (code + user-facing docs) is DISJOINT from
        this test-only task."

# MUST READ — the parallel sibling (CONTRACT) that shares the SAME token mappings; confirms disjoint files.
- docfile: plan/020_6979db625159/P2M1T2S1/PRP.md
  why: "Provider-subsystem test fixtures (Category B, 9 different files) — shares the zai/glm-5.2→anthropic/claude-haiku
        mappings and the same acceptance-grep discipline. Disjoint files → no merge conflict. Its zai/gpt-5.4* guard in
        roles_test.go is the analog of THIS task's zai/gpt-5.4* guard in config_init_interactive_test.go."

# MUST READ — the scope doc (Categories C + D = this task; A/B = siblings).
- docfile: plan/020_6979db625159/architecture/model_cleanup_scope.md
  why: "Category C (config migration fixtures) + Category D (generic config tests) = this task's 8 files. The
        Replacement Strategy + Acceptance Criterion define the mappings + the zero-hit grep gate."

# PRD authority for the canonical example + the v2→v3 fold mechanism the migration tests exercise.
- url: PRD.md#9.17   # §9.17 FR-B7 — the v2→v3 fold: default_provider + '/' + model → prefix (this is M3's rationale)
  why: "Defines the fold that migrate_test.go + config_test.go exercise: the inference provider folds into the model
        prefix. anthropic/claude-haiku is the canonical fold output."
- url: PRD.md#16.3   # §16.3 — git-config example: model = anthropic/claude-haiku for pi
  why: "Establishes anthropic/claude-haiku as the canonical pi example the fixtures should use."

# The files under edit (READ to confirm exact text + lockstep pairs before each edit).
- file: internal/config/migrate_test.go
  why: "TestMigrateV2ToV3 — 12 cases. 6 carry a bare default_provider:'zai' that MUST become 'anthropic' (M3): L23/41/
        56/69/86/166. The fold cases (1-5,10) produce anthropic/claude-haiku; the bare/no-fold cases (7/8/9/11/12)
        STAY bare claude-haiku. Case 6 (single-backend claude, opus) is UNTOUCHED."
  pattern: "Each fold case: input Model:'glm-5.2' + default_provider:'zai' → asserted cfg.Model=='zai/glm-5.2'. Both
            become claude-haiku + anthropic → asserted anthropic/claude-haiku. The 'default_provider should be deleted'
            checks are value-agnostic (key deleted, not value-checked) — UNCHANGED."
  gotcha: "Cases 7/8/9/11/12 are BARE (no fold / no-invent / nil / empty / non-string dp) → bare claude-haiku. Do NOT
           prefix them. Case 5 (idempotent) has ALREADY-folded 'zai/glm-5.2' inputs + asserts 'unchanged' — rename all
           to anthropic/claude-haiku."
- file: internal/cmd/config_test.go
  why: "TestUpgradeConfigVersion_V3Rewrite + TestConfigUpgrade_V2ToV3Rewrite — assert on TOML SUBSTRINGS. 8 bare
        default_provider=\"zai\" occurrences (input + active-form + commented-form asserts): L1349/1359/1362/1382/1402/
        1430/1483/1508 → all anthropic. glm-5.2 inputs + zai/glm-5.2 asserts → claude-haiku / anthropic/claude-haiku.
        L1461 is a NEGATIVE assert (no '/glm-5.2\"' prefix invented) → '/claude-haiku\"'."
  gotcha: "L1359 + L1461 are NEGATIVE assertions (strings.Contains that should be FALSE) — rename their token too, in
           lockstep with the input, or the negative check loses meaning. The single-backend claude sub-test
           (L1413-1431) has no glm/zai token — UNTOUCHED."
- file: internal/config/file_test.go
  why: "Two TOML-parse fixtures (L59/90/91 + L786/800/801): default_model=\"glm-5.2\" + asserts → claude-haiku."
  gotcha: "L187/204/205 are TestOverlayProvidersFieldMerge (default_provider:'zai' + model 'A' sentinel) — OUT OF
           SCOPE. The acceptance grep doesn't match them (no glm). Do NOT touch."
- file: internal/config/git_test.go
  why: "L76 setGitConfig('glm-5.2') + L104/105 assert → claude-haiku (bare). 3 lines."
- file: internal/config/load_test.go
  why: "L148 Setenv('glm-5.2') + L161/162 assert → claude-haiku (bare). 3 lines."
- file: internal/cmd/default_action_test.go
  why: "L1500 bare --model glm-5.2 on pi (FR-R5b error; asserts STATIC 'must be inference/model' — UNCHANGED) → bare
        claude-haiku. L1532 --model glm-5.2 on stub + L1539/1540/1541 FR51b stderr label '↳ Generating with glm-5.2 in
        stub…' → claude-haiku (input + label assertion in LOCKSTEP)."
  gotcha: "L1500 is INPUT-ONLY (assertion is the static error tail, preserved by P2.M1.T1.S2). L1532↔1540/1541 is
           lockstep (the label embeds the --model value; runtime format unchanged)."
- file: internal/cmd/config_init_interactive_test.go
  why: "ONLY L589 override map {'planner':'zai/glm-5.2'} + L593 assert content `model = \"zai/glm-5.2\"` →
        anthropic/claude-haiku. 2 lines."
  gotcha: "L105/107/119/120 are zai/gpt-5.4* (DIFFERENT family) — OUT OF SCOPE. The acceptance grep doesn't match
           them. A blind zai→anthropic replace would corrupt them."
- file: internal/cmd/providers_test.go
  why: "L226 TOML input default_model=\"glm-5.2\" (double quotes) + L241/242 assert 'glm-5.2' (SINGLE quotes — how
        `providers show` renders TOML) → claude-haiku. Keep the quoting style; change ONLY the token."
  gotcha: "L241/242 use SINGLE quotes ('glm-5.2'); L226 uses DOUBLE quotes (\"glm-5.2\"). Match each line's existing
           quoting — don't homogenize. Input (double) + rendered output (single) move in lockstep."
```

### Current Codebase tree (relevant slice)

```bash
# EDIT (8 test files — Category C + D):
internal/config/migrate_test.go              # 12 cases; 6 bare default_provider:"zai" (M3) + glm models
internal/cmd/config_test.go                  # 2 big tests; 8 bare default_provider="zai" (M3) + glm/zai-glm tokens
internal/config/file_test.go                 # 2 TOML-parse fixtures (L59/90/91 + L786/800/801); L187/204/205 OUT OF SCOPE
internal/config/git_test.go                  # 3 lines (bare)
internal/config/load_test.go                 # 3 lines (bare)
internal/cmd/default_action_test.go          # 5 lines (L1500 bare error; L1532-1541 FR51b label lockstep)
internal/cmd/config_init_interactive_test.go # 2 lines ONLY (L589/593); gpt-5.4* lines OUT OF SCOPE
internal/cmd/providers_test.go               # 3 lines (double→single quote lockstep)
# DO NOT TOUCH:
#   P2.M1.T1.S1/S2 code+docs (manifest.go, render.go, roles.go, pi.toml, builtin.go, bootstrap.go, config.go,
#       config_init_interactive.go, default_action.go, README, docs/*) — runtime surface
#   P2.M1.T2.S1 provider-subsystem tests (render_test.go, manifest_test.go, merge_test.go, builtin_test.go,
#       registry_test.go, roles_test.go, realagent_test.go, output_test.go, multiturn_test.go) — Category B sibling
#   file_test.go L187/204/205 (field-merge sentinel, model "A")
#   config_init_interactive_test.go L105/107/119/120 (zai/gpt-5.4* — different family)
#   migrate_test.go case 6 + config_test.go single-backend claude sub-test (claude provider, opus, no glm/zai token)
#   spec/** (human-owned — AGENTS.md rule 1), PRD.md, plan/, tasks.json
```

### Desired Codebase tree with files to be added/edited

```bash
# NONE added. Edits to the 8 existing test files above. No new files, no code, no docs.
```

### Known Gotchas of our codebase & Library Quirks

```text
# CRITICAL (M3 — the bare default_provider=zai is MANDATORY, not optional): the contract's acceptance grep
# (zai/glm|glm-5\.2|glm-5-turbo) does NOT match a bare `default_provider = "zai"` (no /glm). But the v2→v3 fold
# concatenates default_provider + '/' + default_model, so leaving "zai" produces `zai/claude-haiku` ≠ the
# `anthropic/claude-haiku` assertion → go test FAILS. You MUST rename every bare default_provider "zai" → "anthropic"
# (migrate_test.go L23/41/56/69/86/166; config_test.go L1349/1359/1362/1382/1402/1430/1483/1508). The correctness
# grep `rg -n 'zai' config_test.go migrate_test.go` = zero is the proof. The contract grep alone gives a false pass.

# CRITICAL (out-of-scope sentinel — file_test.go L187/204/205): TestOverlayProvidersFieldMerge uses
# `"pi": {"default_model": "A", "default_provider": "zai"}` — a provider-table MERGE test where the model is the
# sentinel "A" (NOT glm) and "zai" is an incidental value proving lower-layer fields survive a merge. The acceptance
# grep doesn't match it (no glm). OUT OF SCOPE — a blind `default_provider.*zai`→anthropic replace would corrupt it.

# CRITICAL (out-of-scope family — config_init_interactive_test.go L105/107/119/120): these use zai/gpt-5.4,
# zai/gpt-5.4-mini, zai/gpt-5.4-nano — a DIFFERENT model family. The acceptance grep doesn't match them. OUT OF
# SCOPE. A blind global zai→anthropic / glm→claude replace would corrupt them. (Exact analog of P2.M1.T2.S1's
# zai/gpt-5.4* guard in decompose/roles_test.go.) Edit ONLY L589 + L593 in this file.

# CRITICAL (out-of-scope provider — migrate_test.go case 6 + config_test.go single-backend claude sub-test): these
# use the `claude` provider with pre-existing `default_provider:"anthropic"` + `default_model:"opus"`. NO glm/zai
# token; the acceptance grep doesn't match. OUT OF SCOPE. (The pre-existing "anthropic" there is incidental to the
# claude single-backend illustration, unrelated to the pi fold.)

# GOTCHA (bare stays bare — the no-fold cases): migrate_test.go cases 7/8/9/11/12 + config_test.go L1456-1461 are the
# FR-B7 no-invent illustrations — a model with NO default_provider MUST stay bare. glm-5.2 → claude-haiku (bare);
# do NOT prefix them, or you invert the test's meaning ("no default_provider ⇒ no prefix invented").

# GOTCHA (lockstep): where an input and its assertion reference the same token, BOTH move:
#   - migrate_test.go fold cases: input default_provider:"zai"+Model:"glm-5.2" ↔ asserted cfg.Model=="zai/glm-5.2"
#     → anthropic + claude-haiku ↔ anthropic/claude-haiku.
#   - config_test.go: input TOML `default_provider = "zai"` / `model = "glm-5.2"` ↔ substring asserts
#     `default_model = "zai/glm-5.2"`, `# default_provider = "zai"`, AND the negative asserts `"\ndefault_provider =
#     \"zai\"\n"` (should be ABSENT) + `"/glm-5.2\""` (no prefix invented). Rename ALL on a line in lockstep.
#   - default_action_test.go: SetArgs --model glm-5.2 ↔ FR51b label '↳ Generating with glm-5.2 in stub…'.
#   - providers_test.go: TOML input (double quotes) ↔ rendered output assert (SINGLE quotes).
#   - config_init_interactive_test.go: override map ↔ written content assert.

# GOTCHA (quote style — providers_test.go): L226 input uses DOUBLE quotes ("glm-5.2"); L241/242 rendered-output
# asserts use SINGLE quotes ('glm-5.2'). Match each line's existing quoting — do NOT homogenize.

# GOTCHA (line numbers can drift): the contract/findings cite specific line numbers. RE-ANCHOR with the acceptance
# grep (Task 0) before editing — use TEXT anchors for the actual edits, not bare line numbers.

# GOTCHA (gofmt): the edits are within existing string/comment literals — gofmt -l should report nothing changed.
# If it does, run gofmt -w on that file.
```

## Implementation Blueprint

### Data models and structure
None — mechanical text-token rename in test fixtures. No production code, no schema, no behavior change.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: RE-ANCHOR (do this FIRST — line numbers drift)
  - RUN: rg -n "zai/glm|glm-5\.2|glm-5-turbo" internal/cmd/config_test.go internal/config/migrate_test.go \
            internal/config/file_test.go internal/config/git_test.go internal/config/load_test.go \
            internal/cmd/default_action_test.go internal/cmd/config_init_interactive_test.go \
            internal/cmd/providers_test.go
  - EXPECT: ~38 glm/zai-glm token-bearing LINES (see findings §4). ALSO run `rg -n 'default_provider.*"zai"' \
            internal/config/migrate_test.go internal/cmd/config_test.go` to enumerate the 14 M3 bare-zai lines
            (6 in migrate_test.go, 8 in config_test.go). If line numbers differ from the findings, trust the rg
            output — the TEXT anchors are what matter. Confirm the out-of-scope guards are PRESENT and will survive:
            `rg -n 'zai/gpt-5' internal/cmd/config_init_interactive_test.go` (L105/107/119/120) and
            `rg -n '"zai"' internal/config/file_test.go` (L187/204/205).

Task 1: EDIT internal/config/migrate_test.go (TestMigrateV2ToV3 — the fold unit tests)
  - For each of the 6 fold cases (1,2,3,4,5,10): rename the INPUT default_provider "zai"→"anthropic" (M3) AND the
    model glm-5.2→claude-haiku (M1) AND the asserted folded model zai/glm-5.2→anthropic/claude-haiku (M2). Per
    findings §4 migrate_test.go table. Key lines:
    * L22/26/27 model+asserts + L23 default_provider (case 1); L39/44/45 + L41 (case 2); L54/59/60 + L56 (case 3);
      L69/73/74 (case 4, both default_provider AND default_model in the same map literal); L85/86/89/90/92/93
      (case 5 idempotent — all zai/glm-5.2 → anthropic/claude-haiku, default_provider zai→anthropic); L164/169/170
      + L166 (case 10).
  - For the 5 bare/no-fold cases (7,8,9,11,12): rename glm-5.2→claude-haiku BARE ONLY (no default_provider change;
    L125 is "", L139 has no dp key, L147 nil, L171 empty map, L191 is int 42). Lines: L122/126/127, L138/142/143,
    L151/155/156, L178/182/183, L191/195/196.
  - DO NOT touch case 6 (L98-111, claude/opus/anthropic — no glm/zai token).
  - USE the edit tool with unique oldText anchors (each case is a unique struct literal + its wantFn).
  - VALIDATE: go test ./internal/config/ -run TestMigrateV2ToV3 -v

Task 2: EDIT internal/cmd/config_test.go (the on-disk TOML rewrite tests — substring asserts)
  - For each sub-test in findings §4 config_test.go table, rename in lockstep: every `default_provider = \"zai\"`
    (input + active-form + commented-form asserts) → `\"anthropic\"` (M3); every `glm-5.2` model input → `claude-haiku`
    (M1); every `zai/glm-5.2` folded assert → `anthropic/claude-haiku` (M2). Key lines:
    * L1349/1350 input + L1356 assert + L1359 NEG-assert + L1362 assert (folds provider default_model);
    * L1379/1382 input + L1384 assert (folds global model);
    * L1396/1399 input + L1402 input + L1405 Count assert (folds per-role model);
    * L1430/1431 input + L1439 assert (agent table renamed);
    * L1445 input+assert (idempotent v3 no-op);
    * L1456/1458 assert + **L1461 NEG-assert `"/glm-5.2\""`→`"/claude-haiku\""`** (bare no-fold);
    * L1480/1483 input + L1505/L1508 asserts (TestConfigUpgrade_V2ToV3Rewrite command round-trip).
  - DO NOT touch the single-backend claude sub-test (L1413-1431 region — uses claude/opus/anthropic, no glm/zai).
  - VALIDATE: go test ./internal/cmd/ -run 'TestUpgradeConfigVersion_V3Rewrite|TestConfigUpgrade_V2ToV3Rewrite' -v

Task 3: EDIT internal/config/file_test.go (2 TOML-parse fixtures ONLY)
  - L59 (TOML literal `default_model = "glm-5.2"`) + L90 (`!= "glm-5.2"`) + L91 (`want glm-5.2`) → claude-haiku.
  - L786 + L800 + L801 (second fixture, identical pattern) → claude-haiku.
  - DO NOT touch L187/204/205 (TestOverlayProvidersFieldMerge sentinel — OUT OF SCOPE).
  - VALIDATE: go test ./internal/config/ -run 'TestParseConfigFile|file' -v

Task 4: EDIT internal/config/git_test.go (3 lines, bare)
  - L76 setGitConfig "glm-5.2"→"claude-haiku"; L104 `!= "glm-5.2"`→`!= "claude-haiku"`; L105 `want glm-5.2`→`want claude-haiku`.
  - VALIDATE: go test ./internal/config/ -run 'git|GitConfig' -v

Task 5: EDIT internal/config/load_test.go (3 lines, bare)
  - L148 Setenv "glm-5.2"→"claude-haiku"; L161 `!= "glm-5.2"`→`!= "claude-haiku"`; L162 `want glm-5.2`→`want claude-haiku`.
  - VALIDATE: go test ./internal/config/ -run 'load|Load|Env' -v

Task 6: EDIT internal/cmd/default_action_test.go (5 lines — L1500 input-only; L1532-1541 FR51b lockstep)
  - L1500 SetArgs `--model glm-5.2`→`--model claude-haiku` (BARE — stays the FR-R5b error case; its assertion is the
    STATIC 'must be inference/model' — UNCHANGED). INPUT-ONLY.
  - L1532 SetArgs `--model glm-5.2`→`--model claude-haiku`; L1539 comment + L1540 Contains + L1541 Errorf
    '↳ Generating with glm-5.2 in stub…'→'↳ Generating with claude-haiku in stub…' (LOCKSTEP — the label embeds the
    --model value).
  - VALIDATE: go test ./internal/cmd/ -run 'ProgressLabel|BareModel|FR5b' -v

Task 7: EDIT internal/cmd/config_init_interactive_test.go (2 lines ONLY — TARGET glm, NOT gpt)
  - L589 `map[string]string{"planner": "zai/glm-5.2"}`→`{"planner": "anthropic/claude-haiku"}`;
    L593 `strings.Contains(content, \`model = "zai/glm-5.2"\`)`→`model = "anthropic/claude-haiku"`.
  - DO NOT touch L105/107/119/120 (zai/gpt-5.4* — DIFFERENT family, OUT OF SCOPE).
  - VALIDATE: go test ./internal/cmd/ -run 'BootstrapConfigWithOverrides|Interactive' -v

Task 8: EDIT internal/cmd/providers_test.go (3 lines — double→single quote lockstep)
  - L226 TOML input `default_model = "glm-5.2"`→`"claude-haiku"` (DOUBLE quotes);
    L241 `strings.Contains(got, "default_model = 'glm-5.2'")`→`'claude-haiku'` (SINGLE quotes);
    L242 `t.Error(\`…overridden "default_model = 'glm-5.2'"\`)`→`'claude-haiku'` (SINGLE quotes).
  - VALIDATE: go test ./internal/cmd/ -run 'TestProvidersShow' -v

Task 9: VALIDATE (see Validation Loop) — scoped tests green + go build + gofmt + acceptance grep zero + correctness
         grep zero + scope-preservation grep + scope guard.
```

### Implementation Patterns & Key Details

```text
# PATTERN (the fold-test lockstep — input + assertion move together; M3 is the key):
#   INPUT cfg:   Provider:"pi", Model:"glm-5.2", Providers:{"pi":{"default_provider":"zai"}}
#   ASSERT:      cfg.Model == "zai/glm-5.2"   (fold = default_provider + '/' + model)
#   → BOTH become anthropic/claude-haiku:
#   INPUT cfg:   Provider:"pi", Model:"claude-haiku", Providers:{"pi":{"default_provider":"anthropic"}}
#   ASSERT:      cfg.Model == "anthropic/claude-haiku"
# The fold MECHANISM (concat default_provider+'/'+model) is unchanged; only the example token moves.

# PATTERN (config_test.go substring asserts — input literal + assert substring + the negative asserts):
#   INPUT:    "default_provider = \"zai\"\n" + "default_model = \"glm-5.2\"\n"
#   ASSERT+:  strings.Contains(got, "default_model = \"zai/glm-5.2\"")
#   ASSERT-:  !strings.Contains(got, "\ndefault_provider = \"zai\"\n")   ← active form must be GONE
#   ASSERT+:  strings.Contains(got, "# default_provider = \"zai\"")       ← commented form must be PRESENT
#   → ALL four token occurrences (zai/glm) on these lines → anthropic/claude-haiku.

# PATTERN (bare stays bare — the no-fold/no-invent cases):
#   INPUT:   Model:"glm-5.2", Providers:{"pi":{"default_model":"glm-5.2"}}   (no default_provider key)
#   ASSERT:  cfg.Model == "glm-5.2"   (no prefix invented)
#   →       Model:"claude-haiku" ... cfg.Model == "claude-haiku"   (STILL bare — no slash)

# PATTERN (default_action FR51b label lockstep — the label embeds the --model value):
#   SetArgs: {"--provider","stub","--model","glm-5.2"}
#   ASSERT:  strings.Contains(errBuf, "↳ Generating with glm-5.2 in stub…")
#   → SetArgs: {"--provider","stub","--model","claude-haiku"}
#     ASSERT:  strings.Contains(errBuf, "↳ Generating with claude-haiku in stub…")
#   (The runtime label format `%s in %s` is unchanged by P2.M1.T1.)

# PATTERN (providers_test.go double→single quote lockstep):
#   INPUT (TOML):   default_model = "glm-5.2"      (double quotes)
#   ASSERT (render): Contains(got, "default_model = 'glm-5.2'")   (single quotes — `providers show` rendering)
#   → default_model = "claude-haiku"  /  Contains(got, "default_model = 'claude-haiku'")
```

### Integration Points

```yaml
# NONE (code/build/config/migration/behavior). This is a test-fixture token rename; no production change.
CONSUMES:
  - P2.M1.T1.S2's canonical token (anthropic/claude-haiku) + preserved STATIC 'must be inference/model' error tail
    (asserted unchanged by default_action_test.go:1500).
  - model_cleanup_scope.md Categories C + D (the 8 files) + the replacement mappings.
PRODUCES:
  - The 8 config-subsystem test files use anthropic/claude-haiku; the P2.M1 acceptance grep hits zero on them.
PARALLEL (non-conflicting):
  - P2.M1.T1.S1/S2 (code+docs) and P2.M1.T2.S1 (provider-subsystem tests, Category B) are on DISJOINT files —
    no merge conflict.
```

## Validation Loop

> **Go-repo test-fixture rename, NOT Python/ruff/mypy.** Use go test (scoped), go build, gofmt, the contract
> acceptance grep, and the M3 correctness grep. Run commands from the repo root.

### Level 1: Build & format (Immediate Feedback)

```bash
# Tests must still compile (the edits are string literals, but confirm no stray quote broke a file).
go build ./...
# Expected: exit 0, no output.

# Formatting check — must print NOTHING.
gofmt -l internal/cmd/config_test.go internal/config/migrate_test.go internal/config/file_test.go \
  internal/config/git_test.go internal/config/load_test.go internal/cmd/default_action_test.go \
  internal/cmd/config_init_interactive_test.go internal/cmd/providers_test.go
# Expected: empty. If a file is listed, run gofmt -w on it and re-check.
```

### Level 2: Scoped unit tests (the green gate — AUTHORITATIVE)

```bash
# The 2 packages whose tests were edited. This is the real correctness gate: a missed M3 (bare default_provider
# left as "zai") makes a fold produce zai/claude-haiku ≠ the anthropic/claude-haiku assertion → FAIL here.
go test ./internal/cmd/... ./internal/config/...
# Expected: all PASS, exit 0. (The rename is fixture-input + lockstep-assertion; the fold mechanism + the STATIC
# 'must be inference/model' assertion are unchanged → no behavioral change → green.)
```

### Level 3: Whole-repo test (System Validation)

```bash
# Confirm nothing else broke (the edits are isolated test literals, so this is a sanity gate).
go test ./... 2>&1 | tail -30
# Expected: no FAIL package. (If a package OUTSIDE the 2 edited fails, it is unrelated to this task — flag it,
# do not fix here. The provider-subsystem tests are P2.M1.T2.S1's scope.)
```

### Level 4: Acceptance grep (contract zero-hit) + M3 correctness grep + scope guards

```bash
# THE acceptance criterion: zero hits across the 8 edited test files.
rg -n "zai/glm|glm-5\.2|glm-5-turbo" internal/cmd/config_test.go internal/config/migrate_test.go \
   internal/config/file_test.go internal/config/git_test.go internal/config/load_test.go \
   internal/cmd/default_action_test.go internal/cmd/config_init_interactive_test.go \
   internal/cmd/providers_test.go
# Expected: NO output (zero hits). If any line remains, edit it per findings §4 and re-run.

# THE M3 CORRECTNESS GATE (the gap the contract grep misses): no bare zai (default_provider or otherwise) remains
# in the two fold-test files. If this is non-empty, a fold will emit zai/claude-haiku and go test will fail.
rg -n 'zai' internal/cmd/config_test.go internal/config/migrate_test.go
# Expected: NO output (zero hits). (Migrate case 6 + config single-backend claude use 'claude'/'opus'/'anthropic' —
# no 'zai' — so they correctly drop out too.)

# SCOPE-PRESERVATION guards (the out-of-scope occurrences survived the rename):
rg -n 'zai/gpt-5' internal/cmd/config_init_interactive_test.go   # → STILL L105/107/119/120 (gpt family untouched)
rg -n '"zai"' internal/config/file_test.go                        # → STILL L187/204/205 (field-merge sentinel untouched)
# Expected: both show their original lines — proving they were NOT corrupted by a blind replace.

# Scope guard: ONLY the 8 test files modified; no code/docs/provider-tests/spec touched.
git status --porcelain
# Expected: M on the 8 test files ONLY.
git status --porcelain | grep -vE '_test\.go' | grep -E '\.go$|\.md$|\.toml$|spec/|PRD|plan/|tasks\.json' \
  && echo "FAIL: out-of-scope file" || echo "OK: only test files touched"
# Expected: "OK: only test files touched".
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` exit 0
- [ ] `go test ./internal/cmd/... ./internal/config/...` green (the authoritative correctness gate)
- [ ] `gofmt -l <8 files>` prints nothing
- [ ] Contract acceptance grep (Level 4) over the 8 test files → zero hits
- [ ] **M3 correctness grep** `rg -n 'zai' internal/cmd/config_test.go internal/config/migrate_test.go` → zero hits

### Feature Validation
- [ ] All 8 files renamed per the 3 mappings (M1 bare glm→bare claude-haiku; M2 zai/glm-5.2→anthropic/claude-haiku;
      M3 bare default_provider "zai"→"anthropic"); every token-bearing line in findings §4 updated
- [ ] Input + token-bearing assertions changed IN LOCKSTEP (migrate fold cases, config_test substring asserts incl.
      the NEGATIVE asserts at L1359/L1461, default_action FR51b label, providers double→single quote, config_init)
- [ ] Bare/no-fold tokens stayed bare (migrate cases 7/8/9/11/12, config_test L1456-1461)
- [ ] The STATIC `must be inference/model` assertion (default_action_test.go:1500) UNCHANGED

### Scope-Boundary Validation
- [ ] `git status --porcelain` shows ONLY the 8 test files modified
- [ ] `zai/gpt-5.4*` lines in config_init_interactive_test.go (L105/107/119/120) UNTOUCHED (Level 4 grep proves it)
- [ ] `default_provider:"zai"` field-merge sentinel in file_test.go (L187/204/205) UNTOUCHED (Level 4 grep proves it)
- [ ] migrate_test.go case 6 + config_test.go single-backend claude sub-test (claude/opus/anthropic) UNTOUCHED
- [ ] NO P2.M1.T1 code/docs file touched; NO P2.M1.T2.S1 provider-subsystem test touched
- [ ] NO spec/**, PRD.md, plan/, tasks.json touched (human/orchestrator-owned)

### Documentation & Deployment
- [ ] Mode A: none required — this is test-fixture hygiene with no user/config/API surface change

---

## Anti-Patterns to Avoid

- ❌ Don't skip M3. The contract grep (`zai/glm|glm-5\.2|glm-5-turbo`) does NOT match bare `default_provider = "zai"`,
  so it gives a false pass — but the fold concatenates `default_provider+'/'+model`, so leaving "zai" makes the
  migration produce `zai/claude-haiku` and the `anthropic/claude-haiku` assertion FAILS at `go test`. Rename EVERY
  bare default_provider "zai" → "anthropic" (migrate_test.go L23/41/56/69/86/166; config_test.go L1349/1359/1362/
  1382/1402/1430/1483/1508). Prove it with `rg -n 'zai' config_test.go migrate_test.go` = zero.
- ❌ Don't run a blind global `zai`→`anthropic` or `glm`→`claude` replace. Three out-of-scope traps: file_test.go
  L187/204/205 (a `default_provider:"zai"` + model `"A"` field-merge sentinel), config_init_interactive_test.go
  L105/107/119/120 (`zai/gpt-5.4*` — a different family), and the claude-provider cases (migrate case 6, config_test
  single-backend sub-test — use claude/opus/anthropic, no glm/zai). The acceptance grep defines the glm/zai boundary;
  the scope guards prove the rest survived.
- ❌ Don't prefix a BARE/no-fold token. The no-invent cases (migrate 7/8/9/11/12, config_test L1456-1461) MUST stay
  bare (`glm-5.2`→`claude-haiku`) to keep illustrating "no default_provider ⇒ no prefix invented" (FR-B7).
- ❌ Don't change the STATIC `must be inference/model` assertion (default_action_test.go:1500). It is a substring
  preserved by P2.M1.T1.S2; only the bare `--model` input changes (input-only).
- ❌ Don't break the lockstep. Where an input and its assertion reference the same token (migrate fold cases,
  config_test substring asserts including the NEGATIVE asserts at L1359/L1461, default_action FR51b label,
  providers double→single quote, config_init override), BOTH must move — change one and the test fails.
- ❌ Don't homogenize quote styles in providers_test.go. L226 input is DOUBLE-quoted TOML; L241/242 rendered-output
  asserts are SINGLE-quoted. Match each line's existing quoting; change ONLY the model token.
- ❌ Don't edit any P2.M1.T1 code/docs file or any P2.M1.T2.S1 provider-subsystem test — those are disjoint sibling
  scopes. This task owns ONLY the 8 config-subsystem test files.
- ❌ Don't edit `spec/**`, `PRD.md`, `plan/`, or `tasks.json` — human/orchestrator-owned.
- ❌ Don't trust bare line numbers — re-anchor with the acceptance grep (Task 0) and use TEXT anchors.
- ❌ Don't run ruff/mypy/pytest — this is a Go repo. The gates are go test, go build, gofmt, and the greps.

---

## Confidence Score

**One-pass success likelihood: 9/10.** This is a deterministic test-fixture token rename pinned to a verified,
grep-enumerated inventory of every token-bearing line. The 3 mappings are unambiguous, the in-lockstep rule is
spelled out per site (including the negative assertions and the double→single-quote case), and the test-safety
guarantee (static vs renamed assertions) is proven. The 1-point deduction is for the THREE traps a careless edit
falls into — (a) the M3 bare-zai `default_provider` that the contract grep misses (defended by Task 1/2 + the L4
correctness grep `rg 'zai' config_test.go migrate_test.go`=zero), (b) the file_test.go field-merge sentinel, and
(c) the config_init_interactive `zai/gpt-5.4*` family — all of which the Critical Gotchas + the L4
scope-preservation greps explicitly defend against. No behavior, schema, or API change; the v2→v3 fold mechanism
the tests exercise is unchanged.