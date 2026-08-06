name: "P1.M1.T2.S1 — configVersionNotice: drop destructive `config init --force` from all branches + update tests (BUG-002)"
description: >
  Fix BUG-002 (Minor): configVersionNotice (internal/config/load.go:629-645) suggests the destructive
  `config init --force` in its newer-than-binary branch (the only LIVE branch in Load — FR-B4 says the
  remedy is "upgrade stagecoach" full-stop; --force would regenerate at the OLD schema and discard the
  unreadable newer config). Drop the `config init --force` clause from the live newer branch AND from the
  two dead older/missing branches (defense-in-depth — they re-live if the migration branch is refactored).
  Update TestConfigVersionNotice: remove "config init --force" from the contains lists and add a
  `notContains` field + loop asserting it NEVER appears in any advisory. Update the doc comment (Mode A,
  FR-B4 rationale). Pure-function string edit + test — no I/O, no other file. Sibling BUG-001
  (preservedDefaultProvider) is in internal/cmd/config.go — different file, no conflict.

---

## Goal

**Feature Goal**: Make configVersionNotice's advisory text comply with FR-B4 — NEVER suggest
`config init --force` for any version-mismatch case. The newer-than-binary case (the only LIVE branch in
Load) advises only "Upgrade stagecoach"; the older/missing cases (dead today, but hardened for
defense-in-depth) advise only "Run 'stagecoach config upgrade'". The invariant is locked by a new
`notContains` test assertion.

**Deliverable**: (1) three string edits in `internal/config/load.go::configVersionNotice` (the live
`default` branch + the two dead `version==0` / `version<current` branches); (2) a Mode A update to the
function's doc comment (FR-B4 rationale); (3) an update to `TestConfigVersionNotice` in
`internal/config/load_test.go` — remove `"config init --force"` from the three non-empty `contains` lists
and add a `notContains []string` field + loop asserting it never appears.

**Success Definition**:
- The newer-than-binary advisory reads `"...this binary supports up to 3. Upgrade stagecoach.\n"` (no
  `config init --force`).
- The older/missing advisories read `"... Run 'stagecoach config upgrade'.\n"` (no `config init --force`).
- `grep -c "config init --force" internal/config/load.go` → 0.
- TestConfigVersionNotice's three non-empty cases (missing/older/ahead) each have
  `notContains: []string{"config init --force"}` and the new loop enforces it.
- `go test ./internal/config/ -run TestConfigVersionNotice -v` passes; `go test ./internal/config/
  -count=1`, `make test`, `make lint` pass.
- No change to Load()'s migration branch, the function signature, the `""`-returning branches, or
  CurrentConfigVersion.

## User Persona (if applicable)

**Target User**: A user who downgraded stagecoach after a newer version rewrote their config (so the
config's `config_version` is ahead of the binary). They see the load-time advisory.

**Use Case**: The user reads the advisory and acts on it. Pre-fix it suggested `config init --force`,
which would regenerate their newer config at the old binary's schema (3) — destroying it. Post-fix it
advises only "Upgrade stagecoach", the correct remedy (FR-B4).

**User Journey**: downgrade stagecoach → run stagecoach → advisory "config file uses schema version 99;
this binary supports up to 3. Upgrade stagecoach." → user upgrades stagecoach → the new binary reads the
config correctly. (Pre-fix the user might have run `config init --force` and lost their newer config.)

**Pain Points Addressed**: BUG-002 — the advisory recommended a destructive action (`config init --force`
discards the newer config) that contradicts FR-B4's "the remedy is to upgrade stagecoach."

## Why

- **BUG-002 (Minor) / FR-B4**: For a newer-than-binary config, `config init --force` regenerates at the
  OLD binary's schema (3), discarding the newer config the binary cannot read — the opposite of the
  remedy. FR-B4 says the remedy is "upgrade stagecoach" full-stop, and elsewhere explicitly forbids
  suggesting `config init --force` ("regenerates from a template and is a re-bootstrap, not an upgrade").
  The same reasoning applies a fortiori to the newer case.
- **Defense-in-depth on dead branches**: the `version==0` and `version<current` branches also suggest
  `config init --force`, but Load()'s migration branch (load.go:192-201) handles those cases first, so
  they are dead today. The PRD recommends hardening them anyway ("Consider also removing the stale
  config init --force suggestion from the (currently dead) older/missing branches for defense-in-depth")
  — if the migration branch is ever refactored, the dead branches would re-live with the wrong advice.
- **Bounded, surgical**: three pure-string edits in one pure function + a doc-comment sentence + a test
  table extension. No I/O, no signature change, no behavior change beyond the advisory wording. Sibling
  BUG-001 is in a different file.

## What

**User-visible behavior**: The load-time advisory for a newer-than-binary config no longer suggests
`config init --force`; it advises only "Upgrade stagecoach". (The older/missing advisories — currently
unreachable via Load — also drop the suggestion.)

**Technical change (three string edits + doc comment + test table):**
1. `load.go` configVersionNotice `default` branch (642-644): `"Upgrade stagecoach, or run 'stagecoach
   config init --force' to regenerate.\n"` → `"Upgrade stagecoach.\n"`.
2. `load.go` configVersionNotice `version==0` branch (636-638): `"Run 'stagecoach config upgrade' or
   'stagecoach config init --force'.\n"` → `"Run 'stagecoach config upgrade'.\n"`.
3. `load.go` configVersionNotice `version<current` branch (639-641): same as #2.
4. Mode A: add one FR-B4-rationale sentence to the function's doc comment (625-628).
5. `load_test.go` TestConfigVersionNotice: add `notContains []string` field + loop; drop
   `"config init --force"` from the three non-empty `contains` lists; set
   `notContains: []string{"config init --force"}` on those three rows.

### Success Criteria
- [ ] newer branch advisory ends "Upgrade stagecoach.\n" (no `config init --force`)
- [ ] older/missing branch advisories end "Run 'stagecoach config upgrade'.\n" (no `config init --force`)
- [ ] `grep -c "config init --force" internal/config/load.go` → 0
- [ ] TestConfigVersionNotice's three non-empty rows have `notContains: ["config init --force"]`
- [ ] doc comment references FR-B4 rationale
- [ ] `go build ./...`, `go test ./internal/config/ -count=1`, `make test`, `make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact current strings (with line numbers), the exact replacement strings, the live/dead
branch reasoning, the doc-comment sentence to add, the test table's exact rows (old → new), the
`notContains` field + loop, the `CurrentConfigVersion=3` pinning, and the scope fences against BUG-001
are all below.

### Documentation & References

```yaml
- file: internal/config/load.go
  why: "THE change site. configVersionNotice @629-645 (PURE function, 4 switch branches). The LIVE bug
        is the `default` branch @642-644 (version > CurrentConfigVersion). The two DEAD branches
        (@636-638 version==0, @639-641 version<current) are hardened for defense-in-depth. Doc comment
        @625-628 gets the Mode A FR-B4 sentence. Load()'s migration branch @192-201 is why older/missing
        are dead — DO NOT change it."
  pattern: >
    // current default branch:
    return fmt.Sprintf("stagecoach: config file uses schema version %d; this binary supports up to %d. "+
        "Upgrade stagecoach, or run 'stagecoach config init --force' to regenerate.\n", version, CurrentConfigVersion)
    // FIXED:
    ... "Upgrade stagecoach.\n", version, CurrentConfigVersion)
  critical: "Three PRECISE string edits — drop the ', or run ... config init --force ...' / 'or ... config
             init --force' clauses. Keep the leading 'stagecoach: ...' diagnostic + the %d/%d verbatim.
             Keep the trailing \\n. Do NOT touch the version==CurrentConfigVersion or !fileLoaded branches
             (they return \"\"). Do NOT touch Load()'s migration branch (@192-201)."

- file: internal/config/load_test.go
  why: "THE test to update. TestConfigVersionNotice @1962 (table-driven; struct has name/fileLoaded/
        version/wantEmpty/contains). The three non-empty rows: 'missing (0)' @~970-line-of-table,
        'older (1)', 'ahead (4)'. Add a `notContains []string` field + an enforcement loop."
  pattern: >
    // struct field to add:
    notContains []string
    // loop to add (after the contains loop, before the \\n-suffix check):
    for _, sub := range tc.notContains {
        if strings.Contains(got, sub) {
            t.Errorf("configVersionNotice(%v, %d) = %q, must NOT contain %q", tc.fileLoaded, tc.version, got, sub)
        }
    }
  critical: "Drop \"config init --force\" from the contains lists of all THREE non-empty rows (missing/
             older/ahead). Set notContains:[\"config init --force\"] on those three. The two wantEmpty=true
             rows return early before the loops ⇒ notContains nil/unused for them (leave nil). CurrentConfigVersion
             is 3 (config.go:20) ⇒ the existing 'current is 3' / 'supports up to 3' pins stay valid."

- docfile: plan/018_29b859efcd56/bugfix/001_4be1bd5c953f/architecture/bug002_analysis.md
  why: "The authoritative BUG-002 analysis: the live/dead branch reality (only default is live in Load
        because the migration branch handles older/missing first), the exact current vs fixed strings,
        and the test changes."
  section: "Fix Design (Part 1 + Part 2 + Test Changes)"

- docfile: plan/018_29b859efcd56/bugfix/001_4be1bd5c953f/P1M1T1S2/PRP.md
  why: "The parallel sibling (BUG-001). It edits internal/cmd/config.go (runConfigInit) + the helper at
        internal/cmd/config.go:449 (preservedDefaultProvider). DIFFERENT file from internal/config/load.go.
        Read it to confirm the non-overlap (no merge conflict)."
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  load.go           # THE fix: configVersionNotice @629-645 (3 string edits) + doc comment @625-628 (Mode A)
  load_test.go      # THE test: TestConfigVersionNotice @1962 (add notContains field/loop; drop "config init --force" from 3 rows)
  config.go         # CurrentConfigVersion = 3 (@20) — UNCHANGED
internal/cmd/
  config.go         # BUG-001 sibling (preservedDefaultProvider @449, runConfigInit) — NOT touched here
docs/cli.md         # changeset-level docs — P1.M1.T3.S1, NOT this task
```

### Desired Codebase tree with files to be added

```bash
internal/config/load.go        # MODIFY: configVersionNotice 3 string edits + doc-comment sentence
internal/config/load_test.go   # MODIFY: TestConfigVersionNotice — +notContains field/loop, drop "config init --force" from 3 rows
# (no new files; no other package touched)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (only the LIVE branch is user-visible, but harden ALL three): Load()'s migration branch
//   (load.go:192-201) handles version<CurrentConfigVersion (incl. 0) BEFORE configVersionNotice, so only
//   the `default` (>current) branch is reachable today. The PRD still wants the dead older/missing
//   branches fixed (defense-in-depth — they re-live if migration is refactored). Edit ALL THREE; do not
//   skip the dead ones.

// CRITICAL (precise string edits — don't re-flow the whole Sprintf): each branch is a two-line
//   fmt.Sprintf with a leading "stagecoach: ..." diagnostic and a trailing action sentence. Edit ONLY
//   the action sentence (drop the config init --force clause); keep the diagnostic + the %d/%d + the
//   leading "stagecoach:" + the trailing \n byte-identical. A gofmt-altering reflow is unnecessary.

// CRITICAL (CurrentConfigVersion == 3, config.go:20): the test rows pin "current is 3" and "supports up
//   to 3" as literals. These stay valid (this task does NOT bump the version). Do not "parameterize"
//   them — leave the existing pins; only the config-init--force token changes.

// CRITICAL (the notContains loop must run only for non-empty results): the wantEmpty=true rows
//   `return` early inside t.Run BEFORE the contains/notContains loops. So notContains is nil/unused for
//   them. Put the new notContains loop AFTER the `if got == "" { t.Fatalf }` guard + the contains loop,
//   so it only runs for the non-empty cases.

// GOTCHA (do NOT touch Load()'s migration branch): load.go:192-201 is correct (it handles older/missing
//   with migrateV2ToV3 + migrationNotice). Only configVersionNotice's strings change. Editing the
//   migration branch would change live behavior and is out of scope.

// GOTCHA (do NOT touch the ""-returning branches): version==CurrentConfigVersion → "" and !fileLoaded →
//   "" are correct and tested (the wantEmpty=true rows). Leave them.

// SCOPE: do NOT touch internal/cmd/config.go (BUG-001 = P1.M1.T1.S1/S2). do NOT touch docs/cli.md
//   (changeset-level docs = P1.M1.T3.S1). do NOT change the function signature, CurrentConfigVersion,
//   or any other configVersionNotice call site (there is only one — Load()).
```

## Implementation Blueprint

### Data models and structure
None. Three pure-string edits in one pure function + a doc-comment sentence + a test-table field/loop
extension. No struct/type/signature/behavior change beyond advisory wording.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/config/load.go configVersionNotice — the 3 string edits
  - LOCATE configVersionNotice (search "func configVersionNotice" — @629).
  - EDIT the `default` branch (@642-644): change the action clause
      OLD: "Upgrade stagecoach, or run 'stagecoach config init --force' to regenerate.\n"
      NEW: "Upgrade stagecoach.\n"
    (keep `"stagecoach: config file uses schema version %d; this binary supports up to %d. "` + the
    two %d args verbatim)
  - EDIT the `version == 0` branch (@636-638): change the action clause
      OLD: "Run 'stagecoach config upgrade' or 'stagecoach config init --force'.\n"
      NEW: "Run 'stagecoach config upgrade'.\n"
  - EDIT the `version < CurrentConfigVersion` branch (@639-641): same OLD→NEW as the version==0 branch.
  - DO NOT touch: the `!fileLoaded → ""`, `version == CurrentConfigVersion → ""` branches; Load()'s
    migration branch (@192-201); the function signature; the trailing \n on any branch.
  - DEPENDENCIES: none.

Task 2: EDIT internal/config/load.go — the Mode A doc comment (@625-628)
  - APPEND one sentence to the existing doc comment, e.g.:
      "Per FR-B4, the newer-than-binary remedy is upgrade-only (never `config init --force`, which would
       regenerate at this binary's older schema and discard the unreadable newer config); the older/missing
       branches advise `config upgrade` (also never `config init --force`)."
  - KEEP the rest of the doc comment (PURE/no-I/O, fileLoaded semantics, §16.1 metadata note) intact.
  - DEPENDENCIES: Task 1.

Task 3: EDIT internal/config/load_test.go TestConfigVersionNotice (@1962) — the table + loop
  - ADD a `notContains []string` field to the table struct (alongside `contains`).
  - UPDATE the three non-empty rows:
      missing (0): contains []string{"has no config_version", "config upgrade"}; notContains []string{"config init --force"}
      older (1):   contains []string{"schema version 1", "current is 3", "config upgrade"}; notContains []string{"config init --force"}
      ahead (4):   contains []string{"schema version 4", "supports up to 3", "Upgrade stagecoach"}; notContains []string{"config init --force"}
    (the two wantEmpty=true rows — "no file" × 2, "current version" — UNCHANGED; notContains stays nil)
  - ADD the notContains enforcement loop inside t.Run, AFTER the `if got == "" { t.Fatalf }` guard and
    AFTER the existing contains loop, BEFORE the \n-suffix check:
        for _, sub := range tc.notContains {
            if strings.Contains(got, sub) {
                t.Errorf("configVersionNotice(%v, %d) = %q, must NOT contain %q", tc.fileLoaded, tc.version, got, sub)
            }
        }
  - DEPENDENCIES: Task 1 (the strings must be fixed or the notContains assertion fails).

Task 4: VERIFY build + vet + format + targeted test + full package
  - go build ./...
  - go vet ./internal/config/...
  - gofmt -l internal/config/load.go internal/config/load_test.go   # must list nothing
  - go test ./internal/config/ -run TestConfigVersionNotice -v       # the updated table
  - go test ./internal/config/ -count=1                              # full package
  - make test && make lint
  - Grep guard: grep -c "config init --force" internal/config/load.go  → 0
  - REGRESSION-CHECK (by reasoning): pre-fix, the ahead(4) row's notContains:["config init --force"]
    would FAIL (the old default-branch string contained it); post-fix it PASSES. This is the test's
    reason to exist.
```

### Implementation Patterns & Key Details

```go
// PATTERN: the three precise string edits (edit ONLY the action clause; keep the diagnostic + args + \n)
// default (LIVE):
fmt.Sprintf("stagecoach: config file uses schema version %d; this binary supports up to %d. "+
    "Upgrade stagecoach.\n", version, CurrentConfigVersion)   // was: "...regenerate.\n"
// version==0 (DEAD, harden):
fmt.Sprintf("stagecoach: config file has no config_version; current is %d. "+
    "Run 'stagecoach config upgrade'.\n", CurrentConfigVersion)   // was: "...or '...config init --force'.\n"
// version<current (DEAD, harden):
fmt.Sprintf("stagecoach: config file uses schema version %d; current is %d. "+
    "Run 'stagecoach config upgrade'.\n", version, CurrentConfigVersion)   // was: "...or '...config init --force'.\n"

// PATTERN: the notContains field + loop (the BUG-002 invariant lock)
type tcStruct struct {
    name       string
    fileLoaded bool
    version    int
    wantEmpty  bool
    contains   []string
    notContains []string  // NEW — substrings that MUST NOT appear
}
// inside t.Run, after the contains loop:
for _, sub := range tc.notContains {
    if strings.Contains(got, sub) {
        t.Errorf("configVersionNotice(%v, %d) = %q, must NOT contain %q", tc.fileLoaded, tc.version, got, sub)
    }
}
```

### Integration Points

```yaml
NO struct / signature / API / config / build / CLI changes. Three advisory-string edits + doc comment + test.

CODE:
  - internal/config/load.go configVersionNotice @629-645 — 3 string edits; doc comment @625-628 (Mode A)
TEST:
  - internal/config/load_test.go TestConfigVersionNotice @1962 — +notContains field/loop; 3 row updates

CONSUMED (read-only, unchanged):
  - CurrentConfigVersion (=3, config.go:20) — the %d in the strings + the "current is 3"/"supports up to 3" pins
  - Load()'s migration branch (load.go:192-201) — why older/missing are dead; NOT edited

DOWNSTREAM (do NOT implement here):
  - P1.M1.T3.S1: docs/cli.md update for the version-advisory accuracy (changeset-level docs)

UNCHANGED (do NOT touch): internal/cmd/config.go (BUG-001); Load()'s migration branch; the ""-returning
  branches; the function signature; CurrentConfigVersion; any other call site (there is only one — Load()).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build (pure-string edit; compiles)
go build ./...
# Vet the package
go vet ./internal/config/...
# Format check
gofmt -l internal/config/load.go internal/config/load_test.go
# Expected: nothing listed. If listed: gofmt -w them.
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The updated configVersionNotice table
go test ./internal/config/ -run TestConfigVersionNotice -v
# Expected: all subtests PASS — missing/older/ahead contain the new advice AND NOT "config init --force".

# Full config package (regression)
go test ./internal/config/ -count=1
# Expected: ALL pass.

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# The advisory is printed by Load() to the notice writer. Optional manual confirmation of the newer-case
# wording (the within-scope proof is the unit test, which calls the pure function directly):
cat > /tmp/ahead.toml <<'EOF'
config_version = 99
[defaults]
provider = "stub"
EOF
go build -o /tmp/sc ./cmd/stagecoach
/tmp/sc --config /tmp/ahead.toml --dry-run 2>&1 | grep -i "schema version 99"
# Expected: a line ending "Upgrade stagecoach." — and NO "config init --force" anywhere in the output.
rm -f /tmp/ahead.toml /tmp/sc
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: ZERO "config init --force" occurrences in load.go (the BUG-002 invariant)
grep -c "config init --force" internal/config/load.go
# Expected: 0.

# Grep guard: the new advice strings are present
grep -c "Upgrade stagecoach" internal/config/load.go                  # the live newer branch → 1
grep -c "Run 'stagecoach config upgrade'" internal/config/load.go     # the two dead branches → 2

# Grep guard: the notContains invariant is asserted in the test
grep -n "notContains" internal/config/load_test.go
# Expected: the struct field + the 3 row values + the enforcement loop.

# Scope-boundary guard: ONLY load.go + load_test.go changed by this subtask
git diff --stat -- internal/cmd/config.go docs/cli.md
# Expected: empty (BUG-001 = P1.M1.T1.S1/S2; docs = P1.M1.T3.S1).

# Scope-boundary guard: Load()'s migration branch UNCHANGED
git diff internal/config/load.go | grep -E '^[+-]' | grep -iE "migrateV2ToV3|migrationNotice|ConfigVersion < CurrentConfigVersion" || echo "OK: migration branch untouched"
# Expected: "OK: migration branch untouched".

# Regression-property check (by reasoning): the ahead(4) row's notContains:["config init --force"]
# would FAIL on the pre-fix code (the old default string contained it); post-fix it PASSES. (Optional
# empirical check: temporarily restore the old default string, re-run, observe the FAIL, revert.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/config/...` clean
- [ ] `gofmt -l internal/config/load.go internal/config/load_test.go` empty
- [ ] `make lint` zero errors
- [ ] `go test ./internal/config/ -count=1` passes; `make test` passes

### Feature Validation
- [ ] newer branch advisory ends "Upgrade stagecoach.\n" (no `config init --force`)
- [ ] older/missing branch advisories end "Run 'stagecoach config upgrade'.\n" (no `config init --force`)
- [ ] `grep -c "config init --force" internal/config/load.go` → 0
- [ ] TestConfigVersionNotice's three non-empty rows have `notContains: ["config init --force"]` and the loop enforces it
- [ ] doc comment references FR-B4 rationale

### Scope-Boundary Validation
- [ ] NO change to internal/cmd/config.go (BUG-001)
- [ ] NO change to docs/cli.md (P1.M1.T3.S1)
- [ ] NO change to Load()'s migration branch, the ""-returning branches, the function signature, or CurrentConfigVersion
- [ ] Only load.go + load_test.go touched

### Code Quality
- [ ] Precise string edits (no whole-Sprintf reflow; diagnostic + args + \n byte-identical)
- [ ] All THREE branches fixed (the live default + the two dead older/missing — defense-in-depth)
- [ ] The notContains loop runs only for non-empty results (after the wantEmpty/Fatalf guards)
- [ ] Mode A doc comment states the FR-B4 upgrade-only rationale

---

## Anti-Patterns to Avoid

- ❌ Don't fix ONLY the live `default` branch — the PRD explicitly wants the dead older/missing branches hardened too (defense-in-depth: they re-live if the migration branch is refactored). Edit all THREE.
- ❌ Don't re-flow the whole `fmt.Sprintf` — edit ONLY the action clause (drop the config init --force part); keep the leading `"stagecoach: ..."` diagnostic, the `%d`/`%d` args, and the trailing `\n` byte-identical. A gofmt-driven reflow is unnecessary noise.
- ❌ Don't touch Load()'s migration branch (load.go:192-201) — it is correct (it handles older/missing via migrateV2ToV3 + migrationNotice). Only configVersionNotice's strings change.
- ❌ Don't touch the `""`-returning branches (`version==CurrentConfigVersion`, `!fileLoaded`) — they are correct and covered by the `wantEmpty=true` rows.
- ❌ Don't "parameterize" the test's `"current is 3"` / `"supports up to 3"` pins — CurrentConfigVersion is 3 (config.go:20) and this task does NOT bump it. Leave those pins; only the config-init--force token changes.
- ❌ Don't put the `notContains` loop before the `wantEmpty` early-return or the `if got == ""` Fatalf — it must run ONLY for non-empty results (else it would error on the empty cases, or skip via the early return). Place it after the contains loop.
- ❌ Don't forget to set `notContains: ["config init --force"]` on ALL THREE non-empty rows (missing/older/ahead) — the invariant is "never in ANY advisory."
- ❌ Don't touch internal/cmd/config.go (BUG-001 = P1.M1.T1.S1/S2 — preservedDefaultProvider @449 + runConfigInit) or docs/cli.md (P1.M1.T3.S1) — different files, different bugs/tasks.
- ❌ Don't change the function signature or add a new call site — configVersionNotice is called from exactly one place (Load()), which is unchanged.

---

## Confidence Score: 10/10

This is three precise advisory-string edits in one pure function (no I/O, no signature change, no
behavior change beyond wording) + a doc-comment sentence + a test-table field/loop extension. The exact
current and replacement strings are specified verbatim, the live/dead branch reality is documented
(Load's migration branch handles older/missing first), the `CurrentConfigVersion=3` pinning is confirmed,
the test rows (old → new) are enumerated, the notContains loop placement is pinned (after the wantEmpty/
Fatalf guards), and the sibling BUG-001 is confirmed to be in a different file (internal/cmd/config.go).
The only conceivable failure modes — fixing only the live branch, re-flowing the Sprintf, touching the
migration branch, or mis-placing the notContains loop — are each explicitly guarded by a CRITICAL gotcha
+ a Level-4 grep guard.