name: "P2.M1.T1.S1 — Verify code-side removal of the gemini built-in"
description: |
  Read-only verification subtask. Confirm the `gemini` (Gemini CLI) built-in provider is fully
  purged from the compiled code surface (manifest set, registry preference list, role-defaults
  table, and reference TOML files). Produce a one-line PASS/FAIL per contract item (a)–(d).
  No code change is expected; the architecture audit already confirmed all four PASS.

---

## Goal

**Feature Goal**: Verify — by direct inspection and by exercising the existing regression-guard
tests — that the EOL `gemini` built-in provider has zero presence anywhere in stagecoach's
compiled code surface, matching PRD §12.5 ("Gemini CLI ~~removed~~ — superseded by agy, §12.5.1").

**Deliverable**: A four-line verification result, one PASS/FAIL line per contract item (a)–(d),
backed by the cited file:line evidence and a green build + targeted test run. If any item fails
(UNEXPECTED — the audit confirmed all pass), restore the removed state per §"Restore procedure".

**Success Definition**: All four items (a)–(d) report PASS; `go build ./...` exits 0;
`go test ./internal/provider/... ./internal/config/...` reports `ok` for both packages; the
known test-fixture uses of the string `"gemini"` (opaque config-merge data) are explicitly
confirmed as NON-drift and left untouched.

## User Persona (if applicable)

**Target User**: The stagecoach maintainer / verifying engineer (and the orchestrator consuming the
verification result).

**Use Case**: Lock in confidence that the gemini removal is complete and will not silently regress,
before sibling tasks (P2.M1.T1.S2 PRD-side stub, P2.M2 agy re-verification, P2.M3 docs drift)
declare the provider-lineup correction done.

**User Journey**: Run the verification commands → read the four file:line anchors → run the
regression-guard tests → emit PASS/FAIL per item → report.

**Pain Points Addressed**: Eliminates ambiguity about whether "removed from PRD" also means
"removed from code"; proves the compiled surface and the shipped reference files agree with the
7-provider built-in set.

## Why

- **Correctness of the provider lineup (PRD §12.5/§12.5.1)**: `gemini` was EOL'd and superseded by
  `agy` on 2026-06-18. A residual gemini built-in, registry entry, role-defaults column, or TOML
  file would resurrect a dead provider and contradict the shipped agy successor.
- **Defense against silent regression**: Four independent locations must stay gemini-free. Three of
  them are guarded by tests; this subtask confirms both the locations AND their guards.
- **Foundation for the P2 milestone**: P2.M2 (agy re-verify), P2.M3 (docs drift) both assume the
  gemini purge is complete in code. This subtask is the gate.

## What

A pure read-only verification. For each of four contract items, confirm the stated invariant holds
in the committed codebase at HEAD, then emit one PASS/FAIL line. The exact expected state:

- **(a)** `internal/provider/builtin.go` `BuiltinManifests()` returns **exactly seven** entries —
  `pi`, `claude`, `agy`, `qwen-code`, `opencode`, `codex`, `cursor` — and **no** `gemini` key.
  No `builtinGemini` function exists anywhere. No `Name: "gemini"` entry anywhere.
- **(b)** `internal/provider/registry.go:15` `preferredBuiltins` equals
  `["pi", "opencode", "cursor", "agy", "qwen-code", "codex", "claude"]` with **no** `gemini`.
- **(c)** `providers/gemini.toml` does NOT exist; `providers/` contains **exactly 7** TOML files.
- **(d)** `internal/config/role_defaults.go` has **no** `gemini` key in the `roleDefaults` map.

**NON-DRIFT (leave alone)**: Test fixtures in `internal/config/*_test.go` and
`internal/cmd/models_test.go` that use `"gemini"` as an **opaque provider-name string** and
`"gemini-2.5-pro"` / `"gemini-2.5-flash"` as **opaque model strings** are NOT drift — they test
config field-merge / precedence mechanics with arbitrary strings, not the provider registry. Do
not rename them.

### Success Criteria

- [ ] Item (a) PASS: `BuiltinManifests()` has exactly the 7 named keys; no `builtinGemini`; no
      `Name: "gemini"`.
- [ ] Item (b) PASS: `preferredBuiltins` matches the exact 7-entry FR-D1 slice with no `gemini`.
- [ ] Item (c) PASS: `providers/` has exactly 7 TOML files and no `gemini.toml`.
- [ ] Item (d) PASS: `roleDefaults` has no `gemini` key.
- [ ] `go build ./...` exits 0.
- [ ] `go test ./internal/provider/... ./internal/config/...` reports `ok` for both packages.
- [ ] The opaque-string `"gemini"` test fixtures are explicitly noted as non-drift and unchanged.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to verify this
successfully?_ **Yes** — every check is pinned to an exact file path + line range, a copy-pasteable
grep/ls command, AND the regression-guard test that enforces it. No inference required.

### Documentation & References

```yaml
# MUST READ — the audit that pre-confirmed all four checks PASS (read-only reference)
- docfile: plan/013_b8a415cc6e79/architecture/code_gemini_agy_audit.md
  why: Authoritative read-only audit confirming checks 1-4 PASS; documents the non-drift
       test fixtures and historical plan-doc references.
  section: "Check 1" (builtin.go), "Check 2" (registry.go), "Check 3" (role_defaults.go),
           "Check 4" (providers/*.toml), "Residual observations" (non-drift note)

# PRD sections that define the removed/superseded state (selected for this work item)
- url: PRD.md#12.5  # §12.5 Gemini CLI — REMOVED (superseded by agy, §12.5.1)
  why: States gemini is "no longer shipped"; manifest + reference file + role-tier defaults removed.
- url: PRD.md#12.5.1  # §12.5.1 agy — the Gemini-CLI successor (the replacement built-in)
  why: Confirms agy is the lineage's current surface; the gemini built-in must not coexist.
- url: PRD.md#12.5.2  # §12.5.2 qwen-code — a Gemini-CLI fork (the 7th built-in)
  why: One of the seven expected built-in keys (verifies the SET, not just the absence of gemini).
- url: PRD.md#9.16  # §9.16 Default provider & per-role model defaults (FR-D1/D4)
  why: Defines preferredBuiltins FR-D1 order and the roleDefaults table that item (d) checks.
- url: PRD.md#12.8  # §12.8 user-defined providers
  why: Explains why user [provider.<name>] blocks are NOT relevant here — only the compiled built-in set.

# Code locations under verification (all verified at HEAD, fd99358)
- file: internal/provider/builtin.go
  why: Item (a). BuiltinManifests() func at :18; return-map literal :19-26 lists the 7 factories.
  pattern: Each entry is `"<name>": builtin<Name>()`; the set MUST be {pi,claude,opencode,codex,cursor,agy,qwen-code}.
  gotcha: "gemini" appears in COMMENTS only (lineage/model-label prose) — comment hits are NOT drift.

- file: internal/provider/registry.go
  why: Item (b). preferredBuiltins declared at :15; the exact FR-D1 slice.
  pattern: `var preferredBuiltins = []string{"pi","opencode","cursor","agy","qwen-code","codex","claude"}`
  gotcha: This slice is enforced (count + exact order) by TestPreferredBuiltins_MatchesBuiltinKeys.

- file: internal/config/role_defaults.go
  why: Item (d). `var roleDefaults` at :52; 7 provider keys at :53,59,65,73,79,85,91.
  pattern: `RoleModelDefaults` map keyed provider → {planner,stager,message,arbiter} → model.
  gotcha: Keys are pi(:53), claude(:59), agy(:65), qwen-code(:73), opencode(:79), codex(:85), cursor(:91). NO gemini.

- file: providers/  (directory)
  why: Item (c). Reference TOML files shipped per built-in.
  pattern: exactly agy.toml, claude.toml, codex.toml, cursor.toml, opencode.toml, pi.toml, qwen-code.toml (7).
  gotcha: Enforced by TestProviderReferenceFiles_AllBuiltinsCovered (builtin↔file parity, both directions).

# Regression-guard tests (the "how we know it stays removed" layer)
- file: internal/provider/builtin_test.go
  why: TestBuiltinManifests_KeysAndCount (:209) asserts len==7 + exact key set — FAILS if gemini re-added.
- file: internal/provider/registry_test.go
  why: TestPreferredBuiltins_MatchesBuiltinKeys (:15) asserts count + exact FR-D1 order.
- file: internal/provider/referencefiles_test.go
  why: TestProviderReferenceFiles_AllBuiltinsCovered (:68) asserts builtin↔reference-file parity.
```

### Current Codebase tree (relevant slice)

```bash
internal/provider/
  builtin.go              # item (a) — BuiltinManifests() + 7 builtin*() factories
  registry.go             # item (b) — preferredBuiltins at :15
  builtin_test.go         # guard: TestBuiltinManifests_KeysAndCount (:209)
  registry_test.go        # guard: TestPreferredBuiltins_MatchesBuiltinKeys (:15)
  referencefiles_test.go  # guard: TestProviderReferenceFiles_AllBuiltinsCovered (:68)
internal/config/
  role_defaults.go        # item (d) — roleDefaults map (7 keys, no gemini) at :52
  load_test.go            # NON-DRIFT: opaque "gemini" string fixtures (config-merge mechanics)
providers/
  agy.toml  claude.toml  codex.toml  cursor.toml  opencode.toml  pi.toml  qwen-code.toml  # item (c) — 7 files, NO gemini.toml
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
# NONE. This is a read-only verification subtask. No files are created, modified, or deleted.
# The only artifact is the verification result (PASS/FAIL per item), reported back.
```

### Known Gotchas of our codebase & Library Quirks

```go
// GOTCHA 1 — "gemini" string hits are NOT all drift.
//   grep -rn gemini internal/ will return hits in COMMENTS (builtin.go lineage prose) and in
//   TEST FIXTURES (internal/config/load_test.go uses "gemini" as an opaque provider-name string and
//   "gemini-2.5-pro"/"gemini-2.5-flash" as opaque model strings to test config field-merge).
//   These are CORRECT and MUST be left alone. Drift = a compiled gemini PROVIDER (manifest key,
//   preferredBuiltins entry, roleDefaults key, or providers/gemini.toml file) — NONE of which exist.

// GOTCHA 2 — BuiltinManifests() builds the map FRESH each call (no package-level var).
//   Verifying "7 entries" means reading the returned map literal in the source (:19-26),
//   NOT searching for a package-level slice. len(BuiltinManifests()) == 7 at runtime (test-asserted).

// GOTCHA 3 — cursor is the only built-in where Detect ≠ Name (Detect="agent").
//   This is unrelated to gemini but explains why a naive `grep -r agent` is noisy; do not confuse
//   cursor's "agent" binary with any provider removal.

// GOTCHA 4 — preferredBuiltins order is significant (FR-D1), not just membership.
//   The exact order ["pi","opencode","cursor","agy","qwen-code","codex","claude"] is test-asserted;
//   "no gemini" alone is insufficient — confirm the exact 7-element slice.
```

## Implementation Blueprint

### Verification approach (not "implementation" — read-only)

There are no data models to create. The "tasks" below are verification steps. Each step has an
exact command and an exact expected result; a step PASSES iff the observed result equals the
expected result. Emit the one-line PASS/FAIL verdict for the corresponding contract item.

### Verification Tasks (ordered; each maps to a contract item)

```yaml
Task V0: CONFIRM environment / baseline
  - RUN: `cd /home/dustin/projects/stagecoach && git rev-parse HEAD`
  - EXPECT: a commit at-or-after the removal commit `010ecee` ("Remove gemini-cli provider...").
            (Contract cites `cdbccf5`; current HEAD `fd99358` is a descendant — the removal persists.)
  - RUN: `go build ./...`
  - EXPECT: exit 0, no output. (If this fails, STOP — the tree does not build; report FAIL for all.)

Task V1 → item (a): CONFIRM builtin.go has exactly 7 builtins, no gemini
  - RUN: `grep -n 'func BuiltinManifests' -A9 internal/provider/builtin.go`
  - EXPECT: func at :18; return-map with EXACTLY these 7 lines: pi, claude, opencode, codex, cursor,
            agy, qwen-code (no gemini line).
  - RUN: `grep -rn 'func builtinGemini' internal/`
  - EXPECT: no match (empty output).
  - RUN: `grep -rn 'Name:.*"gemini"' internal/ providers/`
  - EXPECT: no match (empty output).
  - RUN (runtime proof): `go test ./internal/provider/ -run TestBuiltinManifests_KeysAndCount -v`
  - EXPECT: PASS (asserts len==7 AND exact key set; would FAIL if gemini re-added).
  - VERDICT: item (a) PASS iff all four above hold.

Task V2 → item (b): CONFIRM registry.go:15 preferredBuiltins has no gemini
  - RUN: `sed -n '15p' internal/provider/registry.go`
  - EXPECT: `var preferredBuiltins = []string{"pi", "opencode", "cursor", "agy", "qwen-code", "codex", "claude"}`
            (verbatim; 7 entries; no gemini).
  - RUN (runtime proof): `go test ./internal/provider/ -run TestPreferredBuiltins_MatchesBuiltinKeys -v`
  - EXPECT: PASS (asserts count + EXACT FR-D1 order; would FAIL if gemini re-added or order changed).
  - VERDICT: item (b) PASS iff both above hold.

Task V3 → item (c): CONFIRM providers/ has exactly 7 TOML files, no gemini.toml
  - RUN: `ls -1 providers/*.toml | wc -l`
  - EXPECT: 7
  - RUN: `ls providers/gemini.toml`
  - EXPECT: "No such file or directory" (non-zero exit).
  - RUN: `ls -1 providers/*.toml`
  - EXPECT: agy.toml claude.toml codex.toml cursor.toml opencode.toml pi.toml qwen-code.toml
  - RUN (runtime proof): `go test ./internal/provider/ -run TestProviderReferenceFiles_AllBuiltinsCovered -v`
  - EXPECT: PASS (asserts every builtin has a reference file AND vice-versa; would FAIL if gemini
            re-added without a file, or a stray gemini.toml reappeared without a builtin).
  - VERDICT: item (c) PASS iff all four above hold.

Task V4 → item (d): CONFIRM role_defaults.go roleDefaults has no gemini key
  - RUN: `grep -n 'var roleDefaults' -A45 internal/config/role_defaults.go | grep '^\s*"'`
  - EXPECT: exactly 7 map keys: "pi", "claude", "agy", "qwen-code", "opencode", "codex", "cursor"
            (at lines :53, :59, :65, :73, :79, :85, :91 respectively). NO "gemini" key.
  - RUN: `grep -n 'gemini' internal/config/role_defaults.go`
  - EXPECT: no match (empty output).
  - RUN (runtime proof): `go test ./internal/config/...`
  - EXPECT: ok (the config package compiles + passes; roleDefaults has no gemini reference).
  - VERDICT: item (d) PASS iff all three above hold.

Task V5: CONFIRM non-drift fixtures are left alone (sanity, NOT a FAIL trigger)
  - RUN: `grep -rln '"gemini"' internal/config/*_test.go internal/cmd/models_test.go`
  - EXPECT: matches in internal/config/load_test.go (and possibly others). These are OPAQUE STRINGS
            testing config field-merge mechanics — they are NOT drift. Do NOT edit them.
  - VERDICT: informational only. Their presence is EXPECTED and correct.
```

### Restore procedure (only if an item UNEXPECTEDLY fails)

The architecture audit confirmed all four PASS, so restoration should never be needed. If an item
fails (e.g., a gemini reference reappeared), restore the removed state per the removal commits:

```bash
# The removal is anchored in two commits (inspect, do NOT blanket-revert unrelated changes):
#   010ecee  "Remove gemini-cli provider, switch opencode to stdin delivery"  (code removal)
#   cdbccf5  "Purge gemini-cli from PRD after built-in removal"               (PRD purge)
# Restore by re-applying the code-side removal from 010ecee for the specific failing location:
#   - internal/provider/builtin.go        : delete any re-added "gemini" map entry + builtinGemini()
#   - internal/provider/registry.go:15     : ensure preferredBuiltins has exactly the 7-entry slice above
#   - internal/config/role_defaults.go     : delete any re-added "gemini" roleDefaults column
#   - providers/gemini.toml                : delete the file if it reappeared
# Then re-run Validation Levels 1-2 to confirm green.
# NOTE: a failure here indicates an upstream regression and should be REPORTED, not silently fixed.
```

### Integration Points

```yaml
# NONE. Read-only verification — no DATABASE, CONFIG, or ROUTES integration.
# The only "integration" is consuming the verification result in the P2 milestone reporting.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Read-only verification, so there is nothing new to lint. Confirm the tree still builds:
cd /home/dustin/projects/stagecoach
go build ./...                     # EXPECT: exit 0, no output
go vet ./internal/provider/... ./internal/config/...   # EXPECT: exit 0, no output

# Expected: Zero errors. If go build fails, the tree is broken — report and STOP.
```

### Level 2: Unit Tests (Regression-Guard Validation)

```bash
# The three tests that GUARD the gemini removal — run them targeted, then the full affected packages.
cd /home/dustin/projects/stagecoach

# Targeted guards (each would FAIL if gemini reappeared in its domain):
go test ./internal/provider/ -run TestBuiltinManifests_KeysAndCount -v           # item (a) guard
go test ./internal/provider/ -run TestPreferredBuiltins_MatchesBuiltinKeys -v   # item (b) guard
go test ./internal/provider/ -run TestProviderReferenceFiles_AllBuiltinsCovered -v  # item (c) guard

# Full affected packages (covers item (d) via config-package compile + test):
go test ./internal/provider/... ./internal/config/... -v   # EXPECT: ok for both packages

# Expected: All PASS. If any guard fails, an item has regressed — see "Restore procedure".
```

### Level 3: Integration Testing (System Validation)

```bash
# Full repo test suite + race detector (the Makefile `test` target) — confirms nothing else broke.
cd /home/dustin/projects/stagecoach
go test ./... 2>&1 | tail -40        # EXPECT: all packages ok / no FAIL
make test 2>&1 | tail -20            # equivalent: go test -race ./...

# Expected: All packages pass. (This is a no-code-change verification; a failure indicates a
# pre-existing or environmental issue, not this subtask's work.)
```

### Level 4: Domain-Specific Validation (final grep sweep)

```bash
# Negative-space sweep: confirm NO compiled gemini PROVIDER surface remains.
cd /home/dustin/projects/stagecoach

# A built-in manifest entry, a registry preference, a role-defaults column, OR a TOML file would
# each be drift. The strings below target the PROVIDER surface; comment/fixture hits are expected
# and are NOT failures (see Known Gotchas #1).
grep -rn 'func builtinGemini' internal/                      # EXPECT: empty
grep -rn '"gemini":' internal/provider/builtin.go            # EXPECT: empty (map key)
grep -rn '"gemini"' internal/provider/registry.go            # EXPECT: empty (preferredBuiltins)
grep -n 'gemini' internal/config/role_defaults.go            # EXPECT: empty (roleDefaults key)
ls providers/gemini.toml 2>/dev/null                         # EXPECT: empty (file absent)

# Expected: all five commands produce no output (every gemini PROVIDER surface is gone).
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./internal/provider/... ./internal/config/...` exits 0.
- [ ] `go test ./internal/provider/... ./internal/config/...` reports `ok` for both packages.
- [ ] `go test ./...` (or `make test`) reports no FAIL package.

### Feature (Verification) Validation

- [ ] Item (a) PASS — exactly 7 builtins {pi,claude,opencode,codex,cursor,agy,qwen-code}; no
      `builtinGemini`; no `Name: "gemini"`; `TestBuiltinManifests_KeysAndCount` PASS.
- [ ] Item (b) PASS — `preferredBuiltins` is the exact 7-entry FR-D1 slice with no gemini;
      `TestPreferredBuiltins_MatchesBuiltinKeys` PASS.
- [ ] Item (c) PASS — `providers/` has exactly 7 TOML files; no `gemini.toml`;
      `TestProviderReferenceFiles_AllBuiltinsCovered` PASS.
- [ ] Item (d) PASS — `roleDefaults` has no `gemini` key (7 keys: pi,claude,agy,qwen-code,opencode,codex,cursor).
- [ ] Four-line PASS/FAIL verdict emitted (one per item a–d).
- [ ] Opaque-string `"gemini"` test fixtures explicitly confirmed non-drift and left unchanged.

### Code Quality Validation

- [ ] No source files were modified (read-only verification).
- [ ] No test files were modified (opaque-string fixtures left as-is).
- [ ] No `providers/*.toml` files were added, removed, or modified.

### Documentation & Deployment

- [ ] No user-facing/config/API surface change (per contract §5: DOCS = none).
- [ ] Verification result recorded for the P2 milestone (sibling tasks P2.M1.T1.S2 onward depend on it).

---

## Anti-Patterns to Avoid

- ❌ Don't "fix" the `"gemini"` string in test fixtures — they are opaque config-merge data, NOT drift.
- ❌ Don't treat comment-only `gemini` hits in `builtin.go` (lineage/model-label prose) as drift.
- ❌ Don't re-add `gemini` "to be safe" — it is intentionally EOL'd and superseded by `agy` (§12.5.1).
- ❌ Don't skip the regression-guard tests — they are the durable proof the removal sticks; a passing
  grep today is not enough if the guard test would not catch a future regression.
- ❌ Don't modify any source/PRD/tasks.json file — this is a read-only verification subtask; the only
  output is the PASS/FAIL verdict (plus this PRP + its research notes).

---

## Confidence Score

**One-pass success likelihood: 10/10.** This is a read-only verification of code that a prior
architecture audit already confirmed PASS, with every check pinned to exact file:line anchors and
backed by three named regression-guard tests, all of which were re-confirmed green during research
(`go build ./...` exit 0; `go test ./internal/provider/... ./internal/config/...` both `ok`). The
deliverable is a deterministic PASS/FAIL verdict per item; there is no implementation surface to get
wrong.
