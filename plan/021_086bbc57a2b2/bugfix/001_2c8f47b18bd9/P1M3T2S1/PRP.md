name: "P1.M3.T2.S1 — Add /home/linuxbrew/.linuxbrew/Cellar/ to pathHeuristics (BUG-007)"
description: >
  A ONE-LINE production fix (plus inline comments + one regression test) for BUG-007 (Minor): the
  FR-U2 tier-(c) install-root → channel table `pathHeuristics` in internal/upgrade/detect.go maps the
  Homebrew Cellar roots for Apple Silicon (/opt/homebrew/Cellar/) and Intel macOS (/usr/local/Cellar/)
  but OMITS Linuxbrew on Linux (/home/linuxbrew/.linuxbrew/Cellar/). A Linuxbrew install of stagecoach
  therefore matches NO row, falls through to ChannelDirect, and `stagecoach upgrade` SELF-SWAPS a
  brew-managed binary instead of delegating to `brew upgrade stagecoach` (FR-U1/U3) — the fighting-a-
  package-manager failure the delegate-first design exists to prevent. The fix: add ONE row
  `{prefix: "/home/linuxbrew/.linuxbrew/Cellar/", channel: ChannelBrew}` to the pathHeuristics slice
  (detect.go:370, adjacent to the other Cellar entries), with inline comments annotating the three
  Cellar roots (Apple Silicon / Intel macOS / Linuxbrew) — the contract's optional readability note.
  NO change to detectPath: the matching logic (strings.Contains on a `/`-normalized + lowercased path,
  detect.go:401-404) is GOOS-agnostic and cross-platform, so the new prefix matches
  /home/linuxbrew/.linuxbrew/Cellar/stagecoach/... unchanged. Plus a dedicated regression test
  TestDetect_Path_LinuxbrewCellar in internal/upgrade/detect_test.go (mirrors TestDetect_Path_NixStore,
  GOOS="linux") that FAILS on the current code (detectPath → ChannelDirect,false) and PASSES after the
  row is added. Mode A: code-comment only, NO user-facing doc surface (no README). NOT in scope: BUG-004
  (parallel P1.M3.T1.S1 — edits the defaultQueryTimeout CONSTANT in detect.go's ~line 121 region, NOT
  pathHeuristics; distinct line ranges, zero conflict), BUG-008 (P1.M3.T3.S1, releases.go), detectPath
  itself, the docs sync (P1.M3.T4.S1), go.mod. The struct pathHeuristic{prefix,channel} (detect.go:356)
  and ChannelBrew="brew" (detect.go:39) are reused — no new constant/type.

---

## Goal

**Feature Goal**: Close the BUG-007 detection gap so a Linuxbrew install of stagecoach
(`/home/linuxbrew/.linuxbrew/Cellar/...`) is correctly detected as `ChannelBrew` by the FR-U2 tier-(c)
path heuristics, instead of falling through to `ChannelDirect`. This makes `stagecoach upgrade` DELEGATE
to `brew upgrade stagecoach` (FR-U3) on Linuxbrew — preserving the delegate-first guarantee (FR-U1) that
prevents self-swapping a package-manager-owned binary.

**Deliverable**:
1. `internal/upgrade/detect.go` — add `{prefix: "/home/linuxbrew/.linuxbrew/Cellar/", channel: ChannelBrew}`
   to the `pathHeuristics` slice (after `/usr/local/Cellar/`), with inline comments annotating the three
   Cellar roots.
2. `internal/upgrade/detect_test.go` — add `TestDetect_Path_LinuxbrewCellar` (mirrors
   `TestDetect_Path_NixStore`, `GOOS: "linux"`).

**Success Definition**:
- `pathHeuristics` contains the `/home/linuxbrew/.linuxbrew/Cellar/` → `ChannelBrew` row, placed adjacent
  to the other two Cellar entries.
- `detectPath` returns `(ChannelBrew, "path: /home/linuxbrew/.linuxbrew/Cellar/", true)` for an ExePath
  like `/home/linuxbrew/.linuxbrew/Cellar/stagecoach/1.0/bin/stagecoach` — proven by the new test.
- The new test FAILS on the pre-fix code (detectPath → `ChannelDirect, "", false`) and PASSES after — a
  true regression guard.
- `go build ./...` clean; `go vet ./internal/upgrade/...` clean; `gofmt -l` empty;
  `go test ./internal/upgrade/ -race` green; `make test` + `make lint` clean.
- Scope: `git status --porcelain` == `internal/upgrade/detect.go` + `internal/upgrade/detect_test.go`.
- NO change to `detectPath`, the `defaultQueryTimeout`/`DefaultQueryTimeout` constant (BUG-004's),
  releases.go, go.mod, or any PRD/task file.

## User Persona (if applicable)

**Target User**: A Linux developer who installed stagecoach via Linuxbrew (`brew install stagecoach` on
Linux). On `stagecoach upgrade`, they expect it to delegate to `brew upgrade stagecoach` (the brew
channel's updater), not self-swap the binary.

**Use Case**: User runs `stagecoach upgrade` on a Linuxbrew install. Before the fix: tier-(c) misses
Linuxbrew's Cellar root → `ChannelDirect` → the self-swap path overwrites the brew-managed binary
(reverted on the next `brew upgrade`, corrupting brew's bookkeeping). After: tier-(c) matches →
`ChannelBrew` → the dispatcher runs `brew upgrade stagecoach`.

**User Journey**: `brew install stagecoach` (Linux) → `stagecoach upgrade` → Detect tier-(c) matches
`/home/linuxbrew/.linuxbrew/Cellar/` → `ChannelBrew` → delegate → `brew upgrade stagecoach` streams →
brew manages the upgrade (no fighting).

**Pain Points Addressed**: BUG-007 — the missing Linuxbrew root silently mis-routed Linuxbrew installs to
the direct/self-swap channel, defeating delegate-first (FR-U1) for that install method.

## Why

- **FR-U1/U3 (delegate-first)**: a self-overwrite of a brew-managed binary is reverted on brew's next
  upgrade and corrupts its bookkeeping — the exact v2.1-rejection failure. The Linuxbrew root's absence
  re-opened that hole for Linuxbrew users. One table row closes it.
- **FR-U2 tier-(c) completeness**: the path heuristics exist to recognize well-known install roots.
  Homebrew has THREE Cellar roots (Apple Silicon, Intel macOS, Linuxbrew); the table had two. This adds
  the third — a completeness fix, not new behavior.
- **Trivial + zero-risk**: one slice element + a test. No logic change (detectPath is GOOS-agnostic and
  already matches Contains on a normalized path). No new constant/type/import. The fix is smaller than
  this PRP.
- **Bounded, no-conflict scope**: detect.go pathHeuristics (~line 370) + detect_test.go. The parallel
  BUG-004 edits detect.go's `defaultQueryTimeout` constant (~line 121) — a different region; zero textual
  overlap.

## What

**User-visible behavior**: `stagecoach upgrade` on a Linuxbrew install now delegates to
`brew upgrade stagecoach` (FR-U3) instead of self-swapping. No other behavior change.

**Technical change**: one `pathHeuristics` row + inline comments + one test.

### Success Criteria
- [ ] `internal/upgrade/detect.go` `pathHeuristics` contains
      `{prefix: "/home/linuxbrew/.linuxbrew/Cellar/", channel: ChannelBrew}`, placed immediately after
      the `/usr/local/Cellar/` entry.
- [ ] The three Cellar rows carry inline comments (Apple Silicon / Intel macOS / Linuxbrew) for readability.
- [ ] `internal/upgrade/detect_test.go` adds `TestDetect_Path_LinuxbrewCellar` that constructs
      `&Detector{ExePath: "/home/linuxbrew/.linuxbrew/Cellar/stagecoach/1.0/bin/stagecoach", GOOS: "linux"}`
      and asserts `detectPath()` returns `(ChannelBrew, _, true)`.
- [ ] NO change to `detectPath`, the `pathHeuristic` struct, the `ChannelBrew` constant, or the
      `defaultQueryTimeout`/`DefaultQueryTimeout` constant.
- [ ] `go build ./...` clean; `go vet ./internal/upgrade/...` clean; `gofmt -l` empty on the 2 files.
- [ ] `go test ./internal/upgrade/ -run 'TestDetect_Path_LinuxbrewCellar' -v` PASS;
      `go test ./internal/upgrade/ -race` green; `make test` + `make lint` clean.
- [ ] `git status --porcelain` == `internal/upgrade/detect.go` + `internal/upgrade/detect_test.go`.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact table to edit (pathHeuristics, detect.go:368, quoted with all 5 current rows), the
exact row to add + its placement, the struct + constant to reuse (pathHeuristic{prefix,channel},
ChannelBrew), the proof that detectPath needs NO change (the GOOS-agnostic Contains-on-normalized-path
matching, quoted), the regression test to mirror (TestDetect_Path_NixStore, with the exact Detector
field idiom), the no-conflict analysis vs the parallel BUG-004 (different detect.go region), and 5 grep
guards.

### Documentation & References

```yaml
# MUST READ — the authoritative BUG-007 spec (root cause + fix + files)
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/architecture/bugfix_subsystems.md
  section: "BUG-007 (MINOR): Linuxbrew Cellar root missing from pathHeuristics"
  why: "Gives the exact missing prefix (/home/linuxbrew/.linuxbrew/Cellar/), the exact row to add
        ({prefix, channel: ChannelBrew}), and 'Files to Modify: internal/upgrade/detect.go —
        pathHeuristics table'. Confirms it is a one-row table fix."
  critical: "The fix is ONE slice element. Do NOT touch detectPath — the matching logic is correct; only
             the table was incomplete."

# MUST READ — codebase-specific findings for THIS item (the matching proof + test to clone + no-conflict)
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M3T2S1/research/findings.md
  why: "§0-1 the bug + the one-row fix (with the annotated 3-Cellar-roots form); §2 WHY detectPath needs
        no change (Contains on /-normalized + lowercased path, GOOS-agnostic, cross-GOOS deterministic —
        quoted from detect.go:401-404); §3 the struct + constant to reuse; §4 the regression test to
        mirror (TestDetect_Path_NixStore, GOOS='linux', one-assertion); §5 the no-conflict analysis vs
        BUG-004 (defaultQueryTimeout constant at ~line 121 vs pathHeuristics at ~line 370); §6 scope
        fences; §7 validation cmds; §8 the FR-U1/U3 user-visible effect."
  critical: "detectPath is GOOS-INDEPENDENT — the tier-(c) heuristic matches prefixes on any host GOOS.
             So a dedicated test with GOOS='linux' is the honest mirror of TestDetect_Path_NixStore. Do
             NOT touch the defaultQueryTimeout constant (that's BUG-004 / P1.M3.T1.S1's region)."

# MUST READ — the file being edited (the table + struct + the matching loop)
- file: internal/upgrade/detect.go
  why: "pathHeuristics (line 368-373, the 5-row slice to extend); pathHeuristic struct (line 356:
        {prefix string, channel Channel}); ChannelBrew (line 39: Channel = 'brew'); detectPath (line 378,
        the GOOS-agnostic matcher — line 401-404 is the Contains-on-normalized-prefix loop that needs NO
        change). The pathHeuristics godoc (line 361-367) already cites /opt/homebrew/Cellar/ as the
        Unix-root example — optionally broaden the wording, but it is not required."
  pattern: "A pathHeuristic row is {prefix: '<forward-slash Unix root>', channel: <Channel const>}. Add
            the Linuxbrew row adjacent to the other two Cellar rows. Inline comments annotate each root."
  gotcha: "Do NOT edit detectPath, the struct, ChannelBrew, or defaultQueryTimeout. The ENTIRE production
           change is one slice element (+ comments). Editing detectPath would be scope creep + risk."

# MUST READ — the test file (the regression test to add + the idiom to mirror)
- file: internal/upgrade/detect_test.go
  why: "TestDetect_Path_NixStore (line 388) is the one-assertion per-heuristic test to MIRROR for
        Linuxbrew: `d := &Detector{ExePath: '...', GOOS: 'linux'}; ch, _, ok := d.detectPath();
        if !ok || ch != ChannelNix { t.Errorf(...) }`. TestDetect_Path_BrewCellar (line 374) loops over
        the two macOS Cellar roots with GOOS='darwin' — do NOT restructure it; add a SEPARATE
        TestDetect_Path_LinuxbrewCellar with GOOS='linux' (cleaner, mirrors the per-heuristic convention,
        honest about Linuxbrew being Linux)."
  pattern: "dedicated `func TestDetect_Path_<Root>(t)` with `&Detector{ExePath: '<root>/stagecoach/1.0/
            bin/stagecoach', GOOS: '<host>'}` → `d.detectPath()` → assert `(Channel<X>, _, true)`. The
            ExePath doesn't need to EXIST (detectPath tolerates EvalSymlinks failure and falls back to
            the raw path — detect.go:382-384)."
  gotcha: "The test must FAIL on the pre-fix code (detectPath returns ChannelDirect,'',false → the !ok
           branch fires) and PASS after the row is added — that's what makes it a regression guard."

# CONTEXT — the parallel BUG-004 PRP (no-conflict confirmation)
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M3T1S1/PRP.md
  why: "BUG-004 edits internal/cmd/upgrade_run.go (cmdRunner.Run) + EXPORTS detect.go's
        `defaultQueryTimeout` → `DefaultQueryTimeout` (3 spots around line ~121). It does NOT touch
        pathHeuristics (~line 370). Distinct line ranges in the same file — zero textual overlap, no
        merge conflict. Confirms my scope (pathHeuristics) is isolated."
  critical: "Do NOT rename/touch `defaultQueryTimeout`/`DefaultQueryTimeout` — that constant is BUG-004's
             deliverable. If you see it being exported, that's the parallel item, not yours."

# CONTEXT — PRD §9.29 FR-U1/U2/U3 (the delegate-first guarantee this row restores)
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/prd_snapshot.md
  section: "Overview + Recommendations (Linuxbrew Cellar heuristic) + §9.29 FR-U1/U2/U3 (delegate-first, tier-(c) cascade, the brew row)"
  why: "FR-U1: never overwrite a manager-owned binary except via --force. FR-U2 tier-(c): path heuristics
        on os.Executable() (Homebrew Cellar …). FR-U3 brew row: `brew upgrade stagecoach`. The missing
        Linuxbrew root broke tier-(c) → ChannelDirect → self-swap (FR-U1 violation). One row restores it."
```

### Current Codebase tree (relevant slice)

```bash
internal/upgrade/
  detect.go            # EDIT — pathHeuristics table (line 368): +1 row + inline comments
  detect_test.go       # EDIT — +TestDetect_Path_LinuxbrewCellar (mirror TestDetect_Path_NixStore:388)
  delegate.go / releases.go / resolve.go / download.go / stage.go / version.go  # READ-ONLY — untouched
go.mod                 # READ-ONLY — unchanged (no new import)
```

### Desired Codebase tree with files to be added/modified

```bash
internal/upgrade/
  detect.go            # MODIFIED — pathHeuristics +1 row ({prefix:"/home/linuxbrew/.linuxbrew/Cellar/", channel:ChannelBrew}) + 3 inline Cellar comments
  detect_test.go       # MODIFIED — +func TestDetect_Path_LinuxbrewCellar
# NOTHING ELSE. No new file, no detectPath edit, no constant rename, no go.mod change, no docs.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the ENTIRE production change is one slice element): add
//   {prefix: "/home/linuxbrew/.linuxbrew/Cellar/", channel: ChannelBrew},
// to pathHeuristics (detect.go:368), immediately after the /usr/local/Cellar/ row. Do NOT edit
// detectPath, the pathHeuristic struct, or ChannelBrew — the matching logic is correct; only the table
// was incomplete (BUG-007).

// CRITICAL (detectPath is GOOS-agnostic — no logic change needed): detectPath (detect.go:401-404) matches
// via strings.Contains on a path normalized to '/' + lowercased, for BOTH the ExePath and the prefix.
// /home/linuxbrew/.linuxbrew/Cellar/stagecoach/1.0/bin/stagecoach Contains the new prefix → ChannelBrew.
// The tier-(c) heuristic is cross-GOOS deterministic by design (matches on any host GOOS), so the new
// row works identically in production (linux) and in a cross-GOOS unit test.

// CRITICAL (do NOT touch defaultQueryTimeout / DefaultQueryTimeout): that constant (detect.go ~line 121)
// is the parallel BUG-004 item's (P1.M3.T1.S1) deliverable — it exports the osRunner 3s timeout for the
// production cmdRunner. My edit is pathHeuristics (~line 370) — a different region. Zero overlap; do not
// conflate the two detect.go edits.

// GOTCHA (place the row ADJACENT to the other Cellar entries): the three Homebrew Cellar roots belong
// together (Apple Silicon / Intel macOS / Linuxbrew) for readability — the contract's optional code
// comment. Insert after /usr/local/Cellar/, BEFORE the Scoop row. Do not append at the slice tail.

// GOTCHA (the test ExePath need not EXIST): detectPath tolerates EvalSymlinks failure (detect.go:382-384,
// falls back to the raw ExePath), so a synthetic path like
// /home/linuxbrew/.linuxbrew/Cellar/stagecoach/1.0/bin/stagecoach works in the test without creating any
// file. (Mirrors TestDetect_Path_NixStore, which also uses a non-existent path.)

// GOTCHA (GOOS in the test is for clarity, not matching): detectPath ignores GOOS (only tier-(b) PM
// probes are GOOS-gated). Setting GOOS="linux" in TestDetect_Path_LinuxbrewCellar is honest (Linuxbrew
// is a Linux install) and mirrors TestDetect_Path_NixStore's GOOS="linux", but the match would succeed
// under any GOOS. Use "linux" for semantic accuracy.
```

## Implementation Blueprint

### Data models and structure

None NEW. Reuses `pathHeuristic{prefix string, channel Channel}` (detect.go:356) and `ChannelBrew`
(detect.go:39). No new type, constant, field, or import.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/upgrade/detect.go — add the Linuxbrew Cellar row to pathHeuristics
  - FILE: internal/upgrade/detect.go, the `var pathHeuristics = []pathHeuristic{ … }` block (line 368).
  - CHANGE: insert the new row immediately AFTER `{prefix: "/usr/local/Cellar/", channel: ChannelBrew}`
    and BEFORE the Scoop row. Add inline comments annotating all three Cellar roots:
      var pathHeuristics = []pathHeuristic{
          {prefix: "/opt/homebrew/Cellar/", channel: ChannelBrew},              // Apple Silicon macOS
          {prefix: "/usr/local/Cellar/", channel: ChannelBrew},                 // Intel macOS
          {prefix: "/home/linuxbrew/.linuxbrew/Cellar/", channel: ChannelBrew}, // Linuxbrew on Linux (BUG-007)
          {prefix: `\scoop\shims\`, channel: ChannelScoop},
          {prefix: "/nix/store/", channel: ChannelNix},
          {prefix: "ProgramData/chocolatey/", channel: ChannelChocolatey},
      }
  - NO other edit to detect.go. In particular: do NOT touch detectPath, the pathHeuristic struct,
    ChannelBrew, or defaultQueryTimeout/DefaultQueryTimeout.
  - GOFMT: the inline comments must align (gofmt aligns trailing comments in a block); gofmt -w will fix
    alignment if needed.
  - (Optional) broaden the pathHeuristics godoc (line 362-367) Unix-root example from
    "/opt/homebrew/Cellar/" to mention all three, but this is NOT required (Mode A = code comment only).

Task 2: ADD internal/upgrade/detect_test.go — TestDetect_Path_LinuxbrewCellar (regression test)
  - PLACE: near the other TestDetect_Path_* tests (e.g. right after TestDetect_Path_BrewCellar at line
    386, or beside TestDetect_Path_NixStore at line 388). package upgrade (internal test).
  - BODY (mirror TestDetect_Path_NixStore:388):
      // BUG-007: the Linuxbrew Cellar root (/home/linuxbrew/.linuxbrew/Cellar/) must detect as brew,
      // not fall through to direct (which would self-swap a brew-managed binary — FR-U1). detectPath is
      // GOOS-agnostic (cross-GOOS deterministic), so this matches under any host GOOS; GOOS="linux" is
      // set for semantic accuracy (Linuxbrew is a Linux install).
      func TestDetect_Path_LinuxbrewCellar(t *testing.T) {
          d := &Detector{ExePath: "/home/linuxbrew/.linuxbrew/Cellar/stagecoach/1.0/bin/stagecoach", GOOS: "linux"}
          ch, ev, ok := d.detectPath()
          if !ok || ch != ChannelBrew {
              t.Errorf("detectPath linuxbrew = %q,%q,%v, want brew,true", ch, ev, ok)
          }
      }
  - IMPORTS: none new (detect_test.go already imports testing; Detector/ChannelBrew/detectPath are
    same-package). No new helper.
  - NAMING: TestDetect_Path_LinuxbrewCellar (mirrors TestDetect_Path_NixStore / _BrewCellar / _ScoopShims).
  - FOLLOW pattern: TestDetect_Path_NixStore (line 388) — the one-assertion per-heuristic test.
  - GOTCHA: this test FAILS on the pre-fix code (detectPath → ChannelDirect,"",false → !ok branch) and
    PASSES after Task 1 — confirming it is a real regression guard, not a tautology.

Task 3: VERIFY — build, vet, format, the new test + full upgrade regression, lint, grep guards
  - go build ./...
  - go vet ./internal/upgrade/...
  - gofmt -l internal/upgrade/detect.go internal/upgrade/detect_test.go   # empty
  - go test ./internal/upgrade/ -run 'TestDetect_Path_LinuxbrewCellar' -v  # the new test PASS
  - go test ./internal/upgrade/ -race        # full upgrade regression (all detect/download/resolve/... GREEN)
  - make test ; make lint
  - git status --porcelain   # ONLY detect.go + detect_test.go
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN (the one-row table fix — add adjacent to the other Cellar roots, annotate all three):
var pathHeuristics = []pathHeuristic{
	{prefix: "/opt/homebrew/Cellar/", channel: ChannelBrew},              // Apple Silicon macOS
	{prefix: "/usr/local/Cellar/", channel: ChannelBrew},                 // Intel macOS
	{prefix: "/home/linuxbrew/.linuxbrew/Cellar/", channel: ChannelBrew}, // Linuxbrew on Linux (BUG-007)
	{prefix: `\scoop\shims\`, channel: ChannelScoop},
	{prefix: "/nix/store/", channel: ChannelNix},
	{prefix: "ProgramData/chocolatey/", channel: ChannelChocolatey},
}

// PATTERN (the regression test — mirror TestDetect_Path_NixStore; FAILS pre-fix, PASSES post-fix):
func TestDetect_Path_LinuxbrewCellar(t *testing.T) {
	d := &Detector{ExePath: "/home/linuxbrew/.linuxbrew/Cellar/stagecoach/1.0/bin/stagecoach", GOOS: "linux"}
	ch, ev, ok := d.detectPath()
	if !ok || ch != ChannelBrew {
		t.Errorf("detectPath linuxbrew = %q,%q,%v, want brew,true", ch, ev, ok)
	}
}
```

### Integration Points

```yaml
PRODUCTION (internal/upgrade/detect.go):
  - pathHeuristics slice (line 368): +1 row {prefix: "/home/linuxbrew/.linuxbrew/Cellar/", channel: ChannelBrew}
    + inline comments on the 3 Cellar roots.

TEST (internal/upgrade/detect_test.go):
  - +func TestDetect_Path_LinuxbrewCellar(t *testing.T) (mirror TestDetect_Path_NixStore).

NO detectPath change. NO new constant/type/import. NO database / migration / routes / config / flag / docs.
  - The detectPath matcher (detect.go:401-404) is GOOS-agnostic + cross-platform — unchanged.
  - Mode A: code-comment only; NO README/docs edit (the docs sync is P1.M3.T4.S1, separate).

SCOPE FENCES:
  - Touches ONLY internal/upgrade/detect.go (1 row + comments) + internal/upgrade/detect_test.go (1 test).
  - Does NOT edit detectPath, the pathHeuristic struct, ChannelBrew, defaultQueryTimeout/DefaultQueryTimeout
    (BUG-004's), releases.go (BUG-008's), delegate.go, go.mod, or any PRD/task file.
  - Adds NO flag, NO type, NO constant, NO import, NO dependency.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build (the row compiles; the test compiles).
go build ./...
# Expected: clean.

# Vet.
go vet ./internal/upgrade/...
# Expected: clean.

# Format (gofmt aligns the trailing inline comments in the pathHeuristics block).
gofmt -l internal/upgrade/detect.go internal/upgrade/detect_test.go
# Expected: empty. If listed: gofmt -w internal/upgrade/detect.go internal/upgrade/detect_test.go

# Lint.
make lint   # golangci-lint
# Expected: zero errors.

# Scope guard: ONLY the 2 files changed.
git status --porcelain
# Expected: internal/upgrade/detect.go, internal/upgrade/detect_test.go. ZERO changes to detectPath,
#           defaultQueryTimeout, releases.go, delegate.go, go.mod.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new regression test.
go test ./internal/upgrade/ -run 'TestDetect_Path_LinuxbrewCellar' -v
# Expected: PASS (after Task 1). On the PRE-FIX code this test FAILS (detectPath → ChannelDirect,false).

# Full upgrade-package regression (the new test + all detect/download/resolve/stage/delegate tests).
go test ./internal/upgrade/ -race
# Expected: green. (The existing TestDetect_Path_BrewCellar/NixStore/ScoopShims/GoInstall stay GREEN —
#           the new row does not affect their prefixes.)

# Full race suite.
make test
# Expected: green.
```

### Level 3: Integration Testing (System Validation)

```bash
# This is a unit-level table fix exercised directly via detectPath (no network, no CLI). The user-visible
# integration (a real Linuxbrew install → `stagecoach upgrade` delegates to `brew upgrade`) requires a
# Linuxbrew environment; the unit test is the precise, deterministic proof. Manual smoke (optional, on a
# Linux box with Linuxbrew):
#   brew install stagecoach  (or a stagecoach binary symlinked under /home/linuxbrew/.linuxbrew/Cellar/)
#   stagecoach upgrade   → should delegate to `brew upgrade stagecoach` (not self-swap)
# (The unit test is the real proof; this is a sanity check on a real Linuxbrew install.)
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: the Linuxbrew row exists in pathHeuristics.
grep -n 'prefix: "/home/linuxbrew/.linuxbrew/Cellar/"' internal/upgrade/detect.go
# Expect: 1 hit, with channel: ChannelBrew.

# Guard 2: it is placed adjacent to the other Cellar entries (between /usr/local/Cellar/ and \scoop\shims\).
grep -n -A1 '/usr/local/Cellar/' internal/upgrade/detect.go | grep linuxbrew
# Expect: 1 hit (the Linuxbrew row immediately follows the Intel-macOS row).

# Guard 3: detectPath itself is UNCHANGED (no logic edit).
git diff internal/upgrade/detect.go | grep -E '^[-+].*func \(d \*Detector\) detectPath|^[-+].*strings.Contains'
# Expect: ZERO hits in the detectPath body (the only diff is the +1 pathHeuristics row + comments).

# Guard 4: the regression test exists.
grep -n 'func TestDetect_Path_LinuxbrewCellar' internal/upgrade/detect_test.go
# Expect: 1 hit.

# Guard 5: NO out-of-scope file changed (esp. NOT the BUG-004 constant or releases.go).
git diff --name-only
# Expect: internal/upgrade/detect.go + internal/upgrade/detect_test.go ONLY.
git diff internal/upgrade/detect.go | grep -E 'defaultQueryTimeout|DefaultQueryTimeout' && echo "FAIL: touched BUG-004's constant" || echo "OK: BUG-004 constant untouched"
git diff --name-only | grep -E 'releases\.go|delegate\.go|go\.mod' && echo "FAIL: out-of-scope file" || echo "OK: scope clean"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/upgrade/...` clean
- [ ] `gofmt -l` empty on the 2 files
- [ ] `make lint` zero errors
- [ ] `go test ./internal/upgrade/ -run 'TestDetect_Path_LinuxbrewCellar' -v` PASS
- [ ] `go test ./internal/upgrade/ -race` green
- [ ] `make test` green

### Feature Validation
- [ ] `pathHeuristics` contains `{prefix: "/home/linuxbrew/.linuxbrew/Cellar/", channel: ChannelBrew}` (grep guard 1)
- [ ] It is placed adjacent to the other Cellar entries (grep guard 2)
- [ ] `detectPath` is UNCHANGED — only the table grew (grep guard 3)
- [ ] `TestDetect_Path_LinuxbrewCellar` exists and passes (grep guard 4); it FAILS on pre-fix code (regression guard)
- [ ] A Linuxbrew ExePath → `(ChannelBrew, "path: /home/linuxbrew/.linuxbrew/Cellar/", true)` (the FR-U1/U3 effect)

### Scope-Boundary Validation
- [ ] `git status` shows ONLY `internal/upgrade/detect.go` + `internal/upgrade/detect_test.go` (grep guard 5)
- [ ] NO edit to detectPath, the pathHeuristic struct, ChannelBrew, `defaultQueryTimeout`/`DefaultQueryTimeout`
      (BUG-004's), releases.go (BUG-008's), delegate.go, go.mod, or any PRD/task file
- [ ] NO conflict with the parallel P1.M3.T1.S1 (distinct detect.go regions)
- [ ] NO new flag/type/constant/import/dependency

### Code Quality & Docs
- [ ] The three Cellar rows carry inline comments (Apple Silicon / Intel macOS / Linuxbrew) for readability
- [ ] Mode A: code-comment only; NO README/docs edit (the docs sync is P1.M3.T4.S1)
- [ ] The regression test mirrors the established `TestDetect_Path_*` convention (dedicated, one-assertion)

---

## Anti-Patterns to Avoid

- ❌ Don't edit `detectPath`. The matching logic (Contains on a `/`-normalized + lowercased path, GOOS-
  agnostic) is CORRECT — only the table was incomplete. The entire production change is one slice element.
  Editing detectPath is scope creep and risks breaking the cross-GOOS deterministic matching.
- ❌ Don't touch `defaultQueryTimeout` / `DefaultQueryTimeout`. That constant (detect.go ~line 121) is the
  parallel BUG-004 item's deliverable. My edit is pathHeuristics (~line 370) — a different region. The two
  detect.go edits coexist without conflict; do not conflate them.
- ❌ Don't append the row at the slice tail. The three Homebrew Cellar roots belong TOGETHER (Apple Silicon
  / Intel macOS / Linuxbrew) for readability — insert after `/usr/local/Cellar/`, before the Scoop row.
  (The contract explicitly says "place it adjacent to the other Cellar entries.")
- ❌ Don't restructure `TestDetect_Path_BrewCellar` to add the Linuxbrew path. That loop hardcodes
  `GOOS: "darwin"`; Linuxbrew is a Linux install. Add a DEDICATED `TestDetect_Path_LinuxbrewCellar` with
  `GOOS: "linux"` instead — it mirrors the per-heuristic test convention (NixStore/ScoopShims each have
  their own) and is semantically honest.
- ❌ Don't use a real file for the test ExePath. detectPath tolerates EvalSymlinks failure (falls back to
  the raw path), so a synthetic `/home/linuxbrew/.linuxbrew/Cellar/stagecoach/1.0/bin/stagecoach` works
  without creating anything (mirrors TestDetect_Path_NixStore). Creating files is needless test setup.
- ❌ Don't add docs / README changes. The contract is explicit: "[Mode A] No user-facing doc surface change.
  The detection is internal." The docs sync (docs/packaging.md + README) is P1.M3.T4.S1 — a separate task.
- ❌ Don't add a new constant or type. `ChannelBrew` (detect.go:39) and `pathHeuristic` (detect.go:356)
  already exist — reuse them. The row is `{prefix: "...", channel: ChannelBrew}`; nothing more.
- ❌ Don't conflate this with BUG-008 (escape `c.Repo` in releases.go). That's a different file
  (internal/upgrade/releases.go) and a different task (P1.M3.T3.S1). This item is the pathHeuristics table
  in detect.go only.
- ❌ Don't skip the regression test. Without it, the row has no coverage and the BUG-007 gap can silently
  recur (someone removing the row). The dedicated test is ~6 lines and is the positive proof + regression
  lock. It must FAIL on the pre-fix code (that's what makes it a guard, not a tautology).
- ❌ Don't worry about GOOS in the matcher. detectPath is GOOS-INDEPENDENT (only tier-(b) PM probes are
  GOOS-gated). The new row matches on any host GOOS. Setting `GOOS: "linux"` in the test is for semantic
  clarity, not for the match.