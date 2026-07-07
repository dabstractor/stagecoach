---
name: "P1.M2.T1.S1 — Rename STAGEHAND_* env var literals to STAGECOACH_* across all Go files"
description: |
  Bulk mechanical rename of the uppercase `STAGEHAND_` prefix to `STAGECOACH_` in all `.go` files
  (part of the stagehand→stagecoach project rename, PRD h2.30). The prefix appears as **404 occurrences**
  across ~22 Go files — ALL string literals, comments, or test function names (ZERO Go identifiers/consts;
  verified). The prefix is NOT a single constant; each env var name is a literal string. A single
  case-sensitive sed (`s/STAGEHAND_/STAGECOACH_/g` on `--include='*.go'`) handles every occurrence
  consistently: string literals, comments, error messages, the per-role prefix literal, AND the stub-binary
  env-var coupling (stubtest.go setter ↔ cmd/stubagent/main.go reader). After the rename: zero `STAGEHAND_`
  remain in .go files, and `go test ./...` passes with the new `STAGECOACH_*` names.

  ⚠️ **THE central design call — one case-sensitive sed on *.go files; it is provably safe.** Every
  `STAGEHAND_` occurrence is a string literal, a comment, or a test function name — NOT a Go identifier
  (const/var/type). So the sed cannot break any identifier resolution, import, or type assertion. The two
  test function names it renames (`TestLoad_STAGEHAND_CONFIG_EnvPath` → `TestLoad_STAGECOACH_CONFIG_EnvPath`)
  are harmless — Go's test runner discovers by `Test` prefix. The sed is case-sensitive (uppercase only),
  so lowercase `stagehand_` (git-config keys, error prefixes, import paths) is untouched — those are other
  tasks (P1.M2.T1.S2, P1.M2.T3, P1.M1).

  ⚠️ **THE second design call — the stub-binary coupling MUST rename consistently.** `internal/stubtest/
  stubtest.go` SETS `STAGEHAND_STUB_*` env vars (optsEnvMap); `cmd/stubagent/main.go` READS them via
  `os.Getenv`. The sed renames BOTH files → setter writes `STAGECOACH_STUB_*`, reader reads
  `STAGECOACH_STUB_*` → they match. Tests that set stub env vars DIRECTLY (e.g., `STAGEHAND_STUB_MARKER`
  in lock_scenarios_test.go) are also renamed. The sed's `grep -rl` scope includes ALL .go files, so the
  coupling is handled atomically.

  ⚠️ **THE third design call — sed portability (GNU vs BSD).** The contract's `sed -i 's/.../.../g'` is the
  GNU form; macOS/BSD `sed -i` requires a backup-suffix (`sed -i '' ...` or `sed -i.bak ...`). CI runs on
  ubuntu-latest (GNU sed → contract's form works). For cross-platform dev safety, use `sed -i.bak` then
  `find . -name '*.bak' -delete`, OR detect the platform. Also: `xargs -r` (GNU) prevents the empty-input
  hang if grep returned nothing (in practice 404 hits guarantees non-empty).

  SCOPE: ALL `STAGEHAND_` → `STAGECOACH_` in `--include='*.go'` files (case-sensitive, uppercase only).
  NO lowercase `stagehand_`, NO non-.go files (docs/.toml/.yml/Makefile), NO import paths (already renamed
  by P1.M1.T1). INPUT = the compiled project from M1 (identifiers already `Stagecoach`). OUTPUT = zero
  `STAGEHAND_` in .go files; `go test ./...` green with `STAGECOACH_*` names. DOCS: Mode A — docs/cli.md
  and docs/configuration.md updated in P1.M4 (separate docs task).
---

## Goal

**Feature Goal**: Rename every uppercase `STAGEHAND_` prefix to `STAGECOACH_` across all Go source files
(string literals, comments, error messages, test function names, the per-role prefix literal, and the
stub-binary env-var coupling) so the env var surface matches the stagecoach identity per PRD FR35
("Environment variables use the `stagecoach_` prefix" — the uppercase form `STAGECOACH_*`).

**Deliverable**: A single bulk sed pass (`s/STAGEHAND_/STAGECOACH_/g`) on all `*.go` files containing the
prefix, followed by a zero-remaining verification and a full test-suite run. ~404 occurrences across ~22
files. No new files; edits only.

**Success Definition**: `grep -rn 'STAGEHAND_' --include='*.go' . | grep -v '.git/'` returns **zero**
results; `go build ./...`, `go vet ./...`, `gofmt -l`, `go test ./... -count=1` all green (the renamed
env vars are set/read consistently). go.mod/go.sum unchanged. No lowercase `stagehand_` touched (other
tasks). No non-.go files touched.

## User Persona

**Target User**: Users + CI who set stagecoach env vars (`STAGECOACH_PROVIDER`, `STAGECOACH_MODEL`,
`STAGECOACH_VERBOSE`, `STAGECOACH_STUB_OUT`, etc.). Transitively PRD §9.8 FR35 + §15.2 (env var column).

**Use Case**: `STAGECOACH_PROVIDER=pi stagecoach` (env-var override) works and is read correctly.

**Pain Points Addressed**: the env var surface matches the new project name; no stale `STAGEHAND_*`
references remain to confuse users or docs.

## Why

- **Completes the env-var surface rename.** FR35 documents `stagecoach_`; the code must match (the
  uppercase `STAGECOACH_*` literals). Without this, env vars are `STAGEHAND_*` while everything else says
  stagecoach — an inconsistency that confuses users and docs.
- **Mechanical + safe.** A case-sensitive sed on .go files; all occurrences are values/comments (verified
  zero identifiers). The stub-binary coupling is handled atomically (both setter and reader are .go).
- **One task, one pass.** The prefix is NOT a single constant — it's ~30+ literal strings across ~22
  files. The sed is the ONLY approach that catches all of them consistently without manual per-site edits.

## What

Every uppercase `STAGEHAND_` literal in `.go` files becomes `STAGECOACH_`. Nothing else changes
(lowercase, non-.go, identifiers, imports are out of scope). The full test suite passes with the new names.

### Success Criteria

- [ ] `grep -rn 'STAGEHAND_' --include='*.go' . | grep -v '.git/'` returns **zero** results.
- [ ] The per-role prefix at load.go:263 is `"STAGECOACH_" + strings.ToUpper(role)`.
- [ ] stubtest.go sets `STAGECOACH_STUB_*` AND cmd/stubagent/main.go reads `STAGECOACH_STUB_*` (consistent).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l`, `go test ./... -count=1` all green.
- [ ] go.mod/go.sum unchanged; no non-.go files touched; no lowercase `stagehand_` changed.

## All Needed Context

### Context Completeness Check

_Pass._ A developer with no prior knowledge can implement this from: the sed command (quoted below), the
portability note, the zero-remaining verification, and the test-suite gate. No feature/design knowledge
required — this is a mechanical literal-prefix rename.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/012_963e3918ec08/P1M2T1S1/research/env_var_rename.md
  why: the FULL surface (404 occurrences, ~22 files), the proof that ZERO are Go identifiers (sed is safe),
       the stub-binary coupling (setter↔reader), the per-role prefix literal, the sed portability note,
       and the scope boundary (uppercase-only, .go-only).
  critical: ALL `STAGEHAND_` occurrences are string literals / comments / test names — NOT identifiers.
       The sed cannot break compilation. The stub-binary coupling (stubtest.go ↔ cmd/stubagent/main.go)
       is handled because BOTH are .go files in the sed scope.

- file: internal/config/load.go   (loadEnv: the primary site — ~46 occurrences)
  why: the env-var reader. `os.LookupEnv("STAGEHAND_PROVIDER")`, `_MODEL`, `_REASONING`, `_TIMEOUT`,
       `_VERBOSE`, `_NO_COLOR`, `_COMMITS`, `_FORMAT`, `_LOCALE`, `_TEMPLATE`, `_PUSH`, `_NO_VERIFY`;
       error messages (`fmt.Errorf("STAGEHAND_TIMEOUT: %w", err)`); the per-role prefix at :263
       (`prefix := "STAGEHAND_" + strings.ToUpper(role)`); STAGEHAND_CONFIG path resolution at :89.
  pattern: all are `os.LookupEnv("STAGEHAND_*")` / `os.Getenv("STAGEHAND_*")` — the sed renames the
           string literal inside the quotes AND the error-message literals AND the prefix literal.

- file: internal/stubtest/stubtest.go   + cmd/stubagent/main.go   (the stub-binary coupling)
  why: stubtest.go optsEnvMap SETS `"STAGEHAND_STUB_OUT"` / `_EXIT` / `_SLEEP_MS` / `_STDERR` / `_SCRIPT`
       / `_COUNTER` / `_ARGSFILE`; stubagent main.go READS `os.Getenv("STAGEHAND_STUB_*")` (+ `_STDINFILE`,
       `_MARKER`). The sed renames BOTH → `STAGECOACH_STUB_*` consistently.
  gotcha: if only ONE side were renamed, the stub binary would silently fail to read its knobs. The sed's
           `grep -rl --include='*.go'` scope catches both — but VERIFY post-sed that both sides match.

- file: internal/config/load_test.go   (~83 occurrences — the most of any file)
  why: `t.Setenv("STAGEHAND_PROVIDER", ...)` etc. throughout. The sed renames them → tests set
       `STAGECOACH_*` which loadEnv now reads. Also: test function names `TestLoad_STAGEHAND_CONFIG_EnvPath`
       → `TestLoad_STAGECOACH_CONFIG_EnvPath` (harmless — Go discovers by `Test` prefix).

- file: internal/config/bootstrap.go   (~14 occurrences in the template-comment string)
  why: the bootstrap config template lists env vars as comments (lines 243-262). The sed renames within
       the Go string literal → the generated config comments say `STAGECOACH_*`.

- file: internal/e2e/harness_test.go   (STAGEHAND_RUN_REAL)
  why: the e2e real-agent opt-in env var. `os.Getenv("STAGEHAND_RUN_REAL")` → `STAGECOACH_RUN_REAL`.
       Set by developers opting into real-agent e2e tests. The sed handles it.
```

### Current Codebase tree (relevant slice — the ~22 affected files)

```bash
# All .go files containing 'STAGEHAND_' (the sed's grep -rl scope):
internal/config/load.go            # ~46 (loadEnv — the primary site)
internal/config/load_test.go       # ~83 (t.Setenv calls — the most)
internal/cmd/root.go               # ~24 (flag help text)
internal/cmd/default_action_test.go # ~23
pkg/stagecoach/stagecoach_test.go  # ~22
internal/e2e/lock_scenarios_test.go # ~22
internal/stubtest/stubtest.go      # ~17 (stub env-var SETTER)
internal/cmd/config.go             # ~16 (Long descriptions, template comments)
internal/e2e/scenarios_test.go     # ~14
internal/config/bootstrap.go       # ~14 (template comment string)
cmd/stubagent/main.go              # ~13 (stub env-var READER — the coupling)
internal/e2e/harness_test.go       # ~10 (STAGEHAND_RUN_REAL)
internal/cmd/config_test.go        # ~9
internal/generate/realagent_test.go # ~8
internal/hook/script_test.go       # ~7
internal/e2e/hook_scenarios_test.go # ~7
internal/generate/generate_multiturn_failure_test.go # ~6
internal/decompose/decompose_test.go # ~6
internal/cmd/hook_test.go          # ~6
internal/ui/output_test.go         # ~5
# + a handful more (1-4 each in ~8 other files)
go.mod / go.sum                    # UNCHANGED (no new dep; sed is on source content only)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. The sed edits the ~22 listed .go files IN PLACE. No structural change.
```

### Known Gotchas of our codebase & Library Quirks

```bash
# CRITICAL: the sed is case-sensitive (STAGEHAND_ uppercase only). It does NOT touch lowercase 'stagehand_'
# (git-config keys, error prefixes, import paths). Those are separate tasks. Do NOT make the sed
# case-insensitive — it would over-rename into identifiers/comments that are other tasks' scope.

# CRITICAL: the stub-binary coupling. stubtest.go SETS the env vars; cmd/stubagent/main.go READS them.
# The sed renames BOTH (both are .go). VERIFY post-sed: grep both files for STAGECOACH_STUB_ and confirm
# the keys match (OUT, EXIT, SLEEP_MS, STDERR, SCRIPT, COUNTER, ARGSFILE, STDINFILE, MARKER).

# CRITICAL: sed -i portability. GNU sed: `sed -i 's/.../.../g'`. BSD/macOS: `sed -i '' 's/.../.../g'` or
# `sed -i.bak 's/.../.../g'`. CI is ubuntu (GNU). For cross-platform dev, use `sed -i.bak` + cleanup, OR
# detect the platform. Do NOT assume GNU sed on a macOS dev machine.

# CRITICAL: the per-role prefix at load.go:263: `prefix := "STAGEHAND_" + strings.ToUpper(role)`. The sed
# renames the literal → `"STAGECOACH_" + strings.ToUpper(role)`. This is the runtime-constructed prefix
# for STAGECOACH_PLANNER_PROVIDER etc. Verify it landed.

# GOTCHA: test function names containing STAGEHAND_ (TestLoad_STAGEHAND_CONFIG_EnvPath) ARE renamed by the
# sed (they're file content). This is HARMLESS — Go discovers test functions by the Test prefix, not by name.
# No explicit calls to update (Go's test runner uses reflection).

# GOTCHA: bootstrap.go's template-comment string (lines 243-262) is a Go STRING LITERAL containing
# STAGEHAND_* as comment text for the generated config. The sed renames within the string → the generated
# config template now documents STAGECOACH_* env vars. Correct.

# GOTCHA: xargs with empty input would hang (sed reads stdin). In practice 404 hits guarantees non-empty.
# Belt-and-suspenders: `xargs -r` (GNU) or verify grep output before piping.

# GOTCHA: do NOT run the sed on non-.go files (docs, .toml, .yml, Makefile). Those are P1.M3/P1.M4's scope.
# The `--include='*.go'` flag scopes it correctly.

# GOTCHA: gofmt is not needed after a pure string-content sed (no structural change). But run it as a
# belt-and-suspenders check — `gofmt -l` should be clean (the sed doesn't touch formatting).
```

## Implementation Blueprint

### Data models and structure

N/A — no types, no data models. A bulk string-literal rename.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: RUN the bulk sed (the rename)
  - COMMAND (GNU sed / CI ubuntu):
    grep -rl 'STAGEHAND_' --include='*.go' . | grep -v '.git/' | xargs sed -i 's/STAGEHAND_/STAGECOACH_/g'
  - PORTABLE alternative (works on GNU + BSD/macOS):
    grep -rl 'STAGEHAND_' --include='*.go' . | grep -v '.git/' | xargs sed -i.bak 's/STAGEHAND_/STAGECOACH_/g'
    find . -name '*.bak' -delete
  - WHY: handles ALL 404 occurrences (string literals, comments, error messages, per-role prefix, stub
    coupling, test function names) in one atomic pass.

Task 2: VERIFY zero remaining
  - RUN: grep -rn 'STAGEHAND_' --include='*.go' . | grep -v '.git/'
  - EXPECT: zero output. If ANY result, re-run the sed on the missed file (grep -rl should have caught it;
    check for unusual file paths or permission issues).

Task 3: VERIFY the stub-binary coupling
  - RUN: grep -n 'STAGECOACH_STUB_' internal/stubtest/stubtest.go cmd/stubagent/main.go
  - EXPECT: both files use STAGECOACH_STUB_* consistently (the setter's keys == the reader's keys).
  - ALSO: grep -rn 'STAGECOACH_STUB_MARKER\|STAGECOACH_STUB_STDINFILE' --include='*.go' . # confirm the
    directly-set stub vars are renamed too.

Task 4: VERIFY the per-role prefix
  - RUN: grep -n 'STAGECOACH_.*strings.ToUpper' internal/config/load.go
  - EXPECT: `prefix := "STAGECOACH_" + strings.ToUpper(role)` at ~line 263.

Task 5: BUILD + TEST
  - RUN: gofmt -l internal/ pkg/ cmd/ (expect clean); go vet ./...; go build ./...;
    go test ./... -count=1.
  - EXPECT: all green. The renamed env vars (STAGECOACH_*) are set by tests AND read by loadEnv/stubagent
    consistently → tests pass. If a test FAILS, a STAGEHAND_ literal was missed (re-check Task 2) OR the
    stub coupling is broken (re-check Task 3).

Task 6: FINAL grep audit + scope check
  - RUN: grep -rn 'STAGEHAND_' --include='*.go' . | grep -v '.git/' → zero.
  - RUN: git diff --stat → confirm only .go files changed (no .md/.toml/.yml/Makefile).
  - RUN: git diff --exit-code go.mod go.sum → unchanged.
```

### Implementation Patterns & Key Details

```bash
# The sed (GNU — CI):
grep -rl 'STAGEHAND_' --include='*.go' . | grep -v '.git/' | xargs sed -i 's/STAGEHAND_/STAGECOACH_/g'

# The sed (portable — GNU + BSD/macOS):
grep -rl 'STAGEHAND_' --include='*.go' . | grep -v '.git/' | xargs sed -i.bak 's/STAGEHAND_/STAGECOACH_/g'
find . -name '*.bak' -delete

# The zero-remaining gate:
grep -rn 'STAGEHAND_' --include='*.go' . | grep -v '.git/'   # MUST be empty

# The stub-coupling consistency check:
diff <(grep -oP 'STAGECOACH_STUB_\w+' internal/stubtest/stubtest.go | sort -u) \
     <(grep -oP 'STAGECOACH_STUB_\w+' cmd/stubagent/main.go | sort -u)   # setter keys ⊆ reader keys
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — pure source-content rename; no dep change. go mod tidy is a no-op.

FROZEN / NOT-EDITED:
  - Lowercase stagehand_ (git-config keys stagehand.provider, error prefixes stagehand:, import paths) —
    P1.M2.T1.S2 (git config), P1.M2.T3 (user-facing strings), P1.M1.T1 (import paths, Complete).
  - Non-.go files (docs/cli.md, docs/configuration.md, providers/*.toml, .goreleaser.yaml, Makefile,
    .github/workflows/*.yml) — P1.M3 (build/CI), P1.M4 (docs).
  - Go identifiers (Stagehand→Stagecoach in function/var names) — P1.M1.T2.S1 (Implementing). The uppercase
    STAGEHAND_ string literals survived that rename (they're values, not names).

DOWNSTREAM / RELATED:
  - P1.M2.T1.S2 (next): renames lowercase stagehand.* git-config keys to stagecoach.* — same pattern
    (sed on .go files), different target (lowercase). These two tasks are independent but adjacent.
  - P1.M4 (docs): updates docs/cli.md + docs/configuration.md env-var references — the .md twin of this task.
  - P1.M5.T2.S1 (final grep audit): confirms zero stagehand references in ALL tracked files.

NO DATABASE / NO ROUTES / NO CONFIG CODE CHANGE (the env var names are data, not logic).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# The sed does not change Go structure (only string-literal content). gofmt should be clean:
gofmt -l internal/ pkg/ cmd/   # expect: empty (no formatting drift from a content-only sed)
go vet ./...                    # expect: clean (no broken identifiers — verified zero STAGEHAND_ identifiers)
go build ./...                  # expect: success
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
```

### Level 2: Unit Tests (Component Validation) — the consistency gate

```bash
# The renamed env vars must be set AND read consistently. The full test suite exercises this:
go test ./internal/config/... -count=1 -v   # loadEnv reads STAGECOACH_*; load_test.go sets STAGECOACH_*
go test ./internal/stubtest/... -count=1     # stubtest sets STAGECOACH_STUB_*; (indirectly tests the coupling)
go test ./... -count=1                       # FULL suite — every test that sets/reads an env var
# Expected: ALL PASS. A failure means a STAGEHAND_ literal was missed (the setter says STAGECOACH_ but the
#   reader still says STAGEHAND_, or vice versa). Re-check Task 2 (zero remaining) + Task 3 (stub coupling).
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# THE zero-remaining gate:
grep -rn 'STAGEHAND_' --include='*.go' . | grep -v '.git/' && echo "BAD: STAGEHAND_ remains" || echo "zero STAGEHAND_ in .go (good)"
# Confirm only .go files changed (no .md/.toml/.yml/Makefile touched):
git diff --name-only | grep -vE '\.go$' && echo "BAD: non-.go file changed" || echo "only .go files changed (good)"
# Confirm the stub coupling is consistent (setter keys == reader keys):
echo "setter:"; grep -oP 'STAGECOACH_STUB_\w+' internal/stubtest/stubtest.go | sort -u
echo "reader:"; grep -oP 'STAGECOACH_STUB_\w+' cmd/stubagent/main.go | sort -u
# Confirm the per-role prefix:
grep -n 'STAGECOACH_".*ToUpper' internal/config/load.go   # prefix := "STAGECOACH_" + strings.ToUpper(role)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Env-var parity audit (optional): confirm every env var documented in PRD §15.2 / FR35 has a STAGECOACH_
# reader in load.go. The sed should have caught them all; this is belt-and-suspenders:
for v in PROVIDER MODEL REASONING TIMEOUT VERBOSE NO_COLOR CONFIG COMMITS FORMAT LOCALE TEMPLATE PUSH NO_VERIFY; do
  grep -q "STAGECOACH_$v" internal/config/load.go || echo "MISSING: STAGECOACH_$v not in load.go"
done
# Expected: no MISSING lines (all 13 global env vars have STAGECOACH_ readers). (Per-role prefix is runtime-
# constructed so it won't appear as a literal — verify via Task 4.)
# golangci-lint: `make lint` (project-wide gate — the rename doesn't introduce lint issues; it's content-only).
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `gofmt -l`, `go vet ./...`, `go build ./...`, `go mod tidy` no-op.
- [ ] Level 2 green: `go test ./... -count=1` (all env-var set/read pairs consistent).
- [ ] Level 3: `grep -rn 'STAGEHAND_' --include='*.go' .` = zero; only .go files changed; stub coupling
      consistent; per-role prefix is `STAGECOACH_`.

### Feature Validation

- [ ] Zero `STAGEHAND_` in any .go file.
- [ ] stubtest.go and cmd/stubagent/main.go both use `STAGECOACH_STUB_*` (matching keys).
- [ ] load.go per-role prefix: `"STAGECOACH_" + strings.ToUpper(role)`.
- [ ] `go test ./...` green with the new names.

### Code Quality Validation

- [ ] The sed was case-sensitive (uppercase only); lowercase `stagehand_` untouched.
- [ ] Only .go files changed; no .md/.toml/.yml/Makefile.
- [ ] No Go identifiers broken (verified zero STAGEHAND_ identifiers pre-sed).
- [ ] No scope creep into lowercase git-config keys (P1.M2.T1.S2), docs (P1.M4), or imports (P1.M1).

### Documentation & Deployment

- [ ] No docs edited here (Mode A docs/cli.md + configuration.md are P1.M4's scope).
- [ ] go.mod/go.sum byte-unchanged; no new files.

---

## Anti-Patterns to Avoid

- ❌ Don't make the sed case-insensitive (`s/STAGEHAND_/STAGECOACH_/gi`). It would over-rename into lowercase
  `stagehand_` (git-config keys, error prefixes, import paths) that are OTHER tasks' scope. Uppercase only.
- ❌ Don't run the sed on non-.go files. The `--include='*.go'` flag scopes it correctly. Docs/.toml/.yml/
  Makefile are P1.M3/P1.M4.
- ❌ Don't manually edit each file. The prefix is NOT a single constant — it's ~404 literal occurrences
  across ~22 files. The sed is the only approach that catches them all consistently. Manual edits WILL miss
  some (especially in test files, comments, and the stub-binary reader).
- ❌ Don't forget to verify the stub-binary coupling. If stubtest.go renames but cmd/stubagent/main.go
  doesn't (or vice versa), the stub binary silently fails to read its knobs. The sed handles both (both are
  .go), but VERIFY post-sed that the keys match.
- ❌ Don't assume GNU sed on macOS. `sed -i 's/.../.../g'` (GNU) fails on BSD/macOS (`sed -i ''`). Use
  `sed -i.bak` + cleanup, OR detect the platform. CI is ubuntu (GNU); dev may be macOS.
- ❌ Don't skip the per-role prefix check. load.go:263 `"STAGEHAND_" + strings.ToUpper(role)` is a LITERAL
  the sed catches, but it's the runtime-constructed prefix for per-role env vars — verify it landed as
  `"STAGECOACH_"`.
- ❌ Don't worry about test function names being renamed. `TestLoad_STAGEHAND_CONFIG_EnvPath` →
  `TestLoad_STAGECOACH_CONFIG_EnvPath` is harmless (Go discovers by `Test` prefix). No explicit calls to fix.
- ❌ Don't touch go.mod/go.sum. This is a source-content rename; no dependency change. `go mod tidy` is a
  no-op.
- ❌ Don't skip `go test ./... -count=1`. It is THE consistency gate — if any env-var setter/reader pair is
  mismatched (one side renamed, the other not), a test fails. The `-count=1` disables test caching (forces
  re-run with the new names).
- ❌ Don't skip the zero-remaining grep. `grep -rn 'STAGEHAND_' --include='*.go' .` MUST return empty. A
  single remaining literal means the sed missed a file (unusual, but verify).
