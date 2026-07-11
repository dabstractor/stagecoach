name: "P1.M2.T2.S1 — Insert WriteTimestampedBackup before os.WriteFile in runConfigUpgrade (Issue 3, FR-B8)"
description: >
  Fix the FR-B8 violation in `config upgrade`: `runConfigUpgrade` (`internal/cmd/config.go:157`) overwrites the config
  in-place at `:186` (`os.WriteFile`) with NO preceding backup — an asymmetry vs `config init --force` (writeBootstrapFile
  :521-528), which DOES call `config.WriteTimestampedBackup(path)` first. Insert the EXACT backup block from writeBootstrapFile
  (the inner if/else-if, without its outer `force`/`os.Stat` guard — runConfigUpgrade already proved the file exists via
  `os.ReadFile` at :159, and WriteTimestampedBackup is nil-safe for a missing file regardless) between the `if !changed`
  early-return (:182) and the `os.WriteFile` (:186). The backup fires ONLY when `changed==true` (a real overwrite); the
  no-file / malformed-TOML / inert / already-current gates all return early before reaching it. NO new import (`config`,
  `fmt`, `exitcode` all already in scope — verified); NO new function (reuse WriteTimestampedBackup); NO test change (S2
  adds the positive backup assertions; the existing 3 upgrade tests stay GREEN because they `SetErr(io.Discard)` and don't
  assert file counts — the new `.bak.*` file + stderr notice are behaviorally invisible to them); NO docs (FR-B8 already
  documented). Production-code-only edit to one function. Consumed by P1.M2.T2.S2 (the test assertions).

---

## Goal

**Feature Goal**: Make `config upgrade` honor FR-B8 ("every config-writing command leaves a timestamped backup of the prior
file") identically to `config init --force` — so an in-place schema rewrite is always undoable. The fix is a single localized
insertion of the proven `writeBootstrapFile` backup pattern into `runConfigUpgrade`, between the already-current gate and the
overwrite.

**Deliverable** (ONE edit to ONE function in ONE file):
- **`internal/cmd/config.go`** — in `runConfigUpgrade`, insert the `config.WriteTimestampedBackup(path)` block (error →
  `exitcode.New`; success-with-backup → stderr "Backed up previous config to …" notice) between the `if !changed { … return nil }`
  block and the `os.WriteFile` call. Mirrors `writeBootstrapFile` :521-528 exactly (minus the outer `force`/`os.Stat` guard).

**Success Definition**:
- `config upgrade` on an older config (changed==true) creates a `config.toml.bak.<RFC3339-compact-UTC>` sibling containing the
  PRIOR content, prints "Backed up previous config to <path>" to stderr, THEN overwrites — identical to `config init --force`.
- A backup failure (`WriteTimestampedBackup` returns error) is a HARD error (`exitcode.Error`) — the file is NOT clobbered without
  a recoverable copy (FR-B8 / mirrors writeBootstrapFile).
- The no-file / malformed-TOML / inert / already-current paths still return early WITHOUT creating a backup (no spurious backups
  on no-op runs — the insertion is after the `!changed` gate).
- `go build ./...` clean; `make test` (race) GREEN (the 3 existing upgrade tests pass unchanged — verified: they discard stderr
  and don't assert file counts); `make lint` clean; `gofmt -l` empty.
- Exactly ONE new `config.WriteTimestampedBackup(path)` call site (in runConfigUpgrade); the existing one in writeBootstrapFile
  is untouched.

## User Persona (if applicable)

**Target User**: Any user running `stagecoach config upgrade` to bring their config to the current schema (PRD §9.17 FR-B5).

**Use Case**: A user on an older config_version runs `config upgrade`; the command rewrites their config in place. Before this
fix, the prior content was destroyed with no recovery. After: a timestamped `.bak` sibling is left alongside, so the user can
`diff`/restore if the rewrite surprised them.

**User Journey**: `stagecoach config upgrade` → (stderr) "Backed up previous config to ~/.config/stagecoach/config.toml.bak.2026-07-10T120000Z"
→ (stdout) "Upgraded config at … to version 3." → user inspects; if unhappy, `cp` the `.bak` back. Identical UX to `config init --force`.

**Pain Points Addressed**: Issue 3 (Major) / FR-B8 — `config upgrade` was the ONE config-writing command that did not leave a
recovery backup, violating the reversible-write guarantee the other commands honor.

## Why

- **FR-B8 / §9.17**: "Every command that writes the config file — `config init`, `config init --force` and `--template`, `config
  upgrade`, and the install/first-run bootstrap — must also leave a timestamped backup of the prior file alongside it." `config
  upgrade` was the lone violator (verified: writeBootstrapFile backs up; runConfigUpgrade did not).
- **Symmetry / least-surprise**: the upgrade rewrite is the MOST consequential config write (a schema transform, not a refresh).
  It of all writes should be undoable. The fix makes it byte-for-byte symmetric with `config init --force`.
- **Bounded scope**: a single ~6-line insertion reusing an existing, tested function (`WriteTimestampedBackup`). No new logic,
  no new import, no test change required for green (S2 adds positive assertions later), no docs.

## What

**User-visible behavior**: `config upgrade` that actually changes the file now (a) leaves a `config.toml.bak.<timestamp>` sibling
and (b) prints a one-line "Backed up previous config to …" notice on stderr. No-op upgrades (already-current / inert) are unchanged.

**Technical change**: insert the writeBootstrapFile backup block into runConfigUpgrade at the overwrite site. See the Implementation
Blueprint for the verbatim before/after.

### Success Criteria
- [ ] `runConfigUpgrade` calls `config.WriteTimestampedBackup(path)` AFTER the `if !changed` gate and BEFORE `os.WriteFile`.
- [ ] A backup error returns `exitcode.New(exitcode.Error, fmt.Errorf("backup existing config %s: %w", path, berr))` (hard error, no clobber).
- [ ] A successful backup prints `fmt.Fprintf(cmd.OutOrStderr(), "Backed up previous config to %s\n", backup)` (stderr, matches writeBootstrapFile).
- [ ] A `("", nil)` return (no backup — impossible here since ReadFile proved the file exists, but handled) prints no notice and proceeds.
- [ ] The no-file / malformed-TOML / inert / already-current paths do NOT reach the backup code (early returns unchanged).
- [ ] `go build ./...` clean; `make test` (race) green; `make lint` clean; `gofmt -l internal/cmd/config.go` empty.
- [ ] NO new import; NO new function; NO test change (S2 owns assertions); NO docs change; writeBootstrapFile + backup.go UNTOUCHED.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact insertion point (between the `if !changed` block close and `os.WriteFile`, anchored by string), the verbatim
code to insert (from the contract + the writeBootstrapFile reference), the reason no `os.Stat` guard is needed (runConfigUpgrade
already proved the file exists via `os.ReadFile`; WriteTimestampedBackup is nil-safe regardless), the verified import scope (config/
fmt/exitcode all already used in the same function), the WriteTimestampedBackup contract (3 return cases), the proof that existing
tests stay green (they `SetErr(io.Discard)` and don't count files), and the scope fence (production-only; tests are S2).

### Documentation & References

```yaml
# MUST READ — the authoritative research (the exact code + why no Stat guard + why tests stay green)
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/P1M2T2S1/research/findings.md
  why: "§1 the exact insertion code; §2 why no os.Stat guard (vs writeBootstrapFile); §3 WriteTimestampedBackup's 3 return cases;
        §4 imports already in scope; §5 the backup fires only on changed==true; §6 CRITICAL — the 3 upgrade tests stay green
        (SetErr(io.Discard), no file-count assertions); §7 idempotency preserved; §8 scope fence."
  critical: "§6: do NOT 'fix' a test that appears to ignore the backup — the existing upgrade tests INTENTIONALLY discard stderr
             and don't count files, so they pass unchanged. S2 adds the positive backup assertions. Editing tests here = scope creep into S2."

# MUST READ — the bug + the reference pattern (the source of truth for the insertion)
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/architecture/config_upgrade_backup.md
  why: "The verbatim runConfigUpgrade before/after, the writeBootstrapFile reference pattern, WriteTimestampedBackup's signature,
        and the exact fix block."
  critical: "The doc's suggested insertion omits the outer `if force`/`os.Stat` guard from writeBootstrapFile — that's CORRECT for
             runConfigUpgrade (the file provably exists). Do not copy writeBootstrapFile's guard verbatim; copy its INNER if/else-if block."

# MUST EDIT — the bug site (the ONLY file this task touches)
- file: internal/cmd/config.go
  why: "runConfigUpgrade (:157) is the function; :186 os.WriteFile is the overwrite with no backup. Insert the backup block between
        the `if !changed` block close and the os.WriteFile."
  pattern: "Mirror writeBootstrapFile's inner block (:521-528): `if backup, berr := config.WriteTimestampedBackup(path); berr != nil { …exitcode.New… }
            else if backup != \"\" { fmt.Fprintf(cmd.OutOrStderr(), …) }`."
  gotcha: "Do NOT wrap it in writeBootstrapFile's `if force { if _, err := os.Stat(path); err == nil { … } }` — runConfigUpgrade has no
           `force` concept and has already proved the file exists via os.ReadFile (:159). Insert the bare inner block."

# MUST READ — the function being reused (do NOT modify it; just call it)
- file: internal/config/backup.go
  why: "WriteTimestampedBackup (:18) — the reusable backup primitive. Returns ('', nil) for missing file, (backupPath, nil) on success,
        ('', error) on failure. NO change to this file."
  pattern: "Caller treats error as HARD (return exitcode.Error, do not proceed to WriteFile); treats '' as 'nothing backed up' (proceed)."

# MUST READ — the reference pattern (copy its INNER block, not its guard)
- file: internal/cmd/config.go   # writeBootstrapFile :504-533
  why: "The proven, tested backup pattern (used by config init --force). The new runConfigUpgrade block is its inner if/else-if,
        verbatim, minus the outer force/Stat wrapping."
  gotcha: "writeBootstrapFile gates behind `if force` + `os.Stat` because config init can be a first-time write. runConfigUpgrade cannot
           (ReadFile already gated no-file). Copy the inner block only."

# CONTEXT — the existing tests (stay GREEN; S2 extends them — do NOT edit here)
- file: internal/cmd/config_test.go   # TestConfigUpgrade_OlderUpdated :1137, TestConfigUpgrade_V2ToV3Rewrite :1423, TestConfigUpgrade_Idempotent :1186
  why: "These 3 tests run config upgrade and assert content/stdout. They ALL `SetErr(io.Discard)` and NONE glob the dir / assert file
        counts → the new .bak file + stderr notice are invisible to them → they pass UNCHANGED. Read them to CONFIRM, not to edit."
  critical: "S2 (P1.M2.T2.S2) adds the positive backup assertions (clone config_test.go:631-634's `filepath.Glob(config.toml.bak.*)`).
             Do NOT add/modify any test in this task. The V2ToV3Rewrite 2nd-run idempotency check still holds: changed==false → early return → no 2nd backup."

# CONTEXT — the parallel sibling (no overlap)
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/P1M2T1S2/PRP.md
  why: "PARALLEL sibling is TEST-ONLY in internal/config/bootstrap_test.go (Issue 2, commented-pi-block). Different package + file from
        this task's edit (internal/cmd/config.go). ZERO overlap → no merge conflict."
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/
  config.go          # EDIT — runConfigUpgrade: insert backup block before os.WriteFile (:186)
  config_test.go     # READ-ONLY — 3 upgrade tests stay GREEN (S2 adds backup assertions, not this task)
internal/config/
  backup.go          # READ-ONLY — WriteTimestampedBackup (the reused primitive; do NOT modify)
# go.mod, Makefile, docs/ — READ-ONLY (no docs change; FR-B8 already documented)
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
# MODIFIED (exactly ONE file, ONE function, ONE insertion):
internal/cmd/config.go   # runConfigUpgrade: + backup block between !changed gate and os.WriteFile
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (do NOT copy writeBootstrapFile's outer guard): writeBootstrapFile wraps its backup in
// `if force { if _, err := os.Stat(path); err == nil { … } }` because config init can be a FIRST-TIME write (no
// existing file). runConfigUpgrade is different: os.ReadFile(path) at :159 already returned "no config file" on
// IsNotExist, so by the overwrite site the file provably exists. Insert the BARE inner if/else-if block — no force,
// no os.Stat. (WriteTimestampedBackup is nil-safe for a missing file anyway, so even defensively it's correct.)

// CRITICAL (existing tests stay green — do NOT edit them): the 3 upgrade tests (TestConfigUpgrade_OlderUpdated,
// _V2ToV3Rewrite, _Idempotent) ALL call rootCmd.SetErr(io.Discard), so the new "Backed up previous config to …" stderr
// notice is discarded. NONE glob the dir or assert a file count, so the new .bak.* sibling is harmless. They assert
// stdout ("Upgraded …") and on-disk content — both unchanged by this fix. S2 adds the POSITIVE backup assertion later;
// editing tests here is scope creep into S2.

// CRITICAL (backup fires ONLY on changed==true): the insertion is AFTER `if !changed { … return nil }` (:182) AND after
// the no-file (:164), malformed-TOML (:171), and inert (:177) gates. So no-op upgrades create NO backup. This matches
// FR-B8 ("back up the prior file when you overwrite it") and preserves the V2ToV3Rewrite 2nd-run idempotency assertion
// (2nd run: changed==false → early return → no 2nd backup → content-equal holds).

// GOTCHA (no new import): runConfigUpgrade already uses config.ResolveConfigPath, config.CurrentConfigVersion,
// config.IsInert, exitcode.New, fmt.Errorf, fmt.Fprintf, os.WriteFile in its body. So config (internal/config),
// exitcode, and fmt are imported. The insertion adds config.WriteTimestampedBackup + exitcode.New + fmt.Errorf +
// fmt.Fprintf — ALL already in scope. Zero import-block change.

// GOTCHA (WriteTimestampedBackup is second-granular → same-second collision is a hard error): two REAL upgrades within
// the same UTC second collide on the backup filename and the second fails as exitcode.Error. This is IDENTICAL to config
// init --force today (symmetric, acceptable, and impossible to hit in the idempotency tests since the 2nd run is a no-op).

// GOTCHA (the backup contains the PRIOR content, by design): FR-B8 backs up the file BEFORE the overwrite, so the .bak
// holds the pre-upgrade content. That's the recovery artifact. Do not back up the NEW content.
```

## Implementation Blueprint

### Data models and structure

None. No type changes, no new functions, no new imports. A single localized insertion reusing the existing
`config.WriteTimestampedBackup` primitive.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/cmd/config.go — insert the backup block in runConfigUpgrade (the ONLY change)
  - LOCATE: the `if !changed { … return nil }` block in runConfigUpgrade (the block starting at :182). The os.WriteFile
    is the NEXT statement after its closing `}` (at :186).
  - EXACT ANCHOR (the oldText to match) — the close of the !changed block + the os.WriteFile:
        }
        if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
            return exitcode.New(exitcode.Error, fmt.Errorf("write config %s: %w", path, err))
        }
    (i.e. the `}` terminating the `if !changed` block, immediately followed by the os.WriteFile if-statement.)
  - INSERT between the `}` and the `if err := os.WriteFile` (verbatim):
        // FR-B8 reversible-write guarantee (mirrors writeBootstrapFile): back up the prior config BEFORE the overwrite so
        // every upgrade is undoable. runConfigUpgrade already proved the file exists (os.ReadFile at the top succeeded and
        // returned on IsNotExist), so no os.Stat guard is needed; WriteTimestampedBackup is nil-safe for a missing file anyway.
        // The backup fires ONLY here — the no-file / malformed-TOML / inert / already-current gates all returned early above.
        if backup, berr := config.WriteTimestampedBackup(path); berr != nil {
            return exitcode.New(exitcode.Error, fmt.Errorf("backup existing config %s: %w", path, berr))
        } else if backup != "" {
            fmt.Fprintf(cmd.OutOrStderr(), "Backed up previous config to %s\n", backup)
        }
  - FOLLOW pattern: writeBootstrapFile (:521-528) inner if/else-if block — verbatim error message ("backup existing config %s: %w")
    and notice ("Backed up previous config to %s\n"). Same exitcode.Error severity; same cmd.OutOrStderr() stream.
  - NO IMPORT CHANGES (config/exitcode/fmt all in scope — verified by existing usage in the same function).
  - PRESERVE: the `if !changed` block; the os.WriteFile call + its error wrapping; the "Upgraded config at …" stdout message;
    every gate above (no-file, malformed-TOML, inert); upgradeConfigVersion; writeBootstrapFile; backup.go.
  - NAMING/PLACEMENT: the block sits in runConfigUpgrade only; variable `backup`/`berr` match writeBootstrapFile.

Task 2: VERIFY — build, focused + full tests, lint, format, grep guards
  - go build ./...
  - go test ./internal/cmd/ -run 'TestConfigUpgrade' -v      # the 3 upgrade tests pass UNCHANGED
  - make test                                                 # race; full suite green
  - make lint
  - gofmt -l internal/cmd/config.go                           # empty
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the backup block (the writeBootstrapFile inner if/else-if, verbatim — no outer guard).
// Inserted in runConfigUpgrade between the `if !changed` block and `os.WriteFile`:
if backup, berr := config.WriteTimestampedBackup(path); berr != nil {
    return exitcode.New(exitcode.Error, fmt.Errorf("backup existing config %s: %w", path, berr)) // HARD error — never clobber
} else if backup != "" {
    fmt.Fprintf(cmd.OutOrStderr(), "Backed up previous config to %s\n", backup) // stderr notice (matches config init --force)
}

// PATTERN (reference, do NOT copy its guard): writeBootstrapFile :521-528 wraps the above in
//   if force { if _, err := os.Stat(path); err == nil { <above block> } }
// runConfigUpgrade has no `force` and has already gated no-file via os.ReadFile → insert the BARE inner block.

// WHY this is safe for existing tests: TestConfigUpgrade_* all call rootCmd.SetErr(io.Discard) → the stderr notice is
// discarded. None glob the dir / assert file counts → the .bak.* sibling is invisible. They assert stdout ("Upgraded …")
// + on-disk content — both unchanged. The V2ToV3Rewrite 2nd run hits !changed → returns before this block → no 2nd backup.
```

### Integration Points

```yaml
PRODUCTION CODE (internal/cmd/config.go):
  - runConfigUpgrade: +backup block (WriteTimestampedBackup call + exitcode error + stderr notice) before os.WriteFile.

NO database / migration / routes / new types / new imports / new functions / config-layer change.

DOWNSTREAM (this subtask ENABLES but does NOT write):
  - P1.M2.T2.S2 (test assertions): adds `filepath.Glob(config.toml.bak.*)` → ≥1 match to TestConfigUpgrade_OlderUpdated
    and TestConfigUpgrade_V2ToV3Rewrite (cloning config_test.go:631-634). NOT this task.

SCOPE FENCES: NO test change (S2); NO docs (FR-B8 already documented); NO change to upgradeConfigVersion, writeBootstrapFile,
  backup.go, or any gate; NO new import; the parallel sibling (P1.M2.T1.S2) edits internal/config/bootstrap_test.go — no overlap.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build (the insertion must compile — no new import; all symbols in scope).
go build ./...
# Expected: clean. A failure means a typo in the insertion or a missing symbol (there shouldn't be one — verify config/exitcode/fmt
# are imported, which they are: runConfigUpgrade already uses them).

# Vet.
go vet ./internal/cmd/...
# Expected: clean.

# Format.
gofmt -l internal/cmd/config.go
# Expected: empty. (The block is gofmt-conventional; if listed, gofmt -w it.)

# Lint.
make lint      # golangci-lint (staticcheck/gosimple/govet/errcheck/ineffassign/unused)
# Expected: zero errors. The new config.WriteTimestampedBackup call uses an existing exported func; errcheck is satisfied
#           (the error is checked and returned).

# Scope guard: only internal/cmd/config.go changed.
git diff --name-only
# Expected: internal/cmd/config.go (exactly ONE file).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The 3 existing upgrade tests — they MUST pass UNCHANGED (the fix is behaviorally invisible to them).
go test ./internal/cmd/ -run 'TestConfigUpgrade' -v
# Expected: PASS — OlderUpdated, V2ToV3Rewrite (incl. its 2nd-run idempotency check), Idempotent. They SetErr(io.Discard) and
#           don't count files, so the new .bak + stderr notice don't affect their assertions.

# Regression: the config init --force backup tests (the pattern source) stay green (writeBootstrapFile untouched).
go test ./internal/cmd/ -run 'TestConfigInit_Force' -v

# Full race suite.
make test
# Expected: green (race detector). This is the master gate.

# NOTE: the POSITIVE backup assertion for the upgrade path is S2 (P1.M2.T2.S2) — it is EXPECTED that no test yet asserts the
# upgrade backup exists. Do not add it here. (The fix is proven correct by code inspection against writeBootstrapFile + by the
# WriteTimestampedBackup unit tests in internal/config.)
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary.
make build

# Manual proof: config upgrade on an older config leaves a .bak sibling + prints the notice.
SC=/home/dustin/projects/stagecoach/bin/stagecoach
TESTDIR=$(mktemp -d)
export XDG_CONFIG_HOME="$TESTDIR"
# Plant an older-version config.
mkdir -p "$TESTDIR/stagecoach"
printf 'config_version = 1\n[generation]\nmax_md_lines = 7\n' > "$TESTDIR/stagecoach/config.toml"
# Run the upgrade (capture stderr to see the notice).
"$SC" config upgrade   # stdout: "Upgraded config at … to version 3."; stderr: "Backed up previous config to ….bak.…"
# Verify the backup exists and holds the PRIOR (v1) content.
ls "$TESTDIR/stagecoach/"config.toml.bak.* && echo "--- backup content (should be v1) ---" && cat "$TESTDIR/stagecoach/"config.toml.bak.*
# Verify the live config is now v3.
grep 'config_version = 3' "$TESTDIR/stagecoach/config.toml"
# Verify a no-op 2nd run creates NO new backup (idempotent — !changed early return).
before=$(ls "$TESTDIR/stagecoach/"config.toml.bak.* | wc -l)
"$SC" config upgrade   # "already at version 3 (no changes)"
after=$(ls "$TESTDIR/stagecoach/"config.toml.bak.* | wc -l)
[ "$before" = "$after" ] && echo "idempotent: no spurious backup on no-op run"
unset XDG_CONFIG_HOME; rm -rf "$TESTDIR"
# Expected: backup exists, holds v1 content; live config is v3; 2nd run adds no backup.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Scope guard 1: exactly ONE file changed.
git diff --name-only
# Expected: internal/cmd/config.go (only).

# Scope guard 2: exactly TWO config.WriteTimestampedBackup call sites in config.go now (writeBootstrapFile's + the new runConfigUpgrade one).
grep -n 'config.WriteTimestampedBackup' internal/cmd/config.go
# Expected: 2 hits — one in writeBootstrapFile (~:523), one in runConfigUpgrade (the new insertion).

# Scope guard 3: the new call sits AFTER the !changed gate and BEFORE os.WriteFile (ordering matters — backup before overwrite).
grep -n 'if !changed\|WriteTimestampedBackup\|os.WriteFile' internal/cmd/config.go
# Expected (in runConfigUpgrade): `if !changed` (:182) precedes `WriteTimestampedBackup` (new) precedes `os.WriteFile` (the overwrite).
#           (writeBootstrapFile's 3 lines appear separately ~:523-531.)

# Scope guard 4: the error message + notice match writeBootstrapFile verbatim (consistency).
grep -n 'backup existing config\|Backed up previous config to' internal/cmd/config.go
# Expected: 2 hits each (writeBootstrapFile + runConfigUpgrade) — identical strings.

# Scope guard 5: NO test file edited (S2 owns the assertions).
git diff --name-only | grep '_test.go'
# Expected: EMPTY (no test changes in this task).

# Scope guard 6: NO new import in config.go (the import block is unchanged).
git diff internal/cmd/config.go | grep -E '^\+.*"(fmt|os|exitcode|internal/config)"|^\+\s*"github.com'
# Expected: no NEW import lines (only the insertion block). The import block is byte-identical to before.

# Scope guard 7: backup.go UNTOUCHED (the reused primitive is not modified).
git diff --name-only | grep 'internal/config/backup.go'
# Expected: empty.

# Scope guard 8: the 3 upgrade tests pass UNCHANGED (the canary — proves the fix is behaviorally compatible).
go test ./internal/cmd/ -run 'TestConfigUpgrade' -v
# Expected: PASS (all 3).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/cmd/...` clean
- [ ] `gofmt -l internal/cmd/config.go` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) green — the 3 existing upgrade tests pass unchanged

### Feature Validation
- [ ] `runConfigUpgrade` calls `config.WriteTimestampedBackup(path)` after `!changed` and before `os.WriteFile`
- [ ] backup error → `exitcode.New(exitcode.Error, fmt.Errorf("backup existing config %s: %w", …))` (hard error, no clobber)
- [ ] successful backup → `fmt.Fprintf(cmd.OutOrStderr(), "Backed up previous config to %s\n", backup)`
- [ ] no-op paths (no-file / malformed / inert / already-current) skip the backup (early returns unchanged)
- [ ] manual: older config upgrade leaves a `.bak.<timestamp>` with prior content + prints the notice; 2nd no-op run adds no backup

### Scope-Boundary Validation
- [ ] `git diff --name-only` == only `internal/cmd/config.go`
- [ ] NO test file edited (S2 owns assertions)
- [ ] NO docs change (FR-B8 already documented)
- [ ] NO new import; NO new function; backup.go + writeBootstrapFile + upgradeConfigVersion UNTOUCHED
- [ ] NO overlap with parallel sibling P1.M2.T1.S2 (it edits internal/config/bootstrap_test.go)

### Code Quality & Docs
- [ ] The block mirrors writeBootstrapFile's inner if/else-if verbatim (same error string, same notice, same severity, same stream)
- [ ] Comment cites FR-B8 + explains why no os.Stat guard (file provably exists)
- [ ] The backup contains the PRIOR content (created before the overwrite)

---

## Anti-Patterns to Avoid

- ❌ Don't copy writeBootstrapFile's outer `if force { if _, err := os.Stat(path); err == nil { … } }` guard into runConfigUpgrade.
  writeBootstrapFile needs it because `config init` can be a first-time write (no existing file). runConfigUpgrade has NO `force`
  concept and has ALREADY proved the file exists (`os.ReadFile` at :159 returns "no config file" on IsNotExist). Insert the BARE inner
  if/else-if block. (WriteTimestampedBackup is nil-safe for a missing file regardless, so even defensively the guard is redundant.)
- ❌ Don't edit any test file. The 3 existing upgrade tests stay GREEN unchanged (they `SetErr(io.Discard)` and don't count files —
  verified). The POSITIVE backup assertions (glob `config.toml.bak.*`) are S2 (P1.M2.T2.S2). Editing tests here is scope creep and
  conflicts with S2.
- ❌ Don't add a new import. `config` (internal/config), `exitcode`, and `fmt` are ALL already imported and used in runConfigUpgrade's
  body (it calls config.ResolveConfigPath/CurrentConfigVersion/IsInert, exitcode.New, fmt.Errorf/Fprintf today). The insertion reuses
  them. Adding a redundant import is a compile error (unused) or lint noise.
- ❌ Don't add a new helper function. `config.WriteTimestampedBackup` already does exactly this (backup.go:18). Reuse it — do not
  reinvent a backup routine or inline the copy logic.
- ❌ Don't move the insertion above the `if !changed` gate. The backup must fire ONLY when `changed==true` (a real overwrite). Placing
  it before the gate would create spurious backups on already-current/inert/malformed runs (and would break the V2ToV3Rewrite
  2nd-run idempotency expectation, even though that test doesn't assert backups — it's still wrong semantics).
- ❌ Don't change the error message or notice text. They MUST match writeBootstrapFile verbatim ("backup existing config %s: %w" and
  "Backed up previous config to %s\n") for user-facing consistency across the two commands. Diverging creates a inconsistent UX.
- ❌ Don't treat a backup failure as a soft warning. FR-B8 / writeBootstrapFile treat it as a HARD error (exitcode.Error) — never clobber
  a user's config without a recoverable copy. `return exitcode.New(...)`, do not fall through to os.WriteFile.
- ❌ Don't print the notice to stdout. The "Upgraded config at …" confirmation is on stdout (pipeable); the backup notice is DIAGNOSTIC
  → stderr (`cmd.OutOrStderr()`), matching writeBootstrapFile. Mixing them would pollute the pipeable stdout for users who script
  `config upgrade`.
- ❌ Don't touch backup.go, writeBootstrapFile, upgradeConfigVersion, or any gate. This is a single localized insertion in ONE function.
  The reusable primitive, the reference pattern, the transform, and the early-return gates are all correct as-is.

---

## Confidence Score: 10/10

This is a single ~6-line insertion that copies the INNER block of a proven, fully-tested pattern (writeBootstrapFile's backup call)
into the one function that was missing it, reusing an existing exported primitive (`config.WriteTimestampedBackup`) with no new import,
no new function, no new type. The exact insertion point (anchored by string), the verbatim code, the reason no `os.Stat` guard is needed,
the WriteTimestampedBackup contract (3 return cases), and — critically — the proof that all 3 existing upgrade tests stay green (they
discard stderr and don't count files; the 2nd-run idempotency check holds because `!changed` returns before the block) are all spelled
out. The scope is maximally tight (one file, one function, production-only; tests are S2; no docs; no overlap with the parallel sibling).
There is no new logic to get wrong — the backup either happens before the overwrite (correct) or it doesn't (the bug). One-pass success
is essentially guaranteed.
