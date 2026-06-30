# PRP — P1.M4.T1.S1 (bugfix-002): dry-run `RescueError` branch in `handleGenError` (exit 1, short message, no recipe)

**Issue**: bugfix-002 Issue 4 (Minor) — `--dry-run` can exit 3 (rescue) or 124 (timeout) and prints the full §18.3 rescue recipe, which is surprising for a "preview" command that was never going to commit.
**PRD refs**: §9.12 / FR49 (dry-run runs the full pipeline, incl. the write-tree snapshot, but does not commit); §15.4 exit codes; §18.3 (rescue recipe).
**Binding analysis**: `plan/001_f1f80943ac34/bugfix/002_18158df10968/architecture/issue_analysis.md` **ISSUE 4**.

---

## Goal

**Feature Goal**: Special-case `--dry-run` generation failures in the CLI's `handleGenError` so a dry-run timeout or parse/dedupe-exhaustion (rescue) exits **1** with a **short, single-line stderr message** and **NO manual `git commit-tree` recovery recipe** — instead of exit 124/3 + the full rescue block. The library API (`pkg/stagehand`) is **unchanged**: it still returns `*generate.RescueError`; only the CLI rendering wraps a dry-run failure to exit 1.

**Deliverable**:
1. **Code** (~7 lines): one new branch at the **TOP** of `internal/cmd/default_action.go::handleGenError`, before the existing `errors.As(err, &re)` block. It reads the package var `flagDryRun` directly (same package; no signature/import change).
2. **Tests**: two new CLI tests in `internal/cmd/default_action_test.go` — (a) `--dry-run` + timeout → exit 1 + short timeout message + no recipe; (b) `--dry-run` + rescue (blank stub) → exit 1 + short message + no recipe.
3. **Docs (Mode A)**: `docs/cli.md` — the `--dry-run` Global-flags row (line ~26) and the "Exit codes" table/section (line ~76) note that a dry-run generation failure exits 1 with a short message (not 3/124 + a recovery recipe).

**Success Definition**:
- `go build ./...`, `go vet ./...`, `go test -race ./internal/cmd/... ./pkg/stagehand/...`, `go test -race ./...`, `make lint` all green.
- `stagehand --dry-run` against a timeout stub → exit 1 + stderr `generation timed out; run without --dry-run to see the recovery recipe` + NO `git commit-tree` recipe.
- `stagehand --dry-run` against an unparseable/blank stub (parse/dedupe exhaustion) → exit 1 + stderr `could not generate a commit message; run without --dry-run to see retries and the recovery recipe` + NO recipe.
- The **commit-path** CLI tests (`TestRunDefault_Rescue` → exit 3; `TestRunDefault_Timeout` → exit 124) and the **library** test (`pkg/stagehand` `TestGenerateCommit_Timeout` "dryrun" → library returns `*RescueError{Kind:ErrTimeout}`, `exitcode.For == 124`) are **unchanged and still pass**.

## Why

- **User impact**: `--dry-run` is the "preview a message before trusting it" command (US9). On generation failure, a user reasonably expects either a message or a clear "couldn't generate" outcome — not a non-zero exit with a multi-line `git commit-tree … | xargs git update-ref` recovery recipe for a commit that was never going to happen. A script doing `msg=$(stagehand --dry-run)` currently gets a non-zero exit + no message + a confusing recipe.
- **Consistent with FR49**: dry-run already (correctly, per bugfix-001) runs the full pipeline incl. the snapshot. That means a dry-run failure legitimately surfaces a `*RescueError` from the library. The CLI is the right place to translate that into a dry-run-appropriate *outcome* (exit 1, no recipe) without perturbing the library's stable, well-tested contract.
- **Minimal blast radius**: a single CLI function + package-var read. The library, the registry, the parser, and the config loaders are untouched.

## What

### User-visible behavior (after fix)
- `stagehand --dry-run` (timeout) → exit 1; stderr single line `generation timed out; run without --dry-run to see the recovery recipe`; no recipe.
- `stagehand --dry-run` (parse/dedupe exhaustion / rescue) → exit 1; stderr single line `could not generate a commit message; run without --dry-run to see retries and the recovery recipe`; no recipe.
- `stagehand --dry-run` (success) → exit 0 + message on stdout (unchanged).
- `stagehand` (no `--dry-run`) timeout/rescue → exit 124/3 + full rescue recipe (unchanged).
- Dry-run CAS / nothing-to-commit / generic errors → exit 1/2 as before (the new branch intercepts ONLY `*generate.RescueError`).

### Success Criteria
- [ ] New branch at the TOP of `handleGenError`, guarded on `flagDryRun` AND `errors.As(err, &re *generate.RescueError)`.
- [ ] Default message is the rescue wording; `errors.Is(err, generate.ErrTimeout)` overrides to the timeout wording.
- [ ] `fmt.Fprintln(stderr, msg)` then `return exitcode.New(exitcode.Error, nil)` (exit 1, silent so main does not double-print).
- [ ] `handleGenError` signature unchanged; no new imports beyond what the file already uses (`errors`, `fmt`, `io`, `generate`, `exitcode` — all present).
- [ ] The existing non-dry-run rescue/timeout/CAS/nothing/generic branches are untouched.
- [ ] Two new CLI tests pass; the three regression-guard tests (`TestRunDefault_Rescue`, `TestRunDefault_Timeout`, `TestGenerateCommit_Timeout` "dryrun") pass unchanged.

## All Needed Context

### Context Completeness Check
✅ Passes the "No Prior Knowledge" test: the exact function, insertion point, package-var seam, error sentinels, exit-code mapping, exact message wording, and test patterns (with fixture helpers) are all specified below.

### Documentation & References

```yaml
# MUST READ — binding analysis for this exact fix
- file: plan/001_f1f80943ac34/bugfix/002_18158df10968/architecture/issue_analysis.md
  section: ISSUE 4 (Minor) — --dry-run can exit 3/124 + full rescue recipe
  why: THE binding root-cause + fix analysis. Contains the exact ~7-line patch, the "library unchanged,
       only CLI wraps" rationale, the regression guards (commit-path tests + library test), and the
       docs/cli.md touch points (line ~26 and ~76).
  critical: Insert the branch BEFORE the existing errors.As(err,&re) block. flagDryRun is a package var
            (root.go:40) read directly — no signature change. Do NOT modify the library or its test.

# The function to edit
- file: internal/cmd/default_action.go
  lines: 168-188 (handleGenError); insert the new branch at the TOP of the body (before the
         `var re *generate.RescueError` block at line 171).
  why: THE single seam. It already does errors.As(err,&re) + errors.Is(err,generate.ErrTimeout) +
       FormatRescue + exitcode.New(code,nil). Reuse that exact discrimination; just gate it on flagDryRun
       and substitute the short message + exit 1.
  pattern: Mirror the existing silent pattern: fmt.Fprintln(stderr, ...) then return exitcode.New(code, nil).

# The package var to read (same package — direct access)
- file: internal/cmd/root.go
  lines: 40 (var flagDryRun bool); 89 (pf.BoolVar(&flagDryRun, "dry-run", false, "..."))
  why: flagDryRun is in scope inside handleGenError with no new import/parameter. PersistentPreRunE
       populates it from the parsed --dry-run flag before RunE runs.
  gotcha: saveRootState/restoreRootState + resetFlags (root_test.go:104-137) reset every changed flag
          (incl. --dry-run) back to DefValue "false" in t.Cleanup, so flagDryRun does NOT leak across
          tests. No special reset handling needed in the new tests.

# The error sentinels + RescueError (confirmed; reused as-is)
- file: internal/generate/generate.go
  lines: 54 (var ErrTimeout); 59 (var ErrRescue); 76-93 (type RescueError{Kind error; ...}; Unwrap returns Kind)
  why: errors.As(err, &*generate.RescueError) matches BOTH timeout and rescue. errors.Is(err, generate.ErrTimeout)
       discriminates (RescueError.Unwrap() returns e.Kind). pkg/stagehand.runPipeline returns
       *RescueError{Kind:ErrTimeout|ErrRescue} on dry-run failures — unchanged by this task.

# exitcode values (confirmed)
- file: internal/exitcode/exitcode.go
  lines: 22-27 (Success 0, Error 1, NothingToCommit 2, Rescue 3, Timeout 124); 48 (New(code,err))
  why: exitcode.New(exitcode.Error, nil) → exit 1 + SILENT (main prints nothing extra). The short message
       is already on stderr via fmt.Fprintln, so silent is correct.

# Test patterns to MODEL the two new tests on (internal/cmd/default_action_test.go)
- file: internal/cmd/default_action_test.go
  lines: 268-308 (TestRunDefault_DryRun — the rootCmd.SetArgs({...,--dry-run}) + saveRootState/restoreRootState
         + Execute(ctx) pattern); 504-541 (TestRunDefault_Rescue — setupStubRepoRaw + [generation]
         max_duplicate_retries=0 + STAGEHAND_STUB_OUT="" → exit 3); 563-595 (TestRunDefault_Timeout —
         setupStubRepoWithTimeout(t, out, sleepMs, timeout) → exit 124)
  why: Copy the save/restore bracket + buffer setup verbatim. For the dry-run TIMEOUT test reuse
       setupStubRepoWithTimeout(t, "feat: slow", 2000, 150*time.Millisecond). For the dry-run RESCUE test
       reuse setupStubRepoRaw + max_duplicate_retries=0 + STAGEHAND_STUB_OUT="".
  pattern: assert exitcode.For(err) == exitcode.Error (1); assert stderr CONTAINS the short message;
           assert stderr does NOT contain "git commit-tree" (no recipe); assert HEAD unchanged.

# The library regression guard — DO NOT MODIFY
- file: pkg/stagehand/stagehand_test.go
  lines: ~296-362 (TestGenerateCommit_Timeout "dryrun" subtest)
  why: Asserts the LIBRARY returns *RescueError{Kind:ErrTimeout} and exitcode.For(err)==Timeout(124).
       Still holds: the library is unchanged; only the CLI wraps to exit 1. Leave this test alone.

# Docs to update (Mode A)
- file: docs/cli.md
  lines: ~26 (--dry-run Global-flags table row); ~76 (Exit codes table: codes 1/3/124 rows)
  why: Note that a --dry-run GENERATION failure exits 1 with a short message (not 3/124 + recipe). Keep
       exit 3 (rescue) / 124 (timeout) documented for the non-dry-run (commit) paths.
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/
  root.go                 # var flagDryRun (40); PersistentPreRunE; Execute (106)
  default_action.go       # handleGenError (168) — THE edit site
  default_action_test.go  # TestRunDefault_DryRun (268), _Rescue (504), _Timeout (563) — patterns; +2 new tests
  root_test.go            # saveRootState/restoreRootState/resetFlags (104-137) — flag isolation
internal/generate/
  generate.go             # ErrTimeout (54), ErrRescue (59), RescueError (76) — read, NOT modified
internal/exitcode/
  exitcode.go             # Error=1, Rescue=3, Timeout=124, New(code,err) — read, NOT modified
pkg/stagehand/
  stagehand.go            # runPipeline returns *RescueError on dry-run failure — NOT modified
  stagehand_test.go       # TestGenerateCommit_Timeout "dryrun" — NOT modified (library contract)
docs/
  cli.md                  # Mode A doc edits (Global flags + Exit codes)
```

### Desired Codebase tree (files MODIFIED — no new files)

```bash
internal/cmd/default_action.go       # +~7-line branch at top of handleGenError
internal/cmd/default_action_test.go  # +2 new tests (dry-run timeout; dry-run rescue)
docs/cli.md                          # Mode A: --dry-run row + Exit codes note
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL — insert the branch at the TOP of handleGenError, BEFORE the existing
// `var re *generate.RescueError` / `if errors.As(err, &re)` block. If placed AFTER it, the dry-run
// failure would already have been handled by the rescue branch (printing the recipe + exit 3/124).

// CRITICAL — gate on BOTH flagDryRun AND errors.As(err, &re). Dry-run CAS / nothing-to-commit / generic
// errors must fall through to their existing branches (they already exit 1/2 correctly). Only
// *generate.RescueError (timeout OR rescue) is intercepted.

// CRITICAL — return exitcode.New(exitcode.Error, nil) — the `nil` err makes it SILENT (main prints
// nothing extra), because the short message is ALREADY on stderr via fmt.Fprintln. Passing the err
// instead would make main double-print "stagehand: <...>".

// CRITICAL — DO NOT modify pkg/stagehand or its TestGenerateCommit_Timeout "dryrun" subtest. The
// library intentionally returns *RescueError (FR49 full pipeline). Only the CLI translates it.

// GOTCHA — RescueError.Unwrap() returns e.Kind (ErrTimeout|ErrRescue), so errors.Is(err, generate.ErrTimeout)
// works without special-casing Kind directly. Reuse the existing discrimination exactly as the
// non-dry-run branch does.
```

## Implementation Blueprint

### The single code edit (internal/cmd/default_action.go :: handleGenError)

At the very top of `handleGenError`'s body, before `var re *generate.RescueError`, insert:

```go
	// Dry-run generation failure (PRD §9.12 FR49 + bugfix-002 Issue 4): --dry-run runs the full
	// pipeline (incl. the snapshot), so a timeout or parse/dedupe-exhaustion surfaces a
	// *generate.RescueError from the library. For a "preview" that was never going to commit, the
	// §18.3 manual commit-tree recovery recipe is misleading — so print a short stderr line, map to
	// exit 1 (exitcode.Error), and omit the recipe. (flagDryRun is a package var, root.go:40.)
	if flagDryRun {
		var re *generate.RescueError
		if errors.As(err, &re) { // dry-run timeout OR rescue (both are *RescueError)
			msg := "could not generate a commit message; run without --dry-run to see retries and the recovery recipe"
			if errors.Is(err, generate.ErrTimeout) {
				msg = "generation timed out; run without --dry-run to see the recovery recipe"
			}
			fmt.Fprintln(stderr, msg)
			return exitcode.New(exitcode.Error, nil) // exit 1, silent (already printed)
		}
	}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/cmd/default_action.go :: handleGenError (the branch)
  - INSERT: the ~16-line block (above) as the FIRST statements of handleGenError's body, immediately
            before the existing `var re *generate.RescueError` (line ~171).
  - EXACT semantics: gate on `flagDryRun`; inside, `errors.As(err, &re *generate.RescueError)`;
            default msg = rescue wording; `if errors.Is(err, generate.ErrTimeout)` overrides msg;
            `fmt.Fprintln(stderr, msg)`; `return exitcode.New(exitcode.Error, nil)`.
  - NO new imports: errors, fmt, io, generate, exitcode are all already imported by default_action.go.
  - DO NOT: change handleGenError's signature (stderr io.Writer, err error) error; touch the existing
            rescue/CAS/nothing/generic branches; modify pkg/stagehand or any library code.
  - VERIFY after: `go build ./...` && `go vet ./...` (types already line up — flagDryRun is a package
            var; generate.ErrTimeout/generate.RescueError are exported; exitcode.Error/New exist).

Task 2: ADD two CLI tests in internal/cmd/default_action_test.go
  - TEST (a) TestRunDefault_DryRun_Timeout_Exit1:
      * Bracket: `origArgs, origOut, origErr, origRunE := saveRootState(t); defer restoreRootState(...)`.
      * `repo := setupStubRepoWithTimeout(t, "feat: slow", 2000, 150*time.Millisecond)` (sleep past timeout).
      * `writeFile(t, repo, "z.txt", "data"); stageFile(t, repo, "z.txt")`.
      * `beforeHEAD := headSHA(t, repo)`.
      * buffers + `rootCmd.SetArgs([]string{"--provider", "stub", "--dry-run"})`.
      * `err := Execute(context.Background())` — expect non-nil.
      * assert `exitcode.For(err) == exitcode.Error` (1).
      * assert stderr CONTAINS "generation timed out; run without --dry-run".
      * assert stderr does NOT contain "git commit-tree" (no recipe) and does NOT contain "Tree ID:".
      * assert HEAD unchanged.
  - TEST (b) TestRunDefault_DryRun_Rescue_Exit1:
      * Bracket as above.
      * `bin := stubtest.Build(t)`; `repo := setupStubRepoRaw(t, fmt.Sprintf([provider.stub] ... [generation]
        max_duplicate_retries = 0, bin))` (mirror TestRunDefault_Rescue's toml exactly).
      * commit the config, add+stage z.txt. `t.Setenv("STAGEHAND_STUB_OUT", "")` (blank → unparseable).
      * SetArgs `{"--provider","stub","--dry-run"}`.
      * assert `exitcode.For(err) == exitcode.Error` (1) (NOT Rescue=3).
      * assert stderr CONTAINS "could not generate a commit message; run without --dry-run".
      * assert stderr does NOT contain "git commit-tree" / "Tree ID:" / "❌ Commit generation failed.".
      * assert HEAD unchanged.
  - FOLLOW pattern: TestRunDefault_DryRun (268) for the SetArgs/save-restore/Execute structure;
            TestRunDefault_Timeout (563) for setupStubRepoWithTimeout; TestRunDefault_Rescue (504)
            for the rescue toml + blank stub.
  - DO NOT: modify TestRunDefault_Rescue (504), TestRunDefault_Timeout (563), or pkg/stagehand
            TestGenerateCommit_Timeout "dryrun" — they are regression guards.

Task 3: MODIFY docs/cli.md (Mode A)
  - Global flags table, `--dry-run` row (line ~26): append to the Description that a dry-run
    GENERATION failure (timeout, or parse/duplicate-check exhaustion) exits 1 with a short message
    rather than 3/124 + a recovery recipe (since no commit was ever intended).
  - Exit codes section (line ~76): add a short prose note after the table that `--dry-run` generation
    failures report exit 1 with a short stderr message (not 3/124 + recipe); codes 3 (rescue) and 124
    (timeout) remain the non-dry-run (commit-path) semantics. Do not remove the 3/124 rows.
  - DO NOT: touch other rows or other docs (README.md / how-it-works.md are Mode B, owned by P1.M5.T1).
```

### Implementation Patterns & Key Details

```go
// PATTERN — the dry-run branch (top of handleGenError):
//   flagDryRun is a package var (internal/cmd/root.go:40), readable with no import/param change.
//   errors.As(err, &*generate.RescueError) matches BOTH ErrTimeout and ErrRescue (Unwrap→Kind).
//   exitcode.New(exitcode.Error, nil) is SILENT — main prints nothing; the short msg is already on stderr.
if flagDryRun {
	var re *generate.RescueError
	if errors.As(err, &re) {
		msg := "could not generate a commit message; run without --dry-run to see retries and the recovery recipe"
		if errors.Is(err, generate.ErrTimeout) {
			msg = "generation timed out; run without --dry-run to see the recovery recipe"
		}
		fmt.Fprintln(stderr, msg)
		return exitcode.New(exitcode.Error, nil) // exit 1, silent
	}
}
// ... existing non-dry-run rescue/CAS/nothing/generic branches follow unchanged ...
```

### Integration Points

```yaml
CODE:
  - file: internal/cmd/default_action.go
    function: handleGenError
    change: "+~16-line block (comment + 7 LOC logic) at the top of the body"
    risk: ADDITIVE only on the dry-run path. Non-dry-run rescue/timeout/CAS/nothing/generic behavior is
          byte-for-byte unchanged. The library (pkg/stagehand) is untouched.

TESTS:
  - file: internal/cmd/default_action_test.go
    change: "+2 tests (TestRunDefault_DryRun_Timeout_Exit1, TestRunDefault_DryRun_Rescue_Exit1)"
    guards: TestRunDefault_Rescue (504), TestRunDefault_Timeout (563), pkg/stagehand
            TestGenerateCommit_Timeout "dryrun" (~296-362) stay GREEN UNCHANGED.

NO DATABASE / NO NEW CONFIG KEYS / NO NEW ROUTES / NO NEW DEPENDENCIES / NO LIBRARY CHANGES.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After Task 1 (the code branch) — fix before proceeding to tests/docs.
go build ./...            # compiles (no new imports; flagDryRun/ErrTimeout/exitcode.Error all exist)
go vet ./...              # vet clean
make lint                 # golangci-lint — zero findings
# Expected: zero errors. The branch compiles on the first pass (all symbols already imported).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The two new tests (after Task 2):
go test -race ./internal/cmd/ -run 'TestRunDefault_DryRun_(Timeout|Rescue)_Exit1' -v
# Expected: both PASS (exit 1 + short msg + no recipe + HEAD unchanged).

# Targeted regression gate (the binding contract gate):
go test -race ./internal/cmd/... ./pkg/stagehand/...
# Expected: ALL green, INCLUDING unchanged:
#   - internal/cmd TestRunDefault_Rescue (exit 3) and TestRunDefault_Timeout (exit 124)
#   - pkg/stagehand TestGenerateCommit_Timeout "dryrun" (library returns *RescueError → exitcode.For==124)

# Full suite (Makefile `test`):
go test -race ./...
# Expected: all packages pass. If a regression FAILS, READ the output — most likely cause is placing
# the branch AFTER the existing rescue block (dry-run would then hit the recipe/exit-3-or-124 path).
```

### Level 3: Integration / End-to-End Smoke (manual proof)

```bash
go build -o bin/stagehand ./cmd/stagehand
go build -o bin/stubagent ./cmd/stubagent   # if a build target exists; else `go build ./cmd/stubagent`

tmp=$(mktemp -d) && cd "$tmp"
git init -q && git config user.email t@t && git config user.name t
# Stub provider config (point command at the built stubagent; mirror providers/ stub manifests):
cat > .stagehand.toml <<'EOF'
[provider.stub]
command = "<abs path to bin/stubagent>"
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
[defaults]
timeout = "1s"
EOF
echo a > a.txt && git add a.txt && git commit -qm init

# CASE A — dry-run timeout (stub sleeps past 1s):
echo b > b.txt && git add b.txt
STAGEHAND_STUB_SLEEP_MS=5000 ./bin/stagehand --provider stub --dry-run
echo "exit=$?"
# Expected: exit=1, stderr "...generation timed out; run without --dry-run...", NO "git commit-tree" line.

# CASE B — dry-run rescue (blank/unparseable stub):
STAGEHAND_STUB_OUT="" STAGEHAND_STUB_SLEEP_MS=0 ./bin/stagehand --provider stub --dry-run
echo "exit=$?"
# Expected: exit=1, stderr "...could not generate a commit message; run without --dry-run...", NO recipe.

# CASE C — regression: NON-dry-run timeout still exits 124 + recipe:
STAGEHAND_STUB_SLEEP_MS=5000 ./bin/stagehand --provider stub
echo "exit=$?"   # Expected: 124 + stderr contains "git commit-tree" recovery recipe.

cd - && rm -rf "$tmp"
```

### Level 4: Doc Validation

```bash
# The --dry-run row + Exit codes section read correctly:
go build -o /tmp/stagehand ./cmd/stagehand
/tmp/stagehand --help | grep -A1 "dry-run"   # confirm the flag help is sensible (help text unchanged)
# Markdown lint (repo has .markdownlint.json) — advisory:
npx --yes markdownlint-cli docs/cli.md 2>/dev/null || true
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean.
- [ ] `go vet ./...` clean.
- [ ] `go test -race ./internal/cmd/... ./pkg/stagehand/...` green (incl. the two new tests + the 3 regression guards).
- [ ] `go test -race ./...` green.
- [ ] `make lint` — zero findings.

### Feature Validation
- [ ] New branch at the TOP of `handleGenError`, guarded on `flagDryRun` + `errors.As(err, &*generate.RescueError)`.
- [ ] Default msg = rescue wording; `errors.Is(err, generate.ErrTimeout)` → timeout wording.
- [ ] `fmt.Fprintln(stderr, msg)` + `return exitcode.New(exitcode.Error, nil)` (exit 1, silent).
- [ ] Level 3 smoke: dry-run timeout → exit 1 + short msg + no recipe; dry-run rescue → exit 1 + short msg + no recipe; non-dry-run timeout → exit 124 + recipe (unchanged).
- [ ] `docs/cli.md` `--dry-run` row + Exit codes note updated.

### Code Quality Validation
- [ ] Follows existing `handleGenError` silent-error pattern (`exitcode.New(code, nil)` after `fmt.Fprintln`).
- [ ] Additive on the dry-run path only; non-dry-run branches byte-identical.
- [ ] No new imports, no new files, no signature changes, no library changes.

### Documentation & Boundaries
- [ ] Mode A docs (cli.md) shipped here (S1); P1.M5.T1 (Mode B sweep: README.md + how-it-works.md) runs last and reconciles against these.
- [ ] Library `pkg/stagehand` + `TestGenerateCommit_Timeout "dryrun"` explicitly UNCHANGED (the library contract is a regression guard, not a deliverable).

---

## Anti-Patterns to Avoid

- ❌ Don't place the branch AFTER the existing `errors.As(err, &re)` block — the dry-run failure would be consumed by the rescue branch first (recipe + exit 3/124).
- ❌ Don't `return exitcode.New(exitcode.Error, err)` — that makes main double-print `stagehand: <...>`; use `nil` (silent) since the short message is already on stderr.
- ❌ Don't modify `pkg/stagehand` or its `TestGenerateCommit_Timeout` "dryrun" subtest — the library intentionally returns `*RescueError` (FR49 full pipeline); only the CLI translates it to exit 1.
- ❌ Don't modify `TestRunDefault_Rescue` (504) or `TestRunDefault_Timeout` (563) — they are the commit-path regression guards and stay exit 3 / 124.
- ❌ Don't intercept dry-run CAS / nothing-to-commit / generic errors — they already exit 1/2 correctly; only `*generate.RescueError` needs the dry-run special-case.
- ❌ Don't change `handleGenError`'s signature or add a new parameter/import — `flagDryRun` is a same-package var, read directly.
- ❌ Don't remove the 3/124 rows from docs/cli.md — they remain correct for the non-dry-run (commit) paths.

---

## Confidence Score

**9 / 10** — This is a precisely-scoped, ~7-line additive branch at a single identified function
(`handleGenError`), reading an in-scope package var (`flagDryRun`), reusing the exact error
discrimination the file already performs (`errors.As` + `errors.Is(err, generate.ErrTimeout)`), with a
binding issue analysis providing the verbatim patch, exact message wording, regression guards, and
test-fixture pointers (`setupStubRepoWithTimeout`, `setupStubRepoRaw`, the `saveRootState`/`SetArgs`
bracket). The library contract is explicitly preserved (regression guard, not a deliverable). The only
residual uncertainty is the exact stderr wording assertion strings in the new tests, which are quoted
verbatim from the contract, so the risk is minimal.
