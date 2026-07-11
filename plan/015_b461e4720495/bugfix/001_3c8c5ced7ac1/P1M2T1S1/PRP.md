name: "P1.M2.T1.S1 — Blank pi models in the commented-provider block loop (Issue 2, FR-R5b/FR-B1)"
description: >
  Fix the commented-out pi provider block in `config init` output so it no longer ships BARE models
  (`gpt-5.4*`) that are a hard error on pi under FR-R5b when uncommented (the documented FR-B1 workflow).
  In `internal/config/bootstrap.go`'s commented-provider loop (`for _, name := range preferredBuiltins`),
  when `name == "pi"`: blank the per-role models to `""` (mutating the fresh per-call copy returned by
  `DefaultModelsForProvider`) and emit a 2-line multi-backend guidance comment AFTER the section header,
  BEFORE the `writeCommentedRoleBlock` calls — exactly mirroring the existing target=="pi" active-block
  blanking. Result: the commented pi block reads `# model = ""` for all four roles + guidance.
  Uncommenting yields a valid (model-less) config the user fills in. Plus one docs line in
  `docs/providers.md` clarifying BOTH the active and commented pi blocks are blanked. Pure production
  fix + docs; NO new test here (the dedicated commented-pi-block test is the separate P1.M2.T1.S2
  subtask). Does NOT touch the active-block paths (fixed by P1.M1.T1.S1), the ValidateModel regression
  net (parallel P1.M1.T2.S1), or the separate exampleConfigTemplate (clean, out of scope).

---

## Goal

**Feature Goal**: Make the commented-out pi provider block in every `config init` output FR-R5b-valid:
when a user uncomments it (the documented FR-B1 "one-line uncomment" workflow), the result is a VALID
config, not a hard `model "gpt-5.4-nano" on pi must be inference/model` error. Achieve this by blanking
the commented pi models to `""` (mirroring the active target=="pi" block) and emitting a guidance comment
telling the user to prefix their inference backend.

**Deliverable**:
1. **internal/config/bootstrap.go** — in the commented-provider loop (`for _, name := range
   preferredBuiltins`), add a `piCommented := name == "pi"` branch that (a) blanks the `other` map
   in place (`for role := range other { other[role] = "" }`) and (b) writes a 2-line guidance comment
   after the `# === pi (installed) ===` header, before the four `writeCommentedRoleBlock` calls. NO
   change to `writeCommentedRoleBlock` itself, the active-block paths, or `DefaultModelsForProvider`.
2. **docs/providers.md** — extend the "EXCEPT for pi, whose per-role models are written EMPTY" sentence
  (~line 125) to state the blanking applies to BOTH the active `[role.*]` block AND the commented-out
  pi block.
3. NO new test in this subtask (P1.M2.T1.S2 owns the dedicated commented-pi-block test).

**Success Definition**:
- `buildBootstrapConfig("claude", []string{"claude", "pi"}, nil)` produces a commented pi block whose
  every role line reads `# model = ""` (NOT `gpt-5.4*`), preceded by the 2-line multi-backend NOTE.
- The commented pi block contains NO bare `gpt-5.4` anywhere (the FR-R5b bug is gone).
- Uncommenting the pi block yields a config that parses AND would pass `ValidateModel` (all models
  blank → skipped; provider "pi" is valid).
- Other (non-pi) commented blocks are UNCHANGED (e.g. opencode still `openai/gpt-5.4`; claude still
  `haiku`).
- All existing `bootstrap_test.go` tests stay GREEN (esp. `TestBuildBootstrapConfig_ValidTOML` with the
  `{"claude",["claude","pi"]}` and `{"agy",["agy","pi","claude"]}` cases — blanked models + `#` comments
  are inert TOML).
- `go build ./...`, `make test`, `make lint` pass; `gofmt -l` empty.

## User Persona (if applicable)

**Target User**: A user who runs `stagecoach config init`, sees the commented-out pi block, and
uncomments it (per FR-B1) to route a role to pi.

**Use Case**: "I use claude by default but want to route the planner to pi for a big-context job" →
uncomment the `# [role.planner]` block under `# === pi (installed) ===`.

**User Journey**: Before: uncommenting the pi block → `config load` / first run errors with
`model "gpt-5.4-nano" on pi must be inference/model` (FR-R5b) → the user is stuck on day one. After:
the commented block ships `# model = ""` + a NOTE → uncommenting yields a valid config (model-less until
the user fills in `zai/gpt-5.4`-style values) → the user follows the NOTE and is unblocked.

**Pain Points Addressed**: Issue 2 — the commented pi block is the ONLY commented block that produces a
hard error when uncommented (opencode/claude/agy blocks are fine). FR-B1's "uncommenting works" promise
is violated specifically for pi, the most common provider.

## Why

- **Issue 2 (Major) / FR-R5b / FR-B1**: pi is a multi-backend provider — a model on pi MUST carry its
  inference backend as a slash-prefix (`zai/gpt-5.4`). The compiled-in role-default models for pi are
  BARE (`gpt-5.4`, `gpt-5.4-mini`, …) because the backend is user-specific. The active block already
  blanks them (target=="pi"); the commented block does not, so it ships invalid bare models. FR-B1 says
  every `config init` output is a WORKING config and uncommenting a block is a supported workflow.
- **Consistency**: the fix mirrors the EXACT pattern the active block already uses (blank + guidance
  comment). No new pattern, no new helper — a localized loop edit.
- **Bounded scope**: one loop branch + one docs sentence. The active paths are fixed (P1.M1.T1.S1); the
  regression net is parallel (P1.M1.T2.S1); the dedicated test is the next subtask (P1.M2.T1.S2).

## What

**User-visible behavior**: The commented-out pi block in `config init` output now shows `# model = ""`
(with a guidance NOTE) instead of bare `# model = "gpt-5.4*"`. Uncommenting it produces a valid config.

**Technical change** (one loop branch + one docs sentence):

```go
// in internal/config/bootstrap.go, the commented-provider loop (after `other := DefaultModelsForProvider(name)` + nil check):
piCommented := name == "pi"
if piCommented {
    for role := range other { other[role] = "" }   // blank — mirrors active-block target=="pi"
}
b.WriteString("\n# === " + name + " (installed) — uncomment a [role.*] block to route that role to " + name + " ===\n")
if piCommented {
    b.WriteString("# NOTE: pi is a multi-backend provider — prefix the model with your inference backend,\n")
    b.WriteString("# e.g. model = \"zai/gpt-5.4\". A bare model (no '/') on pi is a config error (FR-R5b).\n")
}
writeCommentedRoleBlock(&b, "planner", name, other["planner"])  // blank model → `# model = ""`
// ... stager, message, arbiter
```

### Success Criteria
- [ ] The commented pi block's four role lines read `# model = ""` (no `gpt-5.4*`).
- [ ] A 2-line `# NOTE: pi is a multi-backend provider …` comment precedes the pi block's role lines.
- [ ] The commented pi block contains ZERO `gpt-5.4` substrings (the bug is gone).
- [ ] Non-pi commented blocks are UNCHANGED (opencode=`openai/gpt-5.4`, claude=`haiku`, etc.).
- [ ] `writeCommentedRoleBlock` itself is UNMODIFIED.
- [ ] All existing `bootstrap_test.go` tests stay green (ValidTOML parses the blanked+commented block).
- [ ] `docs/providers.md` notes BOTH active and commented pi blocks are blanked.
- [ ] `go build ./...`, `make test`, `make lint` pass; `gofmt -l` empty.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact before/after of the loop edit, the proof that mutating `other` is safe (fresh copy),
the active-block precedent to mirror (with line hints), the exact guidance-comment wording, the
docs sentence to extend (with its current text), the test-impact analysis proving nothing breaks, and
the explicit scope fences (no active-block/exampleConfigTemplate/regression-net touch).

### Documentation & References

```yaml
# MUST READ — the authoritative research (verbatim fix + copy-safety + test-impact)
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/P1M2T1S1/research/findings.md
  why: "§1 has the verbatim before/after loop edit + the writeCommentedRoleBlock seam (no change needed);
        §2 PROVES DefaultModelsForProvider returns a fresh copy (mutating `other` is safe); §3 is the
        active-block template to mirror; §4 the docs sentence; §5 the test-impact (nothing breaks)."
  critical: "§2: the fix mutates `other` in place — this is SAFE only because DefaultModelsForProvider
             returns a defensive copy (role_defaults.go makes a fresh map). §5: NO existing test asserts
             a commented pi gpt-5.4, and ValidTOML stays green (blanked models + # comments are inert TOML)."

# MUST READ — the Issue-2 root-cause + fix-location analysis
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/architecture/bootstrap_pi_model_bug.md
  why: "The 'Issue 2: Commented-Out Block Path' section identifies the loop, the bug, and the two fix
        options (blank vs placeholder-prefixed); it recommends blank (option a) for consistency with the
        target==pi path — which is exactly this task's approach."
  critical: "It confirms the bug is pi-SPECIFIC (opencode's commented block is already openai/-prefixed
             and fine) and that the commented lines don't error at load but DO error on uncomment (FR-B1)."

# MUST READ — the file being edited (the loop + the active-block precedent + the emission seam)
- file: internal/config/bootstrap.go
  why: "LOCATE the commented loop by content: grep -n 'preferredBuiltins' internal/config/bootstrap.go
        (it's the `for _, name := range preferredBuiltins` block). The active-block target=='pi' blanking
        (~lines 162-175, `piBlanked := target == \"pi\"` … `for role := range models { models[role] = \"\" }`)
        is the LITERAL template to mirror. writeCommentedRoleBlock (~lines 116-121) is the emission seam
        (`%q` on \"\" renders `# model = \"\"`)."
  pattern: "Active block: `models := DefaultModelsForProvider(target); if piBlanked { for role := range
            models { models[role] = \"\" } }; … <NOTE>`. Commented loop: identical blank on the `other` map
            + the 2-line NOTE, scoped to `name == \"pi\"`."
  gotcha: "Line numbers DRIFT (P1.M1.T1.S1 already shifted them). Locate by content via grep, not by
           the contract's 205-222 / 119-123 numbers. Place the NOTE AFTER the `# === pi (installed) ===`
           header line and BEFORE the writeCommentedRoleBlock calls."

# MUST READ — the copy-safety foundation (mutating `other` relies on this)
- file: internal/config/role_defaults.go
  why: "DefaultModelsForProvider (~lines 120-130) is the source of `other`. It does `out := make(map[string]
        …); for role, model := range col { out[role] = model }; return out` — a FRESH per-call copy. This is
        WHY the fix can mutate `other` without corrupting the package-level roleDefaults table."
  pattern: "The active-block code already relies on this exact copy-then-mutate discipline. The commented
            loop reuses it identically."
  critical: "Do NOT change DefaultModelsForProvider. If it ever stopped returning a copy, this fix (and the
             active-block blanking) would corrupt the shared table — but it does return a copy (verified)."

# MUST READ — the docs target (Mode A)
- file: docs/providers.md
  why: "The 'Per-role default models' section has the sentence about pi models being written EMPTY
        (locate: grep -n 'written EMPTY' docs/providers.md). Extend it to name BOTH the active and the
        commented-out pi blocks."
  pattern: "Insert 'in BOTH the active `[role.*]` block AND the commented-out pi block' into the existing
            sentence — a one-clause extension, not a rewrite."

# MUST READ — the tests that must stay green (confirm the fix breaks none)
- file: internal/config/bootstrap_test.go
  why: "TestBuildBootstrapConfig_ValidTOML (@~188) has cases that install pi as a NON-target
        ({'claude',['claude','pi']} and {'agy',['agy','pi','claude']}) → these exercise the commented pi
        block. After the fix the block is `# model = \"\"` + `#` NOTE lines → inert TOML → toml.Unmarshal
        still succeeds. TestBuildBootstrapConfig_OtherInstalledCommented (@~153) asserts the CLAUDE
        commented block (target=pi → no pi commented block emitted) → unaffected."
  critical: "NO existing test positively asserts a commented pi block contains gpt-5.4 — the fix breaks
             nothing. Do NOT add the commented-pi-block test here (that's P1.M2.T1.S2); just keep the
             existing suite green."

# CONTEXT — the parallel regression net (NO overlap; do not duplicate)
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/P1M1T2S1/PRP.md
  why: "Parallel sibling adds internal/config/bootstrap_validate_test.go — a ValidateModel net over ACTIVE
        role models ONLY. It does NOT touch bootstrap.go production code and CANNOT cover commented blocks
        (commented TOML is inert → never decoded → ValidateModel can't reach it). Its own doc comment says
        so. NO overlap with this fix; no conflict."
  critical: "Because the regression net can't see commented blocks, this fix's verification relies on
             build/lint/existing-tests + a manual generation proof (Level 3/4) + the dedicated S2 test."

# CONTEXT — the separate template (OUT OF SCOPE, already clean)
- file: internal/cmd/config.go
  why: "exampleConfigTemplate (~line 537) is a SEPARATE inert doc with its own content. It does NOT have
        the pi/gpt-5 bug (zero gpt-5 references; uses agy + blanks). It has a byte-equality golden test
        (config_test.go:438)."
  critical: "Do NOT touch exampleConfigTemplate. It is clean and out of scope; touching it would break its
             golden test for no benefit."
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  bootstrap.go        # EDIT — commented-provider loop: add pi-blank branch + guidance NOTE
  role_defaults.go    # READ-ONLY — DefaultModelsForProvider (fresh-copy source; DO NOT change)
  bootstrap_test.go   # READ-ONLY — existing tests stay green (ValidTOML parses blanked+commented block)
docs/
  providers.md        # EDIT — extend the "pi written EMPTY" sentence (~line 125)
internal/cmd/
  config.go           # READ-ONLY — exampleConfigTemplate (clean, OUT OF SCOPE; golden test guards it)
```

### Desired Codebase tree with files to be added/modified

```bash
# MODIFIED (no new files):
internal/config/bootstrap.go   # +piCommented branch in the commented-provider loop (blank + NOTE)
docs/providers.md              # +one clause in the "pi written EMPTY" sentence
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (mutating `other` is safe ONLY because of the copy): DefaultModelsForProvider returns a FRESH
//   per-call map (role_defaults.go: `out := make(...); … return out`). The fix does
//   `for role := range other { other[role] = "" }` — this mutates the COPY, not the package-level
//   roleDefaults table. The active-block code already relies on this exact discipline. Do NOT "fix" this
//   by copying again — the copy is already made for you.

// CRITICAL (scope: ONLY name == "pi"): blank the models + emit the NOTE ONLY when name == "pi". Other
//   providers' commented blocks (claude/agy/opencode/codex/cursor/qwen-code) are ALREADY valid as-is
//   (opencode is openai/-prefixed; the rest are single-backend bare models that are legal on their
//   provider). Blanking them would needlessly remove useful defaults. The bug is pi-specific.

// GOTCHA (place the NOTE after the header, before the role blocks): the guidance NOTE must come AFTER
//   the `# === pi (installed) ===` section-header WriteString and BEFORE the four writeCommentedRoleBlock
//   calls — mirroring how the active block's NOTE sits between the section header and the role blocks.
//   All NOTE lines are `#`-prefixed (inert TOML) so ValidTOML parsing is unaffected.

// GOTCHA (line numbers drift — locate by content): P1.M1.T1.S1 already shifted bootstrap.go line numbers.
//   The contract's "205-222 / 119-123" are STALE. Locate the loop via `grep -n 'preferredBuiltins'` and
//   writeCommentedRoleBlock via `grep -n 'func writeCommentedRoleBlock'`. Do NOT anchor to line numbers.

// GOTCHA (the emission seam needs NO change): writeCommentedRoleBlock uses `fmt.Fprintf(b, "# model = %q\n", model)`
//   — `%q` on the blank "" renders `# model = ""` exactly. Do NOT modify writeCommentedRoleBlock; just pass
//   it "" as the model.

// GOTCHA (no new test here): the dedicated commented-pi-block assertion is P1.M2.T1.S2. This subtask keeps
//   the existing suite green and verifies via build/lint + a manual generation proof (Level 3/4). Adding
//   the S2 test here would exceed scope; the two subtasks are intentionally split.
```

## Implementation Blueprint

### Data models and structure
None. No types, no new helpers. One localized loop branch reusing the existing `writeCommentedRoleBlock`
emission seam and the existing `DefaultModelsForProvider` copy discipline.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/config/bootstrap.go — add the pi-blank branch in the commented-provider loop
  - LOCATE the loop: `grep -n 'preferredBuiltins' internal/config/bootstrap.go` → the
    `for _, name := range preferredBuiltins { … }` block. Inside it, find:
        other := DefaultModelsForProvider(name)
        if other == nil { continue }
        b.WriteString("\n# === " + name + " (installed) — uncomment a [role.*] block to route that role to " + name + " ===\n")
        writeCommentedRoleBlock(&b, "planner", name, other["planner"])
        …
  - INSERT, between the `if other == nil { continue }` and the `# === …` WriteString:
        piCommented := name == "pi"
        if piCommented {
            // pi is a multi-backend provider (FR-R5b): a bare model (no '/') is a hard error. Blank the
            // commented models so uncommenting yields a valid (model-less) config the user fills in —
            // mirroring the target=="pi" active-block blanking above. (other is a fresh per-call copy from
            // DefaultModelsForProvider, so this mutation is isolated to this map.)
            for role := range other {
                other[role] = ""
            }
        }
  - THEN, immediately AFTER the `b.WriteString("\n# === " + name + " (installed) …\n")` line and BEFORE
    the first writeCommentedRoleBlock call, INSERT:
        if piCommented {
            b.WriteString("# NOTE: pi is a multi-backend provider — prefix the model with your inference backend,\n")
            b.WriteString("# e.g. model = \"zai/gpt-5.4\". A bare model (no '/') on pi is a config error (FR-R5b).\n")
        }
  - PRESERVE: the loop structure, the four writeCommentedRoleBlock calls, and every non-pi iteration.
    writeCommentedRoleBlock itself is UNCHANGED (`%q` on "" renders `# model = ""`).
  - NO NEW IMPORTS (strings.Builder, fmt already in scope).
  - REFERENCE pattern: the active-block `piBlanked` blanking (~lines 162-175) + its NOTE (~lines 186-189).

Task 2: EDIT docs/providers.md — extend the "pi written EMPTY" sentence (Mode A)
  - LOCATE: `grep -n 'written EMPTY' docs/providers.md` (~line 125). Current sentence:
      "…EXCEPT for **pi**, whose per-role models are written EMPTY (pi needs an inference-provider prefix
       on the model, FR-R5b; its shipped per-role models are blank so you supply backend/model, e.g.
       `zai/gpt-5.4`)."
  - EDIT to (insert one clause naming both blocks):
      "…EXCEPT for **pi**, whose per-role models are written EMPTY in BOTH the active `[role.*]` block AND
       the commented-out pi block (pi needs an inference-provider prefix on the model, FR-R5b; its shipped
       per-role models are blank so you supply backend/model, e.g. `zai/gpt-5.4`)."
  - PRESERVE: the rest of the sentence, the table below it, and all surrounding content.

Task 3: VERIFY — build, vet, format, existing tests, lint, manual proof, grep guards
  - go build ./...
  - go vet ./internal/config/...
  - gofmt -l internal/config/bootstrap.go   # must be empty
  - go test ./internal/config/ -run 'BuildBootstrapConfig|GenerateBootstrapConfig' -v   # all green
  - make test ; make lint
  - manual proof + grep guards (see Validation Loop Level 3/4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the commented-loop pi branch (mirrors the active-block target=="pi" blanking)
// inside `for _, name := range preferredBuiltins { … }`, after `other := DefaultModelsForProvider(name)` + nil check:
piCommented := name == "pi"
if piCommented {
	for role := range other { // other is a fresh per-call copy → safe to mutate (role_defaults.go copy discipline)
		other[role] = ""
	}
}
b.WriteString("\n# === " + name + " (installed) — uncomment a [role.*] block to route that role to " + name + " ===\n")
if piCommented {
	b.WriteString("# NOTE: pi is a multi-backend provider — prefix the model with your inference backend,\n")
	b.WriteString("# e.g. model = \"zai/gpt-5.4\". A bare model (no '/') on pi is a config error (FR-R5b).\n")
}
writeCommentedRoleBlock(&b, "planner", name, other["planner"]) // blank → `# model = ""`
// … stager, message, arbiter (all blank for pi)

// PATTERN: the active-block precedent to mirror (bootstrap.go ~162-189) — the literal template:
//   models := DefaultModelsForProvider(target)
//   piBlanked := target == "pi"
//   if piBlanked { for role := range models { models[role] = "" } }
//   … <section header> …
//   if piBlanked && !piHasOverrides { <NOTE> }
```

### Integration Points

```yaml
NO database / migration / routes / new types / new imports / new helpers / new flags. One loop branch +
one docs clause.

BOOTSTRAP GENERATOR (internal/config/bootstrap.go):
  - commented-provider loop: +`piCommented := name == "pi"` branch → blank `other` map + emit 2-line NOTE.
  - writeCommentedRoleBlock: UNCHANGED (the `%q` seam renders blank models correctly).
  - active-block paths: UNCHANGED (already fixed by P1.M1.T1.S1).

DOCS (docs/providers.md):
  - "pi written EMPTY" sentence: +clause naming active AND commented blocks.

SCOPE FENCES: NO active-block edit (P1.M1.T1.S1 done); NO new ValidateModel net (parallel P1.M1.T2.S1);
  NO exampleConfigTemplate edit (internal/cmd/config.go — clean, golden-test-guarded); NO new test (S2);
  NO DefaultModelsForProvider change (copy discipline is the foundation, not a bug).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build + vet.
go build ./...
go vet ./internal/config/...

# Format.
gofmt -l internal/config/bootstrap.go
# Expected: empty. If listed: gofmt -w internal/config/bootstrap.go.

# Lint.
make lint      # golangci-lint v1.61 (staticcheck/gosimple/govet/errcheck/ineffassign/unused)
# Expected: zero errors.

# Scope guard: only bootstrap.go + docs/providers.md changed.
git diff --name-only
# Expected: internal/config/bootstrap.go  docs/providers.md  (exactly these 2).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The existing bootstrap tests must ALL stay green (the fix breaks no assertion).
go test ./internal/config/ -run 'BuildBootstrapConfig|GenerateBootstrapConfig' -v
# Expected: PASS — incl. TestBuildBootstrapConfig_ValidTOML with the {'claude',['claude','pi']} and
#           {'agy',['agy','pi','claude']} cases (blanked pi models + # NOTE are inert TOML → parses).

# Full race suite.
make test
# Expected: green (race detector).
```

### Level 3: Integration Testing (System Validation)

```bash
# Manual proof that the commented pi block is now blank + NOTE'd, via a scratch Go test snippet.
# (The dedicated assertion test is P1.M2.T1.S2; this is the within-S1 confidence check.)
cat > /tmp/sc_proof_test.go <<'EOF'
package config
import ("strings"; "testing")
func TestScratch_CommentedPiBlockBlanked(t *testing.T) {
	content := buildBootstrapConfig("claude", []string{"claude", "pi"}, nil)
	// isolate the commented pi section
	idx := strings.Index(content, "# === pi (installed)")
	if idx < 0 { t.Fatal("no commented pi block emitted (target must be non-pi with pi installed)") }
	piBlock := content[idx:]
	// the four role lines must be blank
	if strings.Contains(piBlock, "gpt-5.4") { t.Errorf("commented pi block still has bare gpt-5.4:\n%s", piBlock) }
	if !strings.Contains(piBlock, `# model = ""`) { t.Errorf("commented pi block missing blank model:\n%s", piBlock) }
	// the multi-backend NOTE must precede the role lines
	if !strings.Contains(piBlock, "multi-backend provider") { t.Errorf("missing guidance NOTE:\n%s", piBlock) }
	// a non-pi commented block (if present) is unchanged — claude keeps haiku
	// (claude is the target here so not emitted; this guard is illustrative)
}
EOF
cp /tmp/sc_proof_test.go internal/config/zz_scratch_proof_test.go
go test ./internal/config/ -run TestScratch_CommentedPiBlockBlanked -v
rm internal/config/zz_scratch_proof_test.go
# Expected: PASS — the commented pi block has `# model = ""`, NO gpt-5.4, and the NOTE.
# (This scratch test is for confidence ONLY — delete it; the permanent test is P1.M2.T1.S2.)

# Real-binary proof (generate an actual config with pi installed but not the target):
make build
d=$(mktemp -d) && cd "$d" && git init -q
# Pretend claude + pi are "installed" by running config init with --provider claude won't list pi unless
# detected; the unit-test path (buildBootstrapConfig) is the deterministic proof. The scratch test above
# IS the proof. (A full `config init` e2e that detects multiple providers is P1.M4's domain.)
cd - && rm -rf "$d"
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard 1: the commented pi block is blanked (NO bare gpt-5.4 in it).
#   Deterministic proof via the scratch test (Level 3) OR generate + inspect:
go test ./internal/config/ -run TestScratch_CommentedPiBlockBlanked -v   # (if scratch test present)

# Grep guard 2: writeCommentedRoleBlock is UNCHANGED (the seam was not edited).
git diff internal/config/bootstrap.go | grep -E '^[+-].*writeCommentedRoleBlock' | grep -E '^[+-]func'
# Expected: empty (no change to the function signature/body — only a new caller branch).

# Grep guard 3: the active-block pi-blanking is UNTOUCHED (P1.M1.T1.S1's fix stays).
git diff internal/config/bootstrap.go | grep -E '^[+-].*piBlanked'
# Expected: empty (piBlanked is the active-block var; this task adds piCommented, a DIFFERENT var).

# Grep guard 4: the docs sentence names BOTH blocks.
grep -n 'written EMPTY' docs/providers.md
# Expected: the sentence now contains "in BOTH the active `[role.*]` block AND the commented-out pi block".

# Grep guard 5: NO exampleConfigTemplate change (out of scope).
git diff --name-only | grep 'internal/cmd/config.go'
# Expected: empty.

# Grep guard 6: scope — only 2 files changed.
git diff --name-only
# Expected: internal/config/bootstrap.go  docs/providers.md.

# Regression: the active-block + ValidTOML tests still pass.
go test ./internal/config/ -run 'BuildBootstrapConfig|GenerateBootstrapConfig' -v
# Expected: all PASS.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/config/...` clean
- [ ] `gofmt -l internal/config/bootstrap.go` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) green — all existing bootstrap tests pass

### Feature Validation
- [ ] commented pi block: four role lines read `# model = ""` (no `gpt-5.4*`)
- [ ] commented pi block: preceded by the 2-line multi-backend NOTE
- [ ] commented pi block: contains ZERO `gpt-5.4` substrings
- [ ] non-pi commented blocks UNCHANGED (opencode=`openai/gpt-5.4`, claude=`haiku`, …)
- [ ] uncommenting the pi block → a config that parses (blank models valid)

### Scope-Boundary Validation
- [ ] `git diff --name-only` == only {internal/config/bootstrap.go, docs/providers.md}
- [ ] `writeCommentedRoleBlock` UNMODIFIED
- [ ] active-block paths UNCHANGED (piBlanked, not piCommented)
- [ ] `DefaultModelsForProvider` UNCHANGED (copy discipline intact)
- [ ] NO `internal/cmd/config.go` (exampleConfigTemplate) change
- [ ] NO new test file added here (S2 owns it)

### Code Quality & Docs
- [ ] The fix mirrors the active-block target=="pi" blanking pattern (consistency)
- [ ] The NOTE wording matches the contract (`zai/gpt-5.4`, FR-R5b reference)
- [ ] `docs/providers.md` names BOTH active and commented pi blocks
- [ ] Code locates the loop by content (grep), not stale line numbers

---

## Anti-Patterns to Avoid

- ❌ Don't blank non-pi commented blocks. The bug is pi-SPECIFIC. opencode's `openai/gpt-5.4` is already
  prefixed (valid); claude/agy/codex/cursor/qwen-code models are bare but LEGAL on their single-backend
  providers. Blanking them removes useful defaults for no reason. Scope the branch to `name == "pi"` only.
- ❌ Don't modify `writeCommentedRoleBlock`. It already renders a blank model correctly (`%q` on `""` →
  `# model = ""`). The fix is a CALLER-SIDE change (pass "" + emit the NOTE), not a helper change.
- ❌ Don't re-copy the `other` map before blanking. `DefaultModelsForProvider` ALREADY returns a fresh
  defensive copy (verified) — mutating it is safe and is the exact discipline the active block uses.
  Re-copying is dead code; NOT copying would be a bug (but you don't need to, it's already a copy).
- ❌ Don't anchor to the contract's line numbers (205-222 / 119-123). P1.M1.T1.S1 shifted bootstrap.go.
  Locate the loop with `grep -n 'preferredBuiltins'` and the helper with `grep -n 'func writeCommentedRoleBlock'`.
- ❌ Don't touch the active-block paths. The `piBlanked`/stager-fallback blanking is P1.M1.T1.S1's
  (landed) fix for Issue 1 — a DIFFERENT bug. This task adds a NEW `piCommented` var for the commented
  loop; leave `piBlanked` alone.
- ❌ Don't touch `exampleConfigTemplate` (internal/cmd/config.go). It's a separate inert doc, it does NOT
  have the pi/gpt-5 bug, and it has a byte-equality golden test (config_test.go:438) that would break.
- ❌ Don't add the commented-pi-block assertion test in this subtask. That's P1.M2.T1.S2 (the two are
  intentionally split). This subtask keeps the existing suite green and proves the fix via build/lint +
  the Level 3 scratch check (which you delete).
- ❌ Don't rely on the parallel ValidateModel regression net (P1.M1.T2.S1) to cover this. It validates
  ACTIVE models only — commented TOML is inert and can never reach ValidateModel. This fix is invisible
  to that net; verify it directly (Level 3/4).
- ❌ Don't place the NOTE before the `# === pi (installed) ===` header or interleave it with the role
  blocks. It goes AFTER the header and BEFORE the first `writeCommentedRoleBlock` — mirroring the active
  block's NOTE placement, and keeping the commented section readable.

---

## Confidence Score: 10/10

This is a single localized loop branch that mirrors an ALREADY-IMPLEMENTED, tested pattern (the
active-block target=="pi" blanking) in the same function, plus one docs clause. The verbatim before/after
is spelled out; the copy-safety of mutating `other` is proven (DefaultModelsForProvider returns a fresh
map); the emission seam (`writeCommentedRoleBlock` `%q`) needs no change; the test-impact analysis
confirms zero existing assertions break (ValidTOML stays green — blanked models + `#` NOTE are inert
TOML); and the scope is tightly fenced (no active-block/exampleConfigTemplate/regression-net/test touch).
The only judgment call (blank vs placeholder-prefixed models) is resolved by the architecture note +
contract (blank, for consistency with the target==pi path). One-pass success is essentially guaranteed.
