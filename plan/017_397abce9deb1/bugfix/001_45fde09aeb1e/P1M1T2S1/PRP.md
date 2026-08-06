name: "P1.M1.T2.S1 — ~/go (default GOPATH) fallback in detectPath + regression test (BUG-002)"
description: >
  Fix BUG-002 (Major): detectPath's go-install heuristic (internal/upgrade/detect.go:371-378) only runs
  when GOPATH is EXPLICITLY set, so a binary at ~/go/bin is misdetected as `direct` when GOPATH is unset
  (the common modern-Go case — `go env GOPATH` defaults to ~/go). Add a HOME-based fallback: when
  `envOr("GOPATH","")` is empty, resolve the default as `filepath.Join(envOr("HOME",""), "go")` and run
  the SAME prefix check. An explicit GOPATH always wins; the HOME default applies only when GOPATH is
  empty; both unset ⇒ no false positive (falls through to direct). Reuses the existing d.envOr/d.Env
  seam (no hard os.Getenv). Evidence string distinguishes the two cases. Plus two regression tests
  (default-GOPATH detection + no-false-positive-when-HOME-unset) and a Mode A doc-comment update. Only
  detect.go + detect_test.go touched; stage.go (BUG-001) and releases.go (BUG-003) are siblings.

---

## Goal

**Feature Goal**: Make `stagecoach upgrade` correctly route a `go install`-installed binary (under the
default `~/go/bin`) to the `go-install` delegation channel (FR-U2/FR-U3) instead of misrouting it to the
self-swap `direct` channel — by implementing the `~/go` default-GOPATH fallback that detectPath's
comment already claims but the code never did (BUG-002).

**Deliverable**: (1) a small edit to the go-install block in `internal/upgrade/detect.go::detectPath`
(lines 371-378): resolve `~/go` via `d.envOr("HOME","")` when GOPATH is empty, then run the existing
prefix check; (2) a Mode A rewrite of the inline comment to state the fallback is implemented; (3) two
new regression tests in `internal/upgrade/detect_test.go` (default-GOPATH detection + no-false-positive
guard), plus the `path/filepath` import addition those tests need.

**Success Definition**:
- `Detector{ExePath: $HOME/go/bin/stagecoach, Env: GOPATH→"", HOME→$HOME}` → `detectPath()` returns
  `(ChannelGoInstall, "path: ~/go/bin (default GOPATH)", true)`. *(the BUG-002 fix)*
- `Detector{ExePath: $GOPATH/bin/stagecoach, Env: GOPATH→$GOPATH}` → still returns
  `(ChannelGoInstall, "path: $GOPATH/bin", true)`. *(existing behavior preserved — explicit GOPATH wins)*
- `Detector{ExePath: <a ~/go/bin-style path>, Env: GOPATH→"", HOME→""}` → `detectPath()` does NOT
  return ChannelGoInstall (no false positive when HOME is unknown; falls through to direct).
- Existing `TestDetect_Path_GoInstallViaGOPATH` still passes (it sets GOPATH explicitly).
- `go build ./...`, `go test ./internal/upgrade/ -count=1`, `make test`, `make lint` all pass.
- No hard `os.Getenv` added — the fix uses the existing `d.envOr`/`d.Env` injection seam.

## User Persona (if applicable)

**Target User**: A developer who installed stagecoach via `go install github.com/.../stagecoach@latest`
(the documented §21.3 method) and runs `stagecoach upgrade`.

**Use Case**: The user's binary lives at `~/go/bin/stagecoach` (the `go install` default). They run
`stagecoach upgrade` expecting it to delegate to `go install ...@latest` (FR-U3). After this fix, it does.

**User Journey**: `go install ...@latest` → binary at `~/go/bin/stagecoach` → `stagecoach upgrade` →
detectPath resolves the default GOPATH (`~/go`) → channel=`go-install` → upgrade delegates to
`go install ...@latest` (no self-swap). Pre-fix this misrouted to `direct` (self-swap), contradicting
FR-U1 and (with BUG-001) failing entirely.

**Pain Points Addressed**: BUG-002 — the most common `go install` install path was misdetected as
`direct`, routing users to a self-swap that (a) contradicts the delegate-first architecture (FR-U1) and
(b) combined with BUG-001 fails the sanity-run, breaking `stagecoach upgrade` for go-install users
entirely.

## Why

- **BUG-002 (Major) / FR-U1 / FR-U2 / FR-U3 / §21.3**: `go install` is a documented install method.
  detectPath's GOPATH check only fired when GOPATH was EXPLICITLY set; the modern-Go default (GOPATH
  unset ⇒ `~/go`) was not implemented despite the code comment claiming it. So the common `go install`
  user was misrouted. This fix implements the fallback, restoring correct channel detection.
- **Root cause** (context, no action): the `if gopath := d.envOr("GOPATH",""); gopath != ""` guard at
  detect.go:373 returns false when GOPATH is unset, skipping the whole heuristic. The comment at :371
  claimed a `~/go` default that the code never resolved.
- **Bounded, surgical**: one block in one function, reusing the existing `envOr` seam + the existing
  prefix-check line (byte-identical). No new pattern, no os.Getenv, no change to the prefix-matching
  logic. Sibling BUG-001 (stage.go) and BUG-003 (releases.go) are different files.

## What

**User-visible behavior**: `stagecoach upgrade` for a `go install`-installed binary (under `~/go/bin`,
GOPATH unset) now delegates to `go install ...@latest` instead of attempting a self-swap.

**Technical change (one block rewrite + comment + 2 tests):**
1. `detect.go` detectPath go-install block (371-378): when `envOr("GOPATH","")` is empty, resolve
   `filepath.Join(envOr("HOME",""), "go")`; if non-empty, use it as the gopath for the existing prefix
   check. Evidence string `"path: ~/go/bin (default GOPATH)"` for the default case; `"path: $GOPATH/bin"`
   for the explicit case. Explicit GOPATH always wins.
2. Mode A: rewrite the inline comment to state the fallback is implemented + the HOME resolution +
   the explicit-wins precedence.
3. `detect_test.go`: +`path/filepath` import; +`TestDetect_Path_GoInstallDefaultGOPATH`;
   +`TestDetect_Path_GoInstallNoFalsePositiveWhenHOMEUnset`.

### Success Criteria
- [ ] GOPATH unset + HOME set + ExePath under `$HOME/go/bin` → `(ChannelGoInstall, …, true)`
- [ ] Explicit GOPATH set + ExePath under `$GOPATH/bin` → `(ChannelGoInstall, "path: $GOPATH/bin", true)` (unchanged)
- [ ] Both GOPATH and HOME unset → NOT ChannelGoInstall (falls through to direct; no false positive)
- [ ] Existing `TestDetect_Path_GoInstallViaGOPATH` still passes
- [ ] No hard `os.Getenv` — uses `d.envOr`/`d.Env`
- [ ] `go build ./...`, `go test ./internal/upgrade/ -count=1`, `make test`, `make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact buggy block (with the current code), the exact replacement block, the envOr seam
contract, the prefix-check line that stays byte-identical, the two tests (verbatim), the `path/filepath`
import gotcha, the existing test that must still pass, and the scope fences against BUG-001/BUG-003 are
all enumerated below.

### Documentation & References

```yaml
- file: internal/upgrade/detect.go
  why: "THE change site. detectPath @349; the buggy go-install block @371-378 (the `if gopath := d.envOr
        (\"GOPATH\",\"\"); gopath != \"\"` guard that skips the heuristic when GOPATH is unset). envOr @395
        (the REUSE seam — nil-Env guard + non-empty check). Detector.Env func(string)string @152 (the
        injectable os.Getenv). ChannelGoInstall @47. pathHeuristics @335-339 (Cellar/Scoop/Nix only —
        no false match on ~/go/bin, confirmed)."
  pattern: >
    // CURRENT (buggy) block @371-378:
    if gopath := d.envOr("GOPATH", ""); gopath != "" {
        goBin := filepath.Clean(filepath.Join(gopath, "bin")) + string(filepath.Separator)
        if strings.HasPrefix(real, goBin) || strings.HasPrefix(lower, strings.ToLower(goBin)) {
            return ChannelGoInstall, "path: $GOPATH/bin", true
        }
    }
  critical: "Keep the prefix-check line (goBin := ...; strings.HasPrefix(real, goBin) || strings.HasPrefix
             (lower, strings.ToLower(goBin))) BYTE-IDENTICAL — only the gopath SOURCE and the evidence
             string change. Use d.envOr(\"HOME\", \"\") (the existing seam), NOT os.Getenv. filepath and
             strings are already imported in detect.go — no import change needed in detect.go."

- file: internal/upgrade/detect_test.go
  why: "THE test file to extend. TestDetect_Path_GoInstallViaGOPATH @353 is the EXACT pattern to mirror
        (Detector{ExePath, GOOS, Env: func(k){...}}; call d.detectPath(); assert ch==ChannelGoInstall).
        Imports @3-9 are context/errors/strings/testing/time — NO path/filepath (the new test uses
        filepath.Join ⇒ ADD the import)."
  pattern: "d := &Detector{ExePath: ..., GOOS: \"linux\", Env: func(k string) string {...}}; ch, _, ok :=
            d.detectPath(); if !ok || ch != ChannelGoInstall { t.Errorf(...) }"
  critical: "ADD `\"path/filepath\"` to the import block — the new TestDetect_Path_GoInstallDefaultGOPATH
             uses filepath.Join(home, \"go\", \"bin\", \"stagecoach\"). Forgetting the import = compile
             error. (Alternative: use a literal path string and skip the import; filepath.Join is clearer
             and matches the item contract.) The existing GoInstallViaGOPATH test DISCARDS the evidence
             string (ch, _, ok) ⇒ your evidence-string change does NOT break it."

- docfile: plan/017_397abce9deb1/bugfix/001_45fde09aeb1e/architecture/bug_analysis.md
  why: "§BUG-002 is the authoritative analysis: location, root cause, fix strategy (resolve ~/go via HOME
        when GOPATH empty; explicit GOPATH wins; both-unset ⇒ no false positive), and the regression-test
        shape. The item's contract tracks this exactly."
  section: "BUG-002 (MAJOR) — go-install misdetected as direct when GOPATH unset"

- docfile: plan/017_397abce9deb1/bugfix/001_45fde09aeb1e/P1M1T1S1/PRP.md
  why: "The parallel sibling (BUG-001). It edits internal/upgrade/stage.go (sanityCheck) — a DIFFERENT
        file. Read it to confirm the non-overlap (no merge conflict with this detect.go task)."
```

### Current Codebase tree (relevant slice)

```bash
internal/upgrade/
  detect.go           # THE fix: detectPath go-install block @371-378 (resolve ~/go default via HOME when GOPATH empty) + inline comment
  detect_test.go      # ADD: path/filepath import + TestDetect_Path_GoInstallDefaultGOPATH + TestDetect_Path_GoInstallNoFalsePositiveWhenHOMEUnset
  stage.go            # BUG-001 sibling (P1.M1.T1.S1) — NOT touched here
  releases.go         # BUG-003 sibling (P1.M1.T3.S1) — NOT touched here
```

### Desired Codebase tree with files to be added

```bash
internal/upgrade/detect.go           # MODIFY: go-install block @371-378 (~/go fallback) + inline comment (Mode A)
internal/upgrade/detect_test.go      # MODIFY: +path/filepath import, +2 test funcs
# (no new files; no CLI/config/API/docs change)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (reuse envOr, NOT os.Getenv): the fix MUST use d.envOr("HOME", "") (detect.go:395), which
//   honors the injectable d.Env seam (Detector.Env @152). A hard os.Getenv("HOME") would bypass test
//   injection → the test couldn't control HOME → non-deterministic (reads the real environment). envOr
//   already does the nil-Env guard + non-empty check.

// CRITICAL (explicit GOPATH always wins): compute gopath = envOr("GOPATH","") FIRST; only when it is
//   "" do you resolve the HOME default. If you resolved HOME first or unconditionally, an explicit
//   GOPATH could be ignored — breaking the existing TestDetect_Path_GoInstallViaGOPATH and the
//   documented precedence. The `if gopath == ""` gate is load-bearing.

// CRITICAL (no false positive when both unset): if BOTH GOPATH and HOME are empty (envOr returns ""
//   for both), gopath MUST stay "" so the `if gopath != ""` block is skipped entirely. Do NOT fall
//   back to a hardcoded default home path — HOME unknown means we cannot resolve the default, so we
//   correctly decline (fall through to direct). This is the BUG-002 no-false-positive guard.

// CRITICAL (test import): detect_test.go does NOT currently import path/filepath (imports are context/
//   errors/strings/testing/time). The new TestDetect_Path_GoInstallDefaultGOPATH uses filepath.Join →
//   ADD "path/filepath" to the import block, or the test won't compile. (Or use a literal path string.)

// GOTCHA (keep the prefix-check line identical): the line `goBin := filepath.Clean(filepath.Join(gopath,
//   "bin")) + string(filepath.Separator); if strings.HasPrefix(real, goBin) || strings.HasPrefix(lower,
//   strings.ToLower(goBin))` is correct and must stay byte-identical — only the gopath SOURCE changes
//   (explicit vs ~/go default), plus the evidence string. The trailing-separator + lowercased-HasPrefix
//   handles case-insensitive FS / cross-GOOS edge cases; don't "simplify" it.

// GOTCHA (evidence string is not asserted by the existing test): TestDetect_Path_GoInstallViaGOPATH
//   does `ch, _, ok := d.detectPath()` — it DISCARDS the evidence. So changing the evidence from a
//   constant to a variable ("path: $GOPATH/bin" vs "path: ~/go/bin (default GOPATH)") does NOT break
//   it. (You MAY add an evidence assertion to the new default-GOPATH test if you want, but it's optional.)

// SCOPE: do NOT touch stage.go (BUG-001 = P1.M1.T1.S1, parallel) or releases.go (BUG-003 = P1.M1.T3.S1).
//   do NOT add os.Getenv. do NOT change pathHeuristics or the npm node_modules heuristic. do NOT change
//   CLI/config/API surface or user-facing docs (Mode A = the inline source comment only).
```

## Implementation Blueprint

### Data models and structure
None. No struct/type change. One control-flow edit in `detectPath` (a pure function returning
`(Channel, string, bool)`) reusing the existing `envOr` seam and prefix-check line.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/upgrade/detect.go — implement the ~/go default-GOPATH fallback in detectPath
  - LOCATE the go-install block in detectPath (search "go-install via $GOPATH/bin" — currently @371).
  - REPLACE the block (the 3-line `if gopath := d.envOr("GOPATH", ""); gopath != "" { ... }` plus its
    2-line comment) with:
        // go-install via $GOPATH/bin. When GOPATH is explicitly set, use it; when GOPATH is UNSET (the
        // overwhelmingly common modern-Go case — `go env GOPATH` defaults to ~/go), fall back to $HOME/go
        // so a binary installed via `go install ...@latest` under the default ~/go/bin is detected as
        // go-install (FR-U2/FR-U3) instead of misrouted to direct (BUG-002). Resolved from the injected
        // env getter so a test can set GOPATH/HOME without touching the real environment. An explicit
        // GOPATH always wins; the HOME-based default applies only when GOPATH is empty.
        gopath := d.envOr("GOPATH", "")
        evidence := "path: $GOPATH/bin"
        if gopath == "" {
            if home := d.envOr("HOME", ""); home != "" {
                gopath = filepath.Join(home, "go")
                evidence = "path: ~/go/bin (default GOPATH)"
            }
        }
        if gopath != "" {
            goBin := filepath.Clean(filepath.Join(gopath, "bin")) + string(filepath.Separator)
            if strings.HasPrefix(real, goBin) || strings.HasPrefix(lower, strings.ToLower(goBin)) {
                return ChannelGoInstall, evidence, true
            }
        }
  - VERIFY the prefix-check line is byte-identical to the original (only gopath source + evidence differ).
  - VERIFY no new import is needed in detect.go (filepath + strings already imported).
  - DEPENDENCIES: none.

Task 2: MODIFY internal/upgrade/detect_test.go — add the path/filepath import (if using filepath.Join)
  - ADD `"path/filepath"` to the import block (currently context/errors/strings/testing/time).
  - (If you instead use literal path strings in the new tests, skip this — but filepath.Join is clearer.)
  - DEPENDENCIES: none (enables Task 3).

Task 3: MODIFY internal/upgrade/detect_test.go — add the two regression tests
  - ADD (mirror TestDetect_Path_GoInstallViaGOPATH @353):
        // TestDetect_Path_GoInstallDefaultGOPATH is the BUG-002 regression: GOPATH unset (the common
        // modern-Go case) but HOME set → a binary under $HOME/go/bin must be detected as go-install via
        // the ~/go default (matches `go env GOPATH`).
        func TestDetect_Path_GoInstallDefaultGOPATH(t *testing.T) {
            home := "/tmp/fakehome"
            d := &Detector{
                ExePath: filepath.Join(home, "go", "bin", "stagecoach"),
                GOOS:    "linux",
                Env: func(k string) string {
                    if k == "HOME" {
                        return home
                    }
                    return "" // GOPATH unset
                },
            }
            ch, _, ok := d.detectPath()
            if !ok || ch != ChannelGoInstall {
                t.Errorf("detectPath default-GOPATH = %q,%v, want go-install,true", ch, ok)
            }
        }

        // TestDetect_Path_GoInstallNoFalsePositiveWhenHOMEUnset is the BUG-002 no-false-positive guard:
        // both GOPATH and HOME unset → even a ~/go/bin-style ExePath must NOT be detected as go-install
        // (no HOME ⇒ no default GOPATH to resolve ⇒ falls through to direct).
        func TestDetect_Path_GoInstallNoFalsePositiveWhenHOMEUnset(t *testing.T) {
            d := &Detector{
                ExePath: "/home/me/go/bin/stagecoach", // looks like a go-install path
                GOOS:    "linux",
                Env:     func(k string) string { return "" }, // GOPATH and HOME both unset
            }
            ch, _, ok := d.detectPath()
            if ok && ch == ChannelGoInstall {
                t.Errorf("detectPath false-positive go-install = %q,%v, want no go-install match (HOME unknown)", ch, ok)
            }
        }
  - PLACE near TestDetect_Path_GoInstallViaGOPATH (keep the go-install tests together).
  - DEPENDENCIES: Tasks 1-2.

Task 4: VERIFY build + vet + format + targeted tests + full upgrade suite
  - go build ./...
  - go vet ./internal/upgrade/...
  - gofmt -l internal/upgrade/detect.go internal/upgrade/detect_test.go   # must list nothing
  - go test ./internal/upgrade/ -run 'TestDetect_Path_GoInstall' -v        # new + existing go-install tests
  - go test ./internal/upgrade/ -count=1                                  # the item's exact command
  - make test && make lint
  - REGRESSION-CHECK (by reasoning): pre-fix, TestDetect_Path_GoInstallDefaultGOPATH would FAIL
    (detectPath returned ("", "", false) because the GOPATH guard was skipped); post-fix it PASSES.
    This is the test's reason to exist.
```

### Implementation Patterns & Key Details

```go
// PATTERN: the fixed go-install block (resolve ~/go default when GOPATH empty; reuse the prefix check)
gopath := d.envOr("GOPATH", "")            // explicit GOPATH (existing seam — test-injectable)
evidence := "path: $GOPATH/bin"
if gopath == "" {                          // GOPATH unset → resolve the `go env GOPATH` default
    if home := d.envOr("HOME", ""); home != "" {
        gopath = filepath.Join(home, "go")
        evidence = "path: ~/go/bin (default GOPATH)"
    }
}
if gopath != "" {                          // both-unset ⇒ gopath=="" ⇒ skipped (no false positive)
    goBin := filepath.Clean(filepath.Join(gopath, "bin")) + string(filepath.Separator)
    if strings.HasPrefix(real, goBin) || strings.HasPrefix(lower, strings.ToLower(goBin)) {
        return ChannelGoInstall, evidence, true
    }
}

// PATTERN: the regression test (mirror TestDetect_Path_GoInstallViaGOPATH; inject Env)
d := &Detector{
    ExePath: filepath.Join(home, "go", "bin", "stagecoach"),
    GOOS:    "linux",
    Env:     func(k string) string { if k == "HOME" { return home }; return "" },
}
ch, _, ok := d.detectPath()
if !ok || ch != ChannelGoInstall { t.Errorf(...) }
```

### Integration Points

```yaml
NO CLI / config / API / struct / docs-surface changes. One control-flow block + comment + 2 tests.

CODE:
  - internal/upgrade/detect.go detectPath go-install block (371-378) — ~/go fallback + evidence + comment (Mode A)
TEST:
  - internal/upgrade/detect_test.go — +path/filepath import, +2 test funcs

CONSUMED (read-only, reused):
  - d.envOr (detect.go:395) — the nil-Env-guarded env seam (NOT os.Getenv)
  - the existing prefix-check line (HasPrefix on goBin w/ trailing separator + lowercased variant)

DOWNSTREAM (the fix ENABLES correct routing):
  - detectPath → Detect → ResolveTarget: a ~/go/bin binary now resolves to ChannelGoInstall, so
    `stagecoach upgrade` delegates to `go install ...@latest` (FR-U3) instead of self-swapping (FR-U1).

UNCHANGED (do NOT touch): stage.go (BUG-001); releases.go (BUG-003); pathHeuristics; the npm
  node_modules heuristic; Detector fields; CLI/config/API; user-facing docs.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build (detect.go change compiles; no new detect.go imports needed — filepath/strings already present)
go build ./...
# Vet the package
go vet ./internal/upgrade/...
# Format check
gofmt -l internal/upgrade/detect.go internal/upgrade/detect_test.go
# Expected: nothing listed. If listed: gofmt -w them.
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The go-install tests: the 2 new + the existing GoInstallViaGOPATH (must still pass)
go test ./internal/upgrade/ -run 'TestDetect_Path_GoInstall' -v
# Expected: TestDetect_Path_GoInstallDefaultGOPATH PASS (the BUG-002 fix);
#           TestDetect_Path_GoInstallNoFalsePositiveWhenHOMEUnset PASS (no false positive);
#           TestDetect_Path_GoInstallViaGOPATH PASS (explicit GOPATH, unchanged).

# The item's EXACT command — the whole upgrade package
go test ./internal/upgrade/ -count=1
# Expected: ALL pass.

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# The within-scope proof is the unit test (detectPath is a pure function; the channel-routing effect is
# observed via ResolveTarget, which the unit-level channel assertion already covers). Optional manual
# confirmation of the routing at the Detect level:
go build -o /tmp/sc ./cmd/stagecoach
# (Detect reads the real os.Executable + os.Getenv via production wiring; simulating an unset-GOPATH
#  ~/go/bin install in a real shell is environment-dependent, so the unit test is the authoritative gate.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: the ~/go fallback is present in detect.go
grep -n 'filepath.Join(home, "go")\|default GOPATH' internal/upgrade/detect.go
# Expected: the fallback line + the evidence string.

# Grep guard: no hard os.Getenv was added (the fix uses the seam)
grep -n 'os.Getenv' internal/upgrade/detect.go
# Expected: empty (or only pre-existing unrelated uses — the BUG-002 fix must not add one).

# Grep guard: the two new tests exist
grep -n 'TestDetect_Path_GoInstallDefaultGOPATH\|TestDetect_Path_GoInstallNoFalsePositiveWhenHOMEUnset' internal/upgrade/detect_test.go
# Expected: both func names present.

# Grep guard: path/filepath import was added (if using filepath.Join)
grep -n '"path/filepath"' internal/upgrade/detect_test.go
# Expected: one hit (the import). If absent, the test must use literal paths instead.

# Scope-boundary guard: ONLY detect.go + detect_test.go changed by this subtask
git diff --stat -- internal/upgrade/stage.go internal/upgrade/releases.go
# Expected: empty (BUG-001/BUG-003 are siblings — different files).

# Regression-property check (by reasoning): confirm TestDetect_Path_GoInstallDefaultGOPATH would FAIL
# on the pre-fix code (the GOPATH guard was skipped → detectPath returned ("","",false) → ok==false).
# Post-fix it PASSES. This is the test's reason to exist. (Optional empirical check: temporarily revert
# the detect.go block, re-run, observe the FAIL, then restore.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/upgrade/...` clean
- [ ] `gofmt -l internal/upgrade/detect.go internal/upgrade/detect_test.go` empty
- [ ] `make lint` zero errors
- [ ] `go test ./internal/upgrade/ -count=1` passes (item's exact command); `make test` passes

### Feature Validation
- [ ] GOPATH unset + HOME set + ExePath under `$HOME/go/bin` → `(ChannelGoInstall, …, true)` (BUG-002 fix)
- [ ] Explicit GOPATH + ExePath under `$GOPATH/bin` → `(ChannelGoInstall, "path: $GOPATH/bin", true)` (unchanged)
- [ ] Both GOPATH and HOME unset → NOT ChannelGoInstall (no false positive; falls through to direct)
- [ ] Existing `TestDetect_Path_GoInstallViaGOPATH` still passes
- [ ] Evidence string distinguishes default (`~/go/bin (default GOPATH)`) vs explicit (`$GOPATH/bin`)

### Scope-Boundary Validation
- [ ] NO change to stage.go (BUG-001) or releases.go (BUG-003)
- [ ] NO hard `os.Getenv` added — uses `d.envOr`/`d.Env`
- [ ] NO change to pathHeuristics / npm heuristic / Detector fields
- [ ] NO CLI/config/API/user-facing-docs change (Mode A = inline source comment only)
- [ ] Only detect.go + detect_test.go touched

### Code Quality
- [ ] The prefix-check line is byte-identical to the original (only gopath source + evidence change)
- [ ] Explicit-GOPATH-wins precedence preserved (the `if gopath == ""` gate)
- [ ] Mode A comment states the fallback is implemented + the HOME resolution + explicit-wins
- [ ] Tests mirror the existing TestDetect_Path_GoInstallViaGOPATH style; the no-false-positive test is included

---

## Anti-Patterns to Avoid

- ❌ Don't add a hard `os.Getenv("HOME")` — it bypasses the `d.Env` injection seam, making the test unable to control HOME (non-deterministic, reads the real env). Use `d.envOr("HOME", "")` (detect.go:395), which already does the nil-Env guard + non-empty check.
- ❌ Don't resolve the HOME default before/unconditionally with GOPATH — compute `gopath = envOr("GOPATH","")` FIRST and only resolve `~/go` when it is `""`. An explicit GOPATH must always win (the `if gopath == ""` gate is load-bearing; without it you'd break the existing TestDetect_Path_GoInstallViaGOPATH and the documented precedence).
- ❌ Don't introduce a hardcoded fallback home path when HOME is also unset — if both GOPATH and HOME are empty, `gopath` must stay `""` so the block is skipped (fall through to direct). A guessed default would create false positives. "HOME unknown ⇒ decline" is the correct, safe behavior.
- ❌ Don't alter the prefix-check line (`goBin := filepath.Clean(filepath.Join(gopath, "bin")) + sep; strings.HasPrefix(real, goBin) || strings.HasPrefix(lower, strings.ToLower(goBin))`) — it is correct and handles case-insensitive/cross-GOOS edge cases. Only the gopath SOURCE and the evidence string change.
- ❌ Don't forget the `path/filepath` import in detect_test.go if your new test uses `filepath.Join` — the file currently imports only context/errors/strings/testing/time. A missing import is a compile error. (Or use a literal path string.)
- ❌ Don't assert on the evidence string in the EXISTING TestDetect_Path_GoInstallViaGOPATH — it discards the evidence (`ch, _, ok`); your evidence change is safe there. (You may assert evidence in the NEW default-GOPATH test, but it's optional.)
- ❌ Don't touch stage.go (BUG-001 = P1.M1.T1.S1, parallel) or releases.go (BUG-003 = P1.M1.T3.S1) — different files, different bugs.
- ❌ Don't change the inline comment's CLAIM without implementing it — the Mode A update is to state the fallback is now IMPLEMENTED (the old comment claimed it but the code didn't do it; after your fix the claim is true, so the comment should reflect the actual HOME-based mechanism + the explicit-wins precedence).
- ❌ Don't write a test that passes on the pre-fix code — TestDetect_Path_GoInstallDefaultGOPATH MUST fail without the fix (the GOPATH guard was skipped). Verify by reasoning (or a temporary revert) that it's a real regression guard.

---

## Confidence Score: 10/10

This is a small, surgical control-flow edit in one pure function, reusing the existing `envOr` seam and
a byte-identical prefix-check line, with the replacement block specified verbatim, two verbatim tests
mirroring an existing one, and a single called-out import gotcha (`path/filepath`). The fix strategy is
prescribed by the architecture analysis (BUG-002), the no-false-positive property is explicit (both-unset
⇒ skip), the existing test is confirmed to still pass (it discards the evidence string), and the sibling
bugs are in different files. The only conceivable failure modes — using os.Getenv, resolving HOME before
GOPATH, hardcoding a fallback when HOME is unset, or forgetting the test import — are each explicitly
guarded by a CRITICAL gotcha + a Level-4 grep check.