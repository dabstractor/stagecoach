# Research: P1.M1.T2.S1 — ~/go (default GOPATH) fallback in detectPath (BUG-002)

**Scope**: Fix the go-install misdetection (BUG-002) in internal/upgrade/detect.go detectPath + add
two regression tests in internal/upgrade/detect_test.go. All line numbers verified against the working
tree (2026-08-06). Sibling BUG-001 (P1.M1.T1.S1) is in stage.go — a DIFFERENT file; no conflict.

## 1. The bug (BUG-002) — exact location + root cause

**File:** internal/upgrade/detect.go, detectPath, lines 371-378 (the go-install block):
```go
// go-install via $GOPATH/bin (default ~/go/bin when GOPATH is unset). Resolved from the injected
// env getter so a test can set GOPATH without touching the real environment.
if gopath := d.envOr("GOPATH", ""); gopath != "" {
    goBin := filepath.Clean(filepath.Join(gopath, "bin")) + string(filepath.Separator)
    if strings.HasPrefix(real, goBin) || strings.HasPrefix(lower, strings.ToLower(goBin)) {
        return ChannelGoInstall, "path: $GOPATH/bin", true
    }
}
```
**Root cause:** When GOPATH is unset (the overwhelmingly common modern-Go case — `go env GOPATH`
defaults to ~/go), `d.envOr("GOPATH", "")` returns `""`, the `gopath != ""` guard is FALSE, and the
ENTIRE go-install heuristic is skipped. A binary at `~/go/bin/stagecoach` falls through to
`ChannelDirect`. The inline comment CLAIMS "default ~/go/bin when GOPATH is unset" but the code does
NOT implement that fallback. (Verified by the bug-hunt probe: Detector{ExePath:$HOME/go/bin/stagecoach,
Env: GOPATH→""} detects `direct (default (ambiguous))`, not `go-install`.)

**Impact:** A user who installed via `go install ...@latest` and runs `stagecoach upgrade` is misrouted
to the self-swap path instead of `go install @latest` delegation — contradicting FR-U1 (must not
self-swap a channel that should delegate) and FR-U3. (Compounded by BUG-001: the self-swap then fails
the sanity-run, so a go-install user's upgrade fails entirely.)

## 2. envOr (the existing seam — reuse, do NOT add os.Getenv)

**detect.go:395:**
```go
func (d *Detector) envOr(key, fallback string) string {
    if d.Env == nil {
        return fallback
    }
    if v := d.Env(key); v != "" {
        return v
    }
    return fallback
}
```
`d.Env` is `func(string) string` (the Detector field, detect.go:152) — the injectable os.Getenv seam.
Tests inject `Env: func(k string) string { ... }`. The fix MUST use `d.envOr("HOME", "")` (the existing
seam), NOT a hard `os.Getenv("HOME")` — that would bypass the test injection and break determinism.

## 3. The fix (minimal, reuses the existing prefix-check machinery)

Replace the go-install block (detect.go:371-378) with a version that resolves the default ~/go when
GOPATH is empty, then runs the SAME prefix check. An explicit GOPATH always wins; the HOME-based default
applies ONLY when GOPATH is empty:

```go
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
```

**Why this is correct:**
- Explicit GOPATH set → `gopath != ""` immediately → `if gopath == ""` skipped → uses explicit GOPATH,
  evidence `"path: $GOPATH/bin"`. **Existing TestDetect_Path_GoInstallViaGOPATH still passes** (it sets
  GOPATH="/home/me/go"; the test asserts only `ch == ChannelGoInstall`, discarding evidence).
- GOPATH unset, HOME set → `gopath == ""` → resolve `filepath.Join(home,"go")` → prefix check on
  `$HOME/go/bin/`. A binary at `$HOME/go/bin/stagecoach` → ChannelGoInstall, evidence
  `"path: ~/go/bin (default GOPATH)"`. **This is the BUG-002 fix.**
- Both GOPATH and HOME unset → `gopath` stays `""` → `if gopath != ""` skipped → falls through to
  npm/direct. **No false positive** (HOME unknown ⇒ no default to resolve).

The prefix-check line (`goBin := ...; if strings.HasPrefix(real, goBin) || ...`) is byte-identical to
the existing one — only the `gopath` SOURCE and the evidence string change.

## 4. Mode A doc-comment update

The inline comment at detect.go:371-372 currently CLAIMS "default ~/go/bin when GOPATH is unset". After
the fix that claim is TRUE. Rewrite the comment (shown above) to state the fallback is IMPLEMENTED and
explains the HOME resolution + the explicit-GOPATH-wins precedence. No user-facing CLI/config/API doc
change (the item: "No user-facing CLI/config/API surface change").

## 5. Tests to add (internal/upgrade/detect_test.go)

**IMPORTANT import note:** detect_test.go currently imports only `context, errors, strings, testing,
time` — it does NOT import `path/filepath`. The new test uses `filepath.Join` (per the item contract) →
ADD `"path/filepath"` to the import block. (Alternative: use a literal path string and skip the import;
but filepath.Join is clearer and matches the item contract. The import addition is a trivial safe
one-liner — just don't forget it or the test won't compile.)

**Test 1 — TestDetect_Path_GoInstallDefaultGOPATH (the BUG-002 regression):**
```go
func TestDetect_Path_GoInstallDefaultGOPATH(t *testing.T) {
    // BUG-002: GOPATH unset (the common modern-Go case) but HOME set → a binary under $HOME/go/bin
    // must be detected as go-install via the ~/go default (matches `go env GOPATH`).
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
```

**Test 2 — TestDetect_Path_GoInstallNoFalsePositiveWhenHOMEUnset (the no-false-positive guard):**
```go
func TestDetect_Path_GoInstallNoFalsePositiveWhenHOMEUnset(t *testing.T) {
    // BUG-002 guard: both GOPATH and HOME unset → even if ExePath sits under a ~/go/bin-style path,
    // detectPath must NOT falsely detect go-install (no HOME ⇒ no default GOPATH to resolve ⇒ falls
    // through to direct).
    d := &Detector{
        ExePath: "/home/me/go/bin/stagecoach", // looks like a go-install path
        GOOS:    "linux",
        Env: func(k string) string { return "" }, // GOPATH and HOME both unset
    }
    ch, _, ok := d.detectPath()
    if ok && ch == ChannelGoInstall {
        t.Errorf("detectPath false-positive go-install = %q,%v, want no go-install match (HOME unknown)", ch, ok)
    }
}
```
pathHeuristics (detect.go:335-339) contains only Homebrew Cellar, Scoop shims, Nix store — NONE match
"/home/me/go/bin/stagecoach", and it has no "node_modules" segment, so detectPath returns no match
(ok==false) for Test 2. The assertion `ch != ChannelGoInstall` is the precise no-false-positive contract.

## 6. Anchors (verified)

| Symbol | Location | Notes |
|---|---|---|
| `func (d *Detector) detectPath() (Channel, string, bool)` | detect.go:349 | THE function to edit |
| go-install block (the bug) | detect.go:371-378 | the `if gopath := d.envOr("GOPATH",""); gopath != ""` block |
| `func (d *Detector) envOr(key, fallback)` | detect.go:395 | the seam to REUSE (nil-Env guard + non-empty check) |
| `Detector.Env func(string) string` | detect.go:152 | the injectable os.Getenv seam |
| `ChannelGoInstall Channel = "go-install"` | detect.go:47 | the return value |
| `TestDetect_Path_GoInstallViaGOPATH` | detect_test.go:353 | the existing test to MIRROR (must still pass) |
| detect_test.go imports | detect_test.go:3-9 | context/errors/strings/testing/time — NO path/filepath (ADD it) |
| `pathHeuristics` | detect.go:335-339 | Cellar/Scoop/Nix only — no false match on ~/go/bin |

## 7. COORDINATION WITH PARALLEL SIBLINGS (no conflict)

- **P1.M1.T1.S1** (BUG-001, Implementing in parallel): edits `internal/upgrade/stage.go` (sanityCheck).
  DIFFERENT file from detect.go. No merge conflict.
- **P1.M1.T3.S1** (BUG-003, Planned): edits `internal/upgrade/releases.go`. Also a different file.
- **THIS task (P1.M1.T2.S1):** edits `internal/upgrade/detect.go` + `internal/upgrade/detect_test.go` ONLY.

## 8. What this task does NOT do (scope fences)

- Does NOT touch stage.go (BUG-001 = P1.M1.T1.S1) or releases.go (BUG-003 = P1.M1.T3.S1).
- Does NOT add a hard `os.Getenv` call — uses the existing `d.envOr`/`d.Env` seam (testability).
- Does NOT change the prefix-check logic (HasPrefix on `goBin` with trailing separator) — only the
  gopath SOURCE + evidence string change.
- Does NOT change CLI/config/API surface or user-facing docs (Mode A = source doc-comment only).
- Does NOT touch the npm node_modules heuristic or pathHeuristics.

## 9. Validation

- `go build ./...` (detect.go change compiles; no new imports needed in detect.go — filepath/strings already imported).
- `go vet ./internal/upgrade/...`.
- `gofmt -l internal/upgrade/detect.go internal/upgrade/detect_test.go` → empty.
- `go test ./internal/upgrade/ -run 'TestDetect_Path_GoInstall' -v` (the new + existing go-install tests).
- `go test ./internal/upgrade/ -count=1` (the item's exact command — all upgrade tests pass).
- `make test && make lint`.
- Grep guard: `grep -n 'filepath.Join(home, "go")\|default GOPATH' internal/upgrade/detect.go` (the fix);
  `grep -n 'TestDetect_Path_GoInstallDefaultGOPATH\|TestDetect_Path_GoInstallNoFalsePositive' internal/upgrade/detect_test.go` (the tests).