---
name: "P1.M1.T1.S4 — Update binary build paths in test code and verify compilation (stagehand→stagecoach)"
description: |
  VERIFICATION-FIRST subtask (Layer 1.6 of the rename surface map). After S1's module-path sed, the test
  code that builds the stagecoach/stubagent binaries via `exec.Command("go"/goPath, "build", "-o", out,
  "<import-path>")` must be CONFIRMED to reference `github.com/dustin/stagecoach/cmd/...` (not stagehand),
  and the whole tree must `go build ./...` + `go vet ./...` clean.

  ⚠️ RESEARCH VERDICT: S1's import-path sed ALREADY FIXED all 4 build-path import-path string literals
  (verified in-tree): stubtest.go:59, signal_integration_test.go:143 + :167, harness_test.go:65 are all
  `github.com/dustin/stagecoach/cmd/{stubagent,stagecoach}`. ZERO `github.com/dustin/stagehand` import-path
  literals remain in any .go file. So S4 is primarily a VERIFICATION gate + ONE concrete edit:

  THE ONE CONCRETE EDIT: `cmd/stubagent/main.go:1` godoc comment — `"...for Stagehand's integration/property
  tests"` → `"...for Stagecoach's..."` (explicitly named by the contract).

  ⚠️ SCOPE BOUNDARY (the #1 risk is OVER-REACH): the named test files contain MANY other `stagehand`
  residues that are NOT S4's. They are owned by sibling subtasks and MUST NOT be touched here:
    - IDENTIFIERS (`buildStagehand`, `stagehandBin`, `stagehandOnce`, `runStagehand`, …) → P1.M1.T2
    - `.stagehand.toml` / `stagehand.toml` config-filename literals → P1.M2.T2 (Layer 2.3)
    - `STAGEHAND_RUN_REAL` env var → P1.M2.T1 (Layer 2.1)
    - temp-dir prefixes (`stagehand-stubagent-*` etc.) → P1.M2.T3 (Layer 3.5)
    - error/status strings (`"go build stagehand: …"`) → P1.M2.T3 (Layer 3.2/3.3)
    - temp binary names (`name := "stagehand"`) → P1.M2.T3 / P1.M3.T1 (Layer 3.1/4.5)
  Touching those here collides with their owners and breaks the rename_surface_map's layering discipline.

  S4 depends on S3 landing (pkg/stagecoach package decl + default_action.go qualifiers). NO docs (M4);
  NO identifiers (T2); NO config/env/path strings (M2); NO Makefile/.goreleaser/CI (M3).
---

## Goal

**Feature Goal**: Confirm (and, where still needed, fix) that every test-code path which builds a stagecoach
or stubagent binary references the renamed module `github.com/dustin/stagecoach`, so that the FULL project
compiles and vets cleanly under the new module path — AND do this WITHOUT colliding with the sibling
subtasks (T2 identifiers, M2 config surface) that own the other `stagehand` residues in the same files.

**Deliverable**: (1) A grep-verified proof that all 4 build-path import-path string literals are
`stagecoach` (zero `github.com/dustin/stagehand` import-path literals remain). (2) ONE concrete edit:
`cmd/stubagent/main.go:1` comment `Stagehand's` → `Stagecoach's`. (3) A green `go build ./...` +
`go vet ./...` (the compilation gate — the real acceptance criterion). No functional code change beyond
the one comment.

**Success Definition**: `go build ./...` exits 0; `go vet ./...` exits 0; `grep -rn 'github.com/dustin/
stagehand' --include='*.go' .` (excluding plan/) returns ZERO matches; the 4 build-path literals read
`stagecoach`; `cmd/stubagent/main.go:1` reads `Stagecoach's`. `git diff --stat` shows ONLY
`cmd/stubagent/main.go` changed (the build-path literals were already correct — S1's work — so they are
NOT in S4's diff). The residual `stagehand` refs in the named test files (identifiers, `.stagehand.toml`,
temp prefixes, env vars) are UNTOUCHED (owned by T2/M2).

## User Persona

**Target User**: The contributor/reviewer confirming the Layer-1 structural Go rename (module + imports +
dirs + files + packages) is COMPLETE and the tree compiles end-to-end — the gate between "structural
rename done" (S1–S3) and "identifier + config-surface rename" (T2, M2).

**Use Case**: After S1 sed'd import paths, S2 renamed dirs, and S3 renamed files/packages, someone runs
`go build ./...` to confirm nothing is broken. S4 is that confirmation step: it proves the build-path
literals in test code (which `go build ./...` exercises when those tests compile/run) point at valid
import paths, and it fixes the one cosmetic comment the contract named.

**Pain Points Addressed**: Closes the gap between "S1 sed'd the imports" and "we have PROOF the whole tree
compiles." Without S4, a build-path literal S1's sed missed (e.g., one built from a variable, or a
`cmd/stagehand` subdir S1's module-prefix-only sed skipped) would silently break `go test` for the
integration/e2e/stub packages — and the breakage would surface late, mixed with T2/M2 work. S4 isolates
the build-path verification as its own atomic gate.

## Why

- **The Layer-1 rename's compilation gate.** S1–S3 did the structural rename (module path, import paths,
  directories, files, package declarations). S4 is the verification that the result COMPILES — specifically
  that the test code which shells out to `go build <import-path>` references real (renamed) import paths.
  It is the green light between Layer 1 and Layer 2/3 of the rename_surface_map.
- **S1's sed is TRUSTED but VERIFIED.** S1 did a module-path sed (`stagehand`→`stagecoach`). It almost
  certainly caught the build-path string literals (they contain the module path as a substring), but
  "almost certainly" is not a gate. S4 confirms it with a grep and runs the actual `go build ./...`. If S1
  missed one, S4 is where it's caught — not buried in a later subtask's failure.
- **Sharp scope boundary prevents collision.** The named test files are LADEN with `stagehand` residues
  (identifiers, config filenames, temp prefixes, env vars, status strings). Each belongs to a specific
  sibling subtask per rename_surface_map's layering. S4 touches ONLY Layer 1.6 (build-path import
  literals) + the one named comment. Over-reaching into T2/M2 territory would cause edit collisions and
  double-work. The layering discipline is the project's rename safety net; S4 honors it.
- **Lowest-risk gate.** One comment edit + verification commands. No logic, no identifiers, no config.

## What

Three things — verify, fix one comment, gate:

1. **VERIFY (grep)**: all `exec.Command(... "build" ... "<import-path>")` literals in test code reference
   `github.com/dustin/stagecoach/cmd/{stubagent,stagecoach}`. The 4 sites (verified in-tree at research
   time): `internal/stubtest/stubtest.go:59`, `internal/signal/signal_integration_test.go:143` + `:167`,
   `internal/e2e/harness_test.go:65`. Also confirm ZERO `github.com/dustin/stagehand` import-path
   literals remain anywhere in `.go` (excluding `plan/`).
2. **FIX (one edit)**: `cmd/stubagent/main.go:1` — the godoc comment `// Command stubagent is a tiny
   fake-agent binary for Stagehand's integration/property tests` → `Stagecoach's`. (The contract
   explicitly names this comment.)
3. **GATE**: `go build ./...` exits 0 AND `go vet ./...` exits 0. (Post-S3: `pkg/stagecoach` declares
   `package stagecoach` and `default_action.go` uses `stagecoach.` qualifiers, so the build is green.)

### Success Criteria

- [ ] `grep -rn 'github.com/dustin/stagehand' --include='*.go' . | grep -v './plan/'` → ZERO matches.
- [ ] The 4 build-path literals read `github.com/dustin/stagecoach/cmd/...` (stubtest.go:59,
      signal_integration_test.go:143/:167, harness_test.go:65).
- [ ] `default_action_test.go` has NO binary-build `exec.Command("go","build",...)` (it uses
      `stubtest.Build()` — confirm it needs no build-path edit).
- [ ] `cmd/stubagent/main.go:1` reads `// Command stubagent is a tiny fake-agent binary for Stagecoach's
      integration/property tests`.
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] ONLY `cmd/stubagent/main.go` is in `git diff --stat` (the build-path literals were already correct;
      they are NOT edited by S4).
- [ ] The residual `stagehand` refs in the named files (identifiers / `.stagehand.toml` / temp prefixes /
      `STAGEHAND_RUN_REAL` / status strings / temp binary names) are UNTOUCHED (owned by T2/M2/M3).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the EXACT current state of all 4 build-path literals (verified in-
tree, all already `stagecoach`), the exact `cmd/stubagent/main.go:1` before/after, the complete table of
residual `stagehand` refs in the named files WITH their owner subtasks (so the implementer knows precisely
what NOT to touch), and the exact grep + build/vet gate commands. The research verdict ("S1 already fixed
the functional build paths; S4 is verify + one comment + gate") is stated up front so the implementer does
not invent work. The S3 dependency (pkg/stagecoach package decl) is documented.

### Documentation & References

```yaml
# MUST READ — the rename surface map (the layering authority + the Layer 1.6 scope)
- docfile: plan/012_963e3918ec08/architecture/rename_surface_map.md
  why: "Layer 1.6 'Build commands in test code' is S4's exact scope: it lists stubtest.go:59,
        signal_integration_test.go:143/:167, default_action_test.go, harness_test.go. The map's layer
        headings (Layer 2 config, Layer 3 CLI strings, Layer 4 build/release) define what is OUT of scope:
        temp prefixes are Layer 3.5 (line 106-107), env vars Layer 2.1 (line 46), config file paths Layer
        2.3 (line 65), error/status Layer 3.2/3.3. The Execution Order (line 193-204) mandates Layer 1
        BEFORE Layer 2/3 — so S4 must NOT pull Layer 2/3 items forward."
  critical: "Layer 1.6 is the authority for WHAT S4 owns. Everything else in the named files (identifiers,
        .stagehand.toml, temp prefixes, STAGEHAND_RUN_REAL, status strings) is a DIFFERENT layer owned by
        a DIFFERENT subtask. The Verification Gates (line 208-214) define S4's acceptance: go build/vet
        clean after Layer 1."

# MUST READ — the parallel contract (S3 lands the package decl S4's build depends on)
- docfile: plan/012_963e3918ec08/P1M1T1S3/PRP.md
  why: "S3 (Implementing, parallel) renames pkg/stagecoach files + package declarations to `stagecoach`
        AND fixes default_action.go's `stagehand.`→`stagecoach.` qualifier refs (its CONTRACT CORRECTION).
        S4's `go build ./...` gate is GREEN only post-S3 (the package decl + qualifier must match). S3's
        scope discipline explicitly defers identifiers to T2 and strings to P1.M2 — S4 inherits the same
        boundaries."
  critical: "S4 RUNS AFTER S3. If `go build ./cmd/stagecoach/` fails with `undefined: stagehand`, S3 has
        NOT landed (its default_action.go qualifier sed is missing) — that is S3's bug, NOT S4's. Do NOT
        'fix' it in S4 by reverting the package name; flag it back to S3."

# MUST READ — S1's work (the import-path sed S4 verifies)
- docfile: plan/012_963e3918ec08/P1M1T1S1/PRP.md
  why: "S1 (Complete) renamed go.mod to github.com/dustin/stagecoach and sed'd all Go import-path prefixes.
        S4 is the verification that S1's sed caught the build-path STRING LITERALS (which contain the
        module path as a substring, so the sed should have got them — and DID, verified in-tree)."
  critical: "S1 is LANDED. S4 does NOT re-run S1's sed (no import-path edits). S4 only VERIFIES S1's result
        via grep + fixes the one named comment."

# The file under edit (the ONE concrete edit)
- file: cmd/stubagent/main.go
  why: "EDIT line 1 only: `// Command stubagent is a tiny fake-agent binary for Stagehand's
        integration/property tests` → `Stagecoach's`. The contract explicitly names this comment. The rest
        of the file (package main, the stub-agent logic) is unchanged."
  pattern: "A godoc comment line. `sed -i 's/Stagehand'\''s/Stagecoach'\''s/' cmd/stubagent/main.go` (or a
            precise edit). Verify no OTHER `stagehand`/`Stagehand` identifiers exist in the file (main.go is
            package main with a main() — unlikely; if found, they are T2 scope, defer them)."
  gotcha: "Only the COMMENT changes. Do not rename any identifier in main.go. If `grep -n stagehand
        cmd/stubagent/main.go` finds a var/func/const (not a comment), that is T2 — leave it and note it."

# The files S4 VERIFIES but does NOT edit (build-path literals already correct)
- file: internal/stubtest/stubtest.go
  why: "VERIFY line 59: `build := exec.Command(goPath, \"build\", \"-o\", stubPath,
        \"github.com/dustin/stagecoach/cmd/stubagent\")` — already stagecoach (S1). NO edit. The file's
        OTHER stagehand refs: L1 comment 'Stagehand's' (godoc — cosmetic, but NOT named by the contract;
        defer to M4 docs OR fix if trivial — see Gotchas), L49 temp prefix 'stagehand-stubagent-*' (Layer
        3.5 → M2.T3)."
  pattern: "The build-path literal is the ONLY Layer-1.6 element. grep-verify it; do not edit."
  gotcha: "stubtest.go:1 also has 'Stagehand's' in its package godoc. The contract names ONLY
        cmd/stubagent/main.go's comment. For consistency you MAY fix stubtest.go:1's 'Stagehand's'→
        'Stagecoach's' too (it's a parallel godoc comment, same one-token edit, no collision risk) — but it
        is OPTIONAL, not required by the gate. If in doubt, leave it for M4 (docs) to avoid scope creep."
- file: internal/signal/signal_integration_test.go
  why: "VERIFY lines 143 + 167: both `exec.Command(\"go\", \"build\", \"-o\", out,
        \"github.com/dustin/stagecoach/cmd/...\")` — already stagecoach (S1). NO edit. The file's OTHER
        residues (ALL out of scope): L30/154-155/158/171 identifiers `stagehandBin`/`buildStagehand`→T2;
        L47 `.stagehand.toml`→M2.T2; L137/161 temp prefixes→M2.T3; L152 comment `cmd/stagehand`+`buildStagehand`→T2;
        L165 temp binary name `name := \"stagehand\"`→M2.T3/M3.T1; L169 status string→M2.T3."
- file: internal/e2e/harness_test.go
  why: "VERIFY line 65: `\"github.com/dustin/stagecoach/cmd/stagecoach\"` — already stagecoach (S1). NO edit.
        The file's OTHER residues (ALL out of scope): L40-41/47/63/70 identifiers→T2; L44 comment
        `cmd/stagehand`+`buildStagehand`→T2; L5 `STAGEHAND_RUN_REAL`→M2.T1; L55 temp prefix→M2.T3; L59/61
        temp binary name→M2.T3/M3.T1; L67 status string→M2.T3; L173 `stagehand.toml`→M2.T2."
- file: internal/cmd/default_action_test.go
  why: "VERIFY it has NO binary-build exec.Command (it uses stubtest.Build(), confirmed by grep — no
        `go\", \"build\"` in the file). Its imports (L17-21) are already stagecoach (S1). NO edit. Its
        residues are ALL `.stagehand.toml` config-filename literals (L76-158)→M2.T2, plus test commit-msg
        strings like 'init: add stagehand config' (L104/134) — test-fixture data, defer to M2 or leave."

# PRD authority (already in the selected content)
- prd: PRD.md §14 (package layout — cmd/stagecoach + cmd/stubagent + pkg/stagecoach under module
        github.com/dustin/stagecoach); §h2.30 ("all references to stagehand must be replaced with
        stagecoach"). §15.1 (CLI synopsis — the binary is `stagecoach`).
  why: "§14 confirms the renamed layout S4 verifies against. §h2.30 is the rename mandate (but its
        execution is LAYERED — S4 is Layer 1.6 only; the full sweep completes at P1.M5.T2.S1)."
  critical: "§h2.30's 'all references' is the END STATE (P1.M5.T2.S1's grep audit), NOT S4's job. S4 is
        the narrow Layer-1.6 build-path verification. Do not attempt the full sweep here."
```

### Current Codebase Tree (relevant slice)

```bash
stagehand/                         # repo dir itself still named stagehand (rename is module-level, not repo-dir)
├── go.mod                         # module github.com/dustin/stagecoach  (S1)
├── cmd/
│   ├── stagecoach/main.go         # package main (S2 dir rename; S3 verified no edit needed)
│   └── stubagent/main.go          # EDIT: L1 comment Stagehand's → Stagecoach's
├── internal/
│   ├── stubtest/stubtest.go       # VERIFY L59 build literal (stagecoach ✓); L1/L49 = out of scope
│   ├── signal/signal_integration_test.go  # VERIFY L143/L167 (stagecoach ✓); rest = out of scope
│   ├── e2e/harness_test.go        # VERIFY L65 (stagecoach ✓); rest = out of scope
│   └── cmd/default_action_test.go # VERIFY no build literal; imports stagecoach ✓; .stagehand.toml = out of scope
└── pkg/stagecoach/                # S3: package stagecoach (files renamed)
```

### Desired Codebase Tree After S4

```bash
stagehand/
└── (only ONE file modified)
    cmd/stubagent/main.go          # L1 comment: Stagehand's → Stagecoach's
```

| Path | Action | Responsibility |
|---|---|---|
| `cmd/stubagent/main.go` | MODIFY (1 comment line) | L1 godoc `Stagehand's` → `Stagecoach's`. |
| (4 test files) | VERIFY ONLY (no edit) | Confirm build-path literals are `stagecoach`; confirm no edit needed. |

**Explicitly NOT touched**: all 4 named test files' bodies (the build-path literals are already correct;
their other residues are T2/M2/M3), `pkg/stagecoach/*` (S3), `internal/cmd/default_action.go` (S3),
`go.mod` (S1), any identifier, any `.stagehand.toml`/`stagehand.toml` literal, any `STAGEHAND_*` env var,
any temp-dir prefix, any status/error string, any Makefile/.goreleaser/CI, docs (M4), `PRD.md`,
`tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (S1 already did the functional work — S4 is VERIFY, not re-sed): all 4 build-path import-path
// string literals are ALREADY 'github.com/dustin/stagecoach/cmd/...' (verified in-tree at stubtest.go:59,
// signal_integration_test.go:143/167, harness_test.go:65). grep for 'github.com/dustin/stagehand' in .go
// (excl plan/) returns ZERO. Do NOT re-run S1's sed (no import-path edits). S4's only edit is the main.go
// comment. The build-path literals being already-correct is the EXPECTED state, not a sign S4 has nothing
// to do — the verification + the gate IS the deliverable.

// CRITICAL (the #1 risk is OVER-REACH into T2/M2): the named test files contain MANY 'stagehand' residues
// that are NOT Layer 1.6. Each has a specific owner:
//   buildStagehand / stagehandBin / stagehandOnce / runStagehand / buildStagehandPath  → P1.M1.T2 (identifiers)
//   .stagehand.toml / stagehand.toml (config filename literals)                        → P1.M2.T2 (Layer 2.3)
//   STAGEHAND_RUN_REAL (env var)                                                       → P1.M2.T1 (Layer 2.1)
//   "stagehand-stubagent-*" / "stagehand-stubtest-*" / "stagehand-inttest-*" / "stagehand-e2e-*" (temp prefixes) → P1.M2.T3 (Layer 3.5)
//   "go build stagehand: …" / "cannot build stagehand" (status/error strings)          → P1.M2.T3 (Layer 3.2/3.3)
//   name := "stagehand" / "stagehand.exe" (temp binary filenames)                      → P1.M2.T3 / P1.M3.T1 (Layer 3.1/4.5)
// Touching ANY of these in S4 collides with its owner subtask and breaks the rename_surface_map layering.
// The 2 'cmd/stagehand' COMMENT mentions (signal:152, harness:44) are attached to `buildStagehand`
// identifiers — they belong to T2 (T2 renames the function + fixes its doc comment as a unit). LEAVE THEM.

// CRITICAL (build/vet gate depends on S3 landing): `go build ./...` is GREEN only after S3 renames
// pkg/stagecoach's package declaration to `stagecoach` AND fixes default_action.go's `stagehand.`→`stagecoach.`
// qualifier refs. If the build fails with `undefined: stagehand` in default_action.go, S3 has NOT landed
// its CONTRACT CORRECTION — that is S3's bug, not S4's. Do NOT 'fix' it by reverting the package name.

// GOTCHA (default_action_test.go has NO build literal): the rename_surface_map Layer 1.6 lists it under
// 'binary build paths', but empirically it uses stubtest.Build() (no direct exec.Command("go","build")).
// Its only stagehand residues are .stagehand.toml config-filename literals (M2.T2) + test commit-msg
// strings. S4's verification of it = confirm it COMPILES (imports are stagecoach ✓); no edit.

// GOTCHA (the cmd/stubagent/main.go comment is the ONE named edit): the contract explicitly says 'Also
// check that cmd/stubagent/main.go comment referencing Stagehand is updated.' L1 currently reads
// '// Command stubagent is a tiny fake-agent binary for Stagehand's integration/property tests'. Change
// ONLY 'Stagehand's' → 'Stagecoach's'. Verify the rest of main.go has no stagehand IDENTIFIERS (grep);
// if it does, those are T2 — leave them.

// GOTCHA (stubtest.go:1 has a parallel 'Stagehand's' godoc comment): for CONSISTENCY you may also flip
// stubtest.go:1's 'Stagehand's'→'Stagecoach's' (same one-token godoc edit, no collision — it's a comment,
// not an identifier or config string). This is OPTIONAL (the contract names only main.go). If you prefer
// the strictest scope, leave stubtest.go:1 for M4 (docs). Either is defensible; do not let it block the gate.

// GOTCHA (the repo DIRECTORY is still named 'stagehand'): the working tree is /home/dustin/projects/
// stagehand — the on-disk repo dir was NOT renamed (the rename is module/import/dir-internal, not the
// repo root). This is expected; do not attempt to `mv` the repo dir. All paths in this PRP are relative
// to the repo root regardless of its name.
```

## Implementation Blueprint

### Data models and structure

None. S4 changes one comment line and runs verification commands. No types, no logic, no config.

### The EXACT before/after (the one concrete edit)

**`cmd/stubagent/main.go:1`** (the contract's named comment):

```go
// BEFORE:
// Command stubagent is a tiny fake-agent binary for Stagehand's integration/property tests

// AFTER:
// Command stubagent is a tiny fake-agent binary for Stagecoach's integration/property tests
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: VERIFY — all build-path import-path literals are stagecoach (S1's work)
  - RUN: grep -rn 'github.com/dustin/stagehand' --include='*.go' . | grep -v './plan/'
    → Expected: ZERO matches. (If ANY match, S1 missed an import-path literal — fix it: it is unambiguously
    a Layer 1.6 build-path/import literal, so S4 owns the fix. Replace stagehand→stagecoach in that literal.)
  - RUN: grep -rno '"github.com/dustin/stage[a-z]*/cmd[a-z/]*"' --include='*.go' . | grep -v './plan/'
    → Expected: exactly 4 matches, ALL '.../stagecoach/cmd/...':
        internal/stubtest/stubtest.go:59:"github.com/dustin/stagecoach/cmd/stubagent"
        internal/signal/signal_integration_test.go:143:"github.com/dustin/stagecoach/cmd/stubagent"
        internal/signal/signal_integration_test.go:167:"github.com/dustin/stagecoach/cmd/stagecoach"
        internal/e2e/harness_test.go:65:"github.com/dustin/stagecoach/cmd/stagecoach"
  - RUN: grep -rn '"build"' --include='*.go' . | grep -v './plan/' | grep -i command
    → Expected: the same 4 exec.Command build sites (catches both "go" and goPath variants). NO other build
    sites with a stagehand import path.
  - RUN: grep -n 'go", "build"\|goPath, "build"' internal/cmd/default_action_test.go
    → Expected: ZERO matches (default_action_test.go uses stubtest.Build(); no direct build literal).
  - VERDICT: if all green, S1 fully cleaned the build paths — S4's verification passes; no build-path edit
    is needed. (This is the EXPECTED outcome per research.)

Task 2: FIX — cmd/stubagent/main.go:1 comment (the one named edit)
  - EDIT cmd/stubagent/main.go line 1: 'Stagehand's' → 'Stagecoach's' (exact before/after above).
  - VERIFY no other stagehand IDENTIFIER exists in the file:
        grep -n 'stagehand\|Stagehand' cmd/stubagent/main.go
    → If the only hits are the L1 comment (now fixed) + maybe other COMMENTS, those comments are cosmetic
      (you may fix parallel 'Stagehand'→'Stagecoach' comments for consistency, OR defer to M4). If any hit
      is a var/func/const/type IDENTIFIER, that is T2 — LEAVE it and note it in the task report.
  - (OPTIONAL, for consistency) stubtest.go:1 has a parallel 'Stagehand's' godoc comment. You MAY flip it
    to 'Stagecoach's' (same one-token comment edit, no collision). Not required by the gate.

Task 3: GATE — go build ./... + go vet ./... MUST pass
  - RUN: go build ./...
    → Expected: exit 0. (Post-S3: pkg/stagecoach is package stagecoach, default_action.go uses stagecoach.
      qualifiers.) If it fails with 'undefined: stagehand' in default_action.go, S3 has NOT landed — STOP
      and flag to S3 (do not revert the package name).
  - RUN: go vet ./...
    → Expected: exit 0. (vet does not check comment accuracy; the build-path literals are valid import
      paths, so vet passes.)
  - (OPTIONAL CONFIDENCE) RUN: go test -count=1 ./internal/stubtest/ ./internal/signal/ ./internal/e2e/ ./internal/cmd/
    → Expected: compiles (may skip/run depending on env; the point is they COMPILE — the build-path
      literals resolve to real packages). If a test FAILS TO COMPILE citing a stagehand import path, S1
      missed it → fix per Task 1's fix-forward. (e2e/signal may need STAGEHAND_RUN_REAL unset → they use the
      stub; they should compile and pass with the stub.)

Task 4: SCOPE DISCIPLINE — confirm the residual stagehand refs are untouched + owned
  - RUN: grep -rn 'stagehand' --include='*.go' internal/stubtest/ internal/signal/ internal/e2e/ internal/cmd/default_action_test.go cmd/stubagent/ | wc -l
    → Expected: the residual count is UNCHANGED from pre-S4 (S4 edited only cmd/stubagent/main.go:1's
      'Stagehand's' token, so the count drops by exactly the number of 'Stagehand' tokens you fixed in
      main.go — typically 1). The remaining hits are identifiers/.stagehand.toml/temp-prefixes/env-vars/
      status-strings — ALL owned by T2/M2/M3. Confirm none of THOSE were edited.
  - CONFIRM: git diff --stat → ONLY cmd/stubagent/main.go (and optionally stubtest.go:1 if you took the
    consistency edit). NO test-file body edits, NO identifier renames, NO .stagehand.toml literal changes.
```

### Implementation Patterns & Key Details

```bash
# === The verification (the bulk of S4's value) ===
# 1. Zero remaining stagehand import-path literals (S1 cleaned them):
grep -rn 'github.com/dustin/stagehand' --include='*.go' . | grep -v './plan/'
# Expected: (empty)

# 2. All build-path literals are stagecoach (the 4 sites):
grep -rno '"github.com/dustin/stage[a-z]*/cmd[a-z/]*"' --include='*.go' . | grep -v './plan/'
# Expected:
#   ./internal/stubtest/stubtest.go:59:"github.com/dustin/stagecoach/cmd/stubagent"
#   ./internal/signal/signal_integration_test.go:143:"github.com/dustin/stagecoach/cmd/stubagent"
#   ./internal/signal/signal_integration_test.go:167:"github.com/dustin/stagecoach/cmd/stagecoach"
#   ./internal/e2e/harness_test.go:65:"github.com/dustin/stagecoach/cmd/stagecoach"

# === The one edit ===
sed -i "s/for Stagehand's integration/for Stagecoach's integration/" cmd/stubagent/main.go
# (or a precise edit-tool replacement of 'Stagehand's' → 'Stagecoach's' on line 1)

# === The gate ===
go build ./...   # exit 0 (post-S3)
go vet ./...     # exit 0
```

```go
// === cmd/stubagent/main.go:1 — the before/after (the single concrete edit) ===
// BEFORE: // Command stubagent is a tiny fake-agent binary for Stagehand's integration/property tests
// AFTER:  // Command stubagent is a tiny fake-agent binary for Stagecoach's integration/property tests
// (Only the possessive proper-noun token changes. The command name 'stubagent' is UNCHANGED — it is not
//  a stagehand/stagecoach name; it is the fake-agent binary's own name, correctly left alone.)
```

### Integration Points

```yaml
MODULE (go.mod): github.com/dustin/stagecoach (S1, LANDED). S4 verifies against this; does not edit go.mod.
BUILD-PATH LITERALS (test code): the 4 exec.Command("go"/goPath, "build", "-o", out, "<import-path>") sites
  all reference github.com/dustin/stagecoach/cmd/{stubagent,stagecoach} (S1, VERIFIED). S4 confirms; no edit.
COMPILATION GATE: go build ./... + go vet ./... (the Layer-1 acceptance gate per rename_surface_map
  Verification Gates line 208-214). Green post-S3.

NO-TOUCH (explicitly — owned by sibling/later subtasks):
  - the 4 test files' BODIES (build literals already correct; other residues are T2/M2/M3)
  - identifiers (buildStagehand, stagehandBin, stagehandOnce, runStagehand, buildStagehandPath) → P1.M1.T2
  - .stagehand.toml / stagehand.toml config-filename literals → P1.M2.T2 (Layer 2.3)
  - STAGEHAND_RUN_REAL env var → P1.M2.T1 (Layer 2.1)
  - temp-dir prefixes (stagehand-stubagent-*, etc.) → P1.M2.T3 (Layer 3.5)
  - status/error strings ("go build stagehand: …") → P1.M2.T3 (Layer 3.2/3.3)
  - temp binary names (name := "stagehand") → P1.M2.T3 / P1.M3.T1 (Layer 3.1/4.5)
  - pkg/stagecoach/* + internal/cmd/default_action.go → S3
  - Makefile / .goreleaser.yaml / .github/workflows → P1.M3
  - docs/* (README, docs/*.md, providers/*.toml) → P1.M4
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by OTHER subtasks, NOT S4):
  - P1.M1.T2 (identifiers): renames buildStagehand→buildStagecoach, stagehandBin→stagecoachBin, etc., and
    fixes their attached doc comments (incl. the 'cmd/stagehand' comment at signal:152/harness:44). S4
    leaves these so T2 owns the identifier+comment as a unit.
  - P1.M2.T2/T3 + P1.M3.T1: rename the config filenames, temp prefixes, env vars, status strings, and the
    distributed binary name. S4's verification proves the build is green BEFORE those (purely cosmetic /
    config-surface) layers run.
  - P1.M5.T2.S1: the FINAL grep audit (zero stagehand refs in tracked files). S4 is an intermediate gate,
    NOT the final audit — residual stagehand refs are EXPECTED to remain after S4 (they are T2/M2/M3's work).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagehand   # repo dir is still named stagehand (expected — rename is module-level)

gofmt -w cmd/stubagent/main.go        # the one edited file (comment-only; gofmt is a no-op on comments but safe)
gofmt -l .                            # Expected: empty
go vet ./...                          # Expected: exit 0
go build ./...                        # Expected: exit 0 (THE Layer-1 gate; post-S3)
```

### Level 2: The Build-Path Verification (S4's core deliverable)

```bash
cd /home/dustin/projects/stagehand

# (a) ZERO remaining stagehand import-path literals (S1 cleaned them):
grep -rn 'github.com/dustin/stagehand' --include='*.go' . | grep -v './plan/'
# Expected: (no output) — ZERO matches. If any match, S1 missed a literal → fix it (it IS Layer 1.6).

# (b) All 4 build-path literals are stagecoach:
grep -rno '"github.com/dustin/stage[a-z]*/cmd[a-z/]*"' --include='*.go' . | grep -v './plan/'
# Expected: exactly 4 lines, all '.../stagecoach/cmd/...' (stubtest:59, signal:143/167, harness:65).

# (c) default_action_test.go has NO direct build literal:
grep -n '"build"' internal/cmd/default_action_test.go || echo "none (uses stubtest.Build())"
# Expected: none (uses stubtest.Build()).

# (d) cmd/stubagent/main.go comment is fixed:
grep -n "Stagecoach's integration" cmd/stubagent/main.go   # → 1 match (L1)
grep -n "Stagehand's integration" cmd/stubagent/main.go    # → 0 matches (the old token is gone)
```

### Level 3: Compilation Gate (the real acceptance criterion)

```bash
cd /home/dustin/projects/stagehand

go build ./...        # Expected: exit 0. THE gate. (If 'undefined: stagehand' in default_action.go → S3 not landed; flag it.)
go vet ./...          # Expected: exit 0.

# Confidence: the test packages that own the build-path literals COMPILE (the literals resolve to real pkgs):
go test -count=1 -run='^$' ./internal/stubtest/ ./internal/signal/ ./internal/e2e/ ./internal/cmd/
# (-run='^$' = compile-only, no test execution.) Expected: builds each package; exit 0.
# (signal/e2e integration tests may need a TTY/git/git-config to RUN, but they must COMPILE — that is S4's concern.)
```

### Level 4: Scope-Discipline Verification (S4 did not over-reach)

```bash
cd /home/dustin/projects/stagehand

# ONLY cmd/stubagent/main.go changed (build-path literals were already correct → NOT in the diff):
git diff --stat
# Expected: cmd/stubagent/main.go ONLY (optionally + internal/stubtest/stubtest.go IF you took the
#           consistency edit on its L1 godoc comment; nothing else).

# The residual stagehand refs in the named test files are UNCHANGED (owned by T2/M2/M3):
git diff -- internal/stubtest/ internal/signal/ internal/e2e/ internal/cmd/default_action_test.go | grep -E '^[+-]' | grep -v '^[+-][+-]' | grep -i stagehand
# Expected: EMPTY (S4 did not edit any test-file body). If non-empty, S4 over-reached — revert those edits
#           (they belong to T2/M2/M3).

# Confirm the identifiers / config literals / temp prefixes / env var are STILL stagehand (untouched, as
# they must be — their owners haven't run yet):
grep -rn 'buildStagehand\|stagehandBin\|\.stagehand\.toml\|STAGEHAND_RUN_REAL\|stagehand-stubagent-\|stagehand-e2e-' --include='*.go' internal/ | grep -v './plan/' | head
# Expected: matches (these are INTENTIONALLY still stagehand — T2/M2/M3 own them). S4 must NOT have changed them.
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0 (THE Layer-1 compilation gate).
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `grep -rn 'github.com/dustin/stagehand' --include='*.go' . | grep -v './plan/'` → ZERO matches.
- [ ] The 4 build-path literals (stubtest:59, signal:143/167, harness:65) read `stagecoach`.
- [ ] `default_action_test.go` has no direct `exec.Command("go","build")` (uses `stubtest.Build()`).

### Feature Validation

- [ ] `cmd/stubagent/main.go:1` reads `// Command stubagent is a tiny fake-agent binary for Stagecoach's
      integration/property tests`.
- [ ] The test packages that own the build-path literals COMPILE (`go test -count=1 -run='^$' ...`).

### Scope Discipline Validation

- [ ] `git diff --stat` shows ONLY `cmd/stubagent/main.go` (optionally + `internal/stubtest/stubtest.go` for
      the parallel godoc consistency edit).
- [ ] S4 did NOT edit any identifier (`buildStagehand`, `stagehandBin`, etc.) — those are T2.
- [ ] S4 did NOT edit any `.stagehand.toml`/`stagehand.toml` literal — those are M2.T2.
- [ ] S4 did NOT edit `STAGEHAND_RUN_REAL` — that is M2.T1.
- [ ] S4 did NOT edit any temp-dir prefix (`stagehand-stubagent-*` etc.) — those are M2.T3.
- [ ] S4 did NOT edit any status/error string or temp binary name — those are M2.T3/M3.T1.
- [ ] S4 did NOT touch `pkg/stagecoach/*` or `default_action.go` (S3) or `go.mod` (S1).
- [ ] S4 did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] The one comment edit is a clean token swap (`Stagehand's` → `Stagecoach's`), no reflowing.
- [ ] No re-running of S1's sed (the import paths are already correct — verify, don't re-edit).
- [ ] The verification greps are the ones quoted above (deterministic, reproducible).

---

## Anti-Patterns to Avoid

- ❌ Don't re-run S1's import-path sed or "fix" the build-path literals. They are ALREADY `stagecoach`
  (verified: stubtest:59, signal:143/167, harness:65 all read `github.com/dustin/stagecoach/cmd/...`; zero
  `github.com/dustin/stagehand` literals remain). S4 VERIFIES S1's work; it does not redo it. Re-sedding
  risks collateral damage to the many other (intentionally-still-stagehand) residues in those files.
- ❌ Don't rename `buildStagehand`, `stagehandBin`, `stagehandOnce`, `runStagehand`, or any other IDENTIFIER.
  Those are P1.M1.T2 (Layer "identifiers"). T2 owns the identifier AND its attached doc comment as a unit;
  S4 touching the identifier (or its `// buildStagehand compiles cmd/stagehand` comment) collides with T2.
- ❌ Don't rename `.stagehand.toml` / `stagehand.toml` config-filename literals. Those are P1.M2.T2
  (Layer 2.3 config file paths). They appear in default_action_test.go, signal, harness, etc. — leave them.
- ❌ Don't rename `STAGEHAND_RUN_REAL` (harness_test.go). That is P1.M2.T1 (Layer 2.1 env vars).
- ❌ Don't rename the temp-dir prefixes (`stagehand-stubagent-*`, `stagehand-stubtest-*`,
  `stagehand-inttest-*`, `stagehand-e2e-*`). Those are P1.M2.T3 (Layer 3.5 temp dir prefix).
- ❌ Don't rename the status/error strings (`"go build stagehand: …"`, `"cannot build stagehand"`) or the
  temp binary names (`name := "stagehand"`, `"stagehand.exe"`). Those are P1.M2.T3 (Layer 3.2/3.3) and
  P1.M2.T3/P1.M3.T1 (Layer 3.1/4.5).
- ❌ Don't attempt the FULL `stagehand`→`stagecoach` sweep. §h2.30's "all references" is the END STATE
  achieved at P1.M5.T2.S1, NOT S4. S4 is the narrow Layer-1.6 gate. Pulling Layer 2/3/4/5 work forward
  breaks the rename_surface_map's layering and collides with every sibling subtask.
- ❌ Don't "fix" the `// buildStagehand compiles cmd/stagehand` comments (signal:152, harness:44). They are
  attached to the `buildStagehand` identifier; T2 renames the function and rewrites its doc comment together.
  S4 editing the comment but not the function leaves it internally inconsistent AND collides with T2.
- ❌ Don't revert the package name or default_action.go qualifiers if `go build` fails with `undefined:
  stagehand`. That means S3 (parallel) has NOT landed its CONTRACT CORRECTION — flag it back to S3. S4
  assumes S3 lands exactly as its PRP specifies.
- ❌ Don't rename the repo directory `/home/dustin/projects/stagehand`. The rename is module/import/
  internal-dir level, not the repo root. The repo dir's name is irrelevant to `go build`.
- ❌ Don't touch `pkg/stagecoach/*`, `default_action.go`, `go.mod`, Makefile/.goreleaser/CI, docs, or any
  other package. S4 is `cmd/stubagent/main.go` (one comment) + verification commands. Nothing else.
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is a verification-first task where the research ALREADY CONFIRMED the functional state (all
4 build-path literals are `stagecoach`; zero `stagehand` import-path literals remain; S1 fully did its job).
The one concrete edit (`cmd/stubagent/main.go:1` `Stagehand's`→`Stagecoach's`) is quoted verbatim. The
acceptance gate (`go build ./...` + `go vet ./...`) is deterministic and post-S3-green by construction. The
verification greps are quoted exactly with their expected outputs. The single real risk — an implementer
OVER-REACHING into the identifiers / config literals / temp prefixes / env vars that LITTER the named test
files (each owned by a different sibling subtask) — is walled off in the Gotchas, the Scope Discipline
Validation, and 11 explicit Anti-Patterns, each naming the exact token and its owner. The S3 dependency is
documented (if the build fails with `undefined: stagehand`, it's S3's bug, not S4's — flag, don't fix). The
only residual uncertainty (not 10/10) is the optional stubtest.go:1 godoc consistency edit (defensible either
way; the PRP permits it but doesn't require it) and whether the implementer correctly resists the temptation
to "clean up" the many still-stagehand residues that are intentionally deferred to T2/M2/M3. No logic, no
identifiers, no config — the blast radius is one comment line plus verification commands.
