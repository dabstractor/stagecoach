name: "P1.M1.T1.S2 — delegate.go: move Chocolatey RUN→PRINT, add printCommand case, drop runArgv winget case + tests + upgrade comments"
description: >
  Wire the FR-U3/FR-U4 delegation table for Chocolatey (replacing Winget): in internal/upgrade/delegate.go,
  add ChannelChocolatey to the PRINT-channel switch arm, DELETE the winget case from runArgv (it is no
  longer a RUN channel), and add a ChannelChocolatey case to printCommand emitting
  `choco upgrade stagecoach -y` + an FR-U1/FR-U4 admin/ownership comment. Update delegate_test.go (drop
  the winget RUN-table row, drop ChannelWinget from the NeverSudo RUN slice, add a chocolatey PRINT case)
  and the winget→chocolatey narrative comments in delegate.go + upgrade.go + upgrade_run.go. Consumes
  ChannelChocolatey from sibling S1. No non-Windows channel changes; no new types/imports.

---

## Goal

**Feature Goal**: Make `stagecoach upgrade`'s delegation table (FR-U3) treat Chocolatey as a **PRINT**
channel (`choco upgrade stagecoach -y`, exit 0 — never run, never self-swap) instead of the dropped
Winget RUN channel — so a Windows Chocolatey install (detected by S1) prints the admin-required command
for the user rather than auto-running it (FR-U4) or self-swapping a choco-owned binary (FR-U1).

**Deliverable**: (1) delegate.go — add `ChannelChocolatey` to the PRINT switch arm; DELETE the
`case ChannelWinget:` runArgv arm; ADD a `case ChannelChocolatey:` printCommand arm; update 4 narrative
comments; (2) delegate_test.go — delete the winget RUN-table row, drop ChannelWinget from the NeverSudo
RUN slice, add a chocolatey row to `TestDelegate_PrintChannels`; (3) upgrade.go (:4, :79 — :79 is the
user-facing `Long` help) + upgrade_run.go (:266) winget→chocolatey comments.

**Success Definition**:
- `Delegate(ctx, ChannelChocolatey, opts)` returns `Ran:false, ExitCode:0, Command:"choco upgrade stagecoach -y"`, prints the command + an FR-U1/FR-U4 comment, and NEVER calls Exec.
- `runArgv` has NO winget/Chocolatey case (Chocolatey is PRINT-only).
- `go test ./internal/upgrade/...` green; `make test`/`make lint` pass.
- `rg -n winget internal/upgrade/delegate.go internal/upgrade/delegate_test.go internal/cmd/upgrade.go internal/cmd/upgrade_run.go` → ZERO hits.
- No non-Windows channel (brew/scoop/npm/mise/asdf/go-install) or existing PRINT channel (AUR/Nix/Deb/Rpm) behavior changes.

## User Persona (if applicable)

**Target User**: A Windows user who installed stagecoach via `choco install stagecoach` and runs `stagecoach upgrade`.

**Use Case**: `stagecoach upgrade` detects the Chocolatey install (S1) and PRINTs `choco upgrade stagecoach -y` (needs admin — FR-U4) instead of self-swapping a choco-owned binary (FR-U1) or silently failing.

**Pain Points Addressed**: G27/§21.2 — Chocolatey replaces Winget (whose `microsoft/winget-pkgs` Defender install-gate blocked every release). This subtask completes the upgrade-path half: detect (S1) + delegate-to-PRINT (S2).

## Why

- **FR-U3/FR-U4 (P1)**: Chocolatey → PRINT `choco upgrade stagecoach -y`; do NOT run it (admin — FR-U4) and do NOT self-swap (FR-U1: choco owns the binary under `ProgramData\chocolatey`). S1 made the DETECTOR recognize Chocolatey; S2 wires the DELEGATION table so the detected channel prints the right command.
- **Compile coupling**: S1 renames the `ChannelWinget` const→`ChannelChocolatey` in detect.go. delegate.go references that const (`case ChannelWinget:` in runArgv). After S1 lands, delegate.go will NOT compile until S2 replaces the reference. S1+S2 must land together for a green build.

## What

**User-visible behavior**: `stagecoach upgrade` on a Chocolatey install prints `choco upgrade stagecoach -y` (+ a comment) and exits 0; it never runs choco (admin) and never self-swaps. The `--help` Long text (upgrade.go:79) lists Chocolatey instead of Winget.

**Technical change (delegate.go + delegate_test.go + 2 comment files):**
1. Delegate() PRINT switch arm gains `ChannelChocolatey`.
2. runArgv() loses the `ChannelWinget` case.
3. printCommand() gains a `ChannelChocolatey` case.
4. 4 delegate.go narrative comments + upgrade.go(:4,:79) + upgrade_run.go(:266) winget→chocolatey.
5. delegate_test.go: drop winget RUN-table row; drop ChannelWinget from NeverSudo RUN slice; add chocolatey PRINT case.

### Success Criteria
- [ ] `ChannelChocolatey` in the Delegate() PRINT switch arm
- [ ] runArgv() has NO winget/Chocolatey case
- [ ] printCommand() `case ChannelChocolatey:` emits primary `choco upgrade stagecoach -y` + FR-U1/FR-U4 comment
- [ ] Delegate(ctx, ChannelChocolatey, opts) → Ran:false, ExitCode:0, Command=primary, Exec NOT called
- [ ] delegate_test.go: winget RUN-table row deleted; ChannelWinget removed from NeverSudo slice; chocolatey PRINT case added
- [ ] upgrade.go(:4,:79) + upgrade_run.go(:266) winget→chocolatey
- [ ] `rg -n winget` (the 4 S2 files) → empty; `go test ./internal/upgrade/...` green

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — every edit site is verified with the exact code construct + line number, the printCommand/ test assertion shapes are quoted verbatim, the S1 compile-coupling is explained, and the scope fences are explicit.

### Documentation & References

```yaml
- file: internal/upgrade/delegate.go
  why: "THE file. Delegate() switch (~:205): PRINT arm `case ChannelAUR, ChannelNix, ChannelDeb,
        ChannelRpm:` — ADD ChannelChocolatey. runArgv() (~:259): `case ChannelWinget:` at :273-274 —
        DELETE. printCommand() (~:342): per-channel cases; ADD `case ChannelChocolatey:` after ChannelRpm,
        before `default:`. Narrative comments :4, :74, :195, :221 name winget in the RUN list — drop it."
  pattern: "PRINT arm: printCommand(ch) → Fprintln(opts.out(), full) → return {Ran:false, Command:primary,
            ExitCode:0}, nil. printCommand case: `primary = ...; full = primary + \"\\n# (...)\"; return
            primary, full` (mirror AUR/Nix/Deb/Rpm)."
  critical: "ChannelChocolatey is defined in detect.go (S1 renames it from ChannelWinget). After S1 lands,
             `case ChannelWinget:` in runArgv WILL NOT COMPILE — S2 must replace it. Anchor on constructs
             (grep `case ChannelWinget`, `case ChannelAUR, ChannelNix`), not line numbers (they drift)."

- file: internal/upgrade/delegate_test.go
  why: "THE test file. TestDelegate_RunArgvPerChannel (:54 table): winget row at :62 — DELETE.
        TestDelegate_PrintChannels (:142 table): shape {name, ch, primary, containsAll []string};
        assertions Ran=false/ExitCode=0/Command==primary/Exec-NOT-called/Out-contains-all — ADD a
        chocolatey row. TestDelegate_NeverSudo (:336): runChannels slice at :338 — REMOVE ChannelWinget."
  pattern: "PrintChannels row: {name:\"chocolatey\", ch:ChannelChocolatey, primary:\"choco upgrade
            stagecoach -y\", containsAll:[]string{\"choco upgrade stagecoach -y\",\"admin\",\"FR-U1\"}}."
  critical: "NeverSudo's runChannels is RUN channels ONLY. Chocolatey is PRINT now → it leaves the slice
             (6 channels: brew/scoop/npm/mise/asdf/go-install). Do NOT add ChannelChocolatey to runChannels."

- file: internal/cmd/upgrade.go
  why: "winget→chocolatey in 2 comments: :4 (header) and :79 (the cobra `Long` help — USER-FACING via
        `stagecoach upgrade --help`; Mode A docs ride with the work)."
- file: internal/cmd/upgrade_run.go
  why: "winget→chocolatey in the :266 runDelegate comment."

- docfile: plan/020_6979db625159/architecture/upgrade_subsystem.md
  why: "Verbatim change spec for delegate.go (PRINT case add, runArgv delete, printCommand add, narratives)
        + delegate_test.go (RUN-row delete, runChannels, printCommand test). §'File: internal/upgrade/delegate.go'."
- docfile: plan/020_6979db625159/P1M1T1S1/PRP.md
  why: "S1 is the CONTRACT — it produces ChannelChocolatey in detect.go. S2 consumes it. Confirms S1 does
        NOT touch delegate.go (so S2 owns every delegate.go winget reference)."
- docfile: plan/020_6979db625159/P1M1T1S2/research/findings.md
  why: "Verified edit sites, the printCommand/test assertion shapes, the S1 compile-coupling, scope fences."
```

### Current Codebase tree (relevant slice)

```bash
internal/upgrade/
  delegate.go       # THE file: PRINT-case add + runArgv winget-case DELETE + printCommand Chocolatey case + 4 comments
  delegate_test.go  # RUN-table winget row DELETE + NeverSudo runChannels edit + PrintChannels chocolatey row ADD
  detect.go         # S1 owns (ChannelChocolatey const) — UNCHANGED in S2
internal/cmd/
  upgrade.go        # winget→chocolatey in comments :4 + :79 (Long help, user-facing)
  upgrade_run.go    # winget→chocolatey in comment :266
```

### Desired Codebase tree with files to be added

```bash
internal/upgrade/delegate.go       # MODIFY: PRINT-case add + runArgv delete + printCommand add + comments
internal/upgrade/delegate_test.go  # MODIFY: RUN-row delete + NeverSudo edit + PrintChannels row add
internal/cmd/upgrade.go            # MODIFY: 2 winget→chocolatey comments
internal/cmd/upgrade_run.go        # MODIFY: 1 winget→chocolatey comment
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (S1 compile coupling): ChannelChocolatey is defined in detect.go; S1 renames it from
//   ChannelWinget. delegate.go's `case ChannelWinget:` (runArgv :273) won't compile after S1 lands.
//   S2 MUST use ChannelChocolatey everywhere. Confirm S1 landed: `grep -n ChannelChocolatey
//   internal/upgrade/detect.go`. S1+S2 land together for a green build.

// CRITICAL (Chocolatey is PRINT, never RUN): do NOT add a ChannelChocolatey case to runArgv, and do NOT
//   add ChannelChocolatey to NeverSudo's runChannels slice. Chocolatey prints (admin — FR-U4); it is
//   never executed by Delegate. The runArgv delete + the runChannels removal are the LOAD-BEARING parts.

// GOTCHA (printCommand case placement): add `case ChannelChocolatey:` AFTER ChannelRpm and BEFORE the
//   `default:` (which returns "",""). Mirror the AUR/Nix/Deb/Rpm shape: set primary, set full =
//   primary + "\\n# (...)", return primary, full.

// GOTCHA (the printCommand comment): mirror the AUR/.deb/.rpm comment style — explain WHY (choco owns
//   the binary under ProgramData\chocolatey — FR-U1; needs admin — FR-U4), not just "run this". Keep it
//   1-2 lines (choco is simpler than deb/rpm — no repo-fallback needed).

// GOTCHA (PrintChannels test): ride the EXISTING TestDelegate_PrintChannels table (add a row), don't
//   spawn a separate test. The table already asserts Ran=false/ExitCode=0/Command==primary/Exec-NOT-called/
//   Out-contains-all — a chocolatey row satisfies the item's "new printCommand test" idiomatically.

// GOTCHA (Long help is user-facing): upgrade.go:79 is the cobra `Long` text shown by `stagecoach upgrade
//   --help`. The winget→chocolatey edit there IS the Mode A docs change (rides with the work); no separate
//   docs subtask for it. (docs/cli.md / README.md / docs/packaging.md are P1.M3 — separate.)

// SCOPE: S2 is delegate.go + delegate_test.go + upgrade.go(:4,:79) + upgrade_run.go(:266). Do NOT touch
//   detect.go/detect_test.go (S1), .goreleaser.yaml/release.yml/install.ps1 (P1.M2), or
//   docs/cli.md/README.md/docs/packaging.md (P1.M3). No new types/imports; no non-Windows channel change.
```

## Implementation Blueprint

### Data models and structure
None. Pure switch/list/comment/test edits. No new types, consts, or imports (ChannelChocolatey comes from S1's detect.go).

### Implementation Tasks (ordered by dependencies)

> **Prerequisite — S1 has LANDED** (detect.go: `ChannelChocolatey` ×5, zero `ChannelWinget`).
> ⚠️ **LIVE TREE — being actively edited in parallel.** VERIFY each edit site's CURRENT state via grep
> before editing, and drive toward the TARGET state in each task. Observed mid-research (will keep
> drifting): runArgv has an INCORRECT `case ChannelChocolatey:` (returns `choco upgrade stagecoach` —
> a RUN case that violates FR-U4; S2 must DELETE it); the PRINT arm + printCommand do NOT yet have
> Chocolatey; delegate.go has no `winget` text left (the rename swept the comments — verify the PRINT
> narrative names Chocolatey); delegate_test.go's winget row/runChannels entry may already be removed.
> ANCHOR ON CONSTRUCTS via grep, not line numbers. S1+S2 must land together for a green build.

```yaml
Task 1: MODIFY internal/upgrade/delegate.go — add ChannelChocolatey to the PRINT switch arm
  - LOCATE the PRINT arm in Delegate(): grep `case ChannelAUR, ChannelNix, ChannelDeb, ChannelRpm:`.
  - EDIT: add ChannelChocolatey to the case list:
        case ChannelAUR, ChannelNix, ChannelDeb, ChannelRpm, ChannelChocolatey:
  - UPDATE the PRINT narrative comment above it (currently "AUR needs root, Nix is declarative (FR-U4)")
    to also name Chocolatey: e.g. "AUR needs root, Nix is declarative/immutable, Chocolatey needs admin
    and owns the binary (FR-U1/FR-U4)".
  - DEPENDENCIES: S1 (ChannelChocolatey exists).

Task 2: MODIFY internal/upgrade/delegate.go — DELETE the runArgv Chocolatey case (it is NOT a RUN channel)
  - LOCATE: grep `case ChannelChocolatey:` in runArgv (live ~:273). The parallel editor RENAMED the old
    winget case to ChannelChocolatey; it currently returns `[][]string{{"choco", "upgrade", "stagecoach"}}` —
    an INCORRECT RUN case (would auto-run choco, violating FR-U4: admin). Chocolatey is PRINT-only.
  - DELETE the 2-line case (the `case ChannelChocolatey:` line + its `return [][]string{{"choco",...}}` line).
  - AFTER the delete, grep `case Channel` in runArgv → must show Brew/Scoop/Mise/GoInstall/Npm/Asdf ONLY
    (NO Chocolatey, NO winget).
  - DEPENDENCIES: Task 1.

Task 3: MODIFY internal/upgrade/delegate.go — ADD the printCommand Chocolatey case
  - LOCATE the ChannelRpm case in printCommand (grep `case ChannelRpm:`); find its `return primary, full`
    and the `default:` after it.
  - INSERT between ChannelRpm's return and `default:`:
        case ChannelChocolatey:
            // Chocolatey owns the binary under ProgramData\chocolatey (FR-U1: never self-swap) and
            // `choco upgrade` needs admin (FR-U4: print, never auto-elevate). The user runs this.
            primary = "choco upgrade stagecoach -y"
            full = "choco upgrade stagecoach -y\n# (choco owns the binary under ProgramData\\chocolatey; run as admin — FR-U1/FR-U4)"
            return primary, full
  - DEPENDENCIES: Task 1.

Task 4: MODIFY internal/upgrade/delegate.go — update the RUN-list narrative comments + the PRINT narrative
  - GREP FIRST: `rg -n winget internal/upgrade/delegate.go`. The parallel rename may have already swept
    these (the live tree shows zero `winget` in delegate.go). If empty, the winget→choco rename is done;
    instead VERIFY the PRINT narrative (~:191 "ChannelAUR, ChannelNix, ChannelDeb, ChannelRpm → PRINT.
    AUR needs root...") NAMES Chocolatey and explains admin (FR-U4) + ownership (FR-U1) — edit it to
    include ChannelChocolatey if missing. Also confirm the RUN-list comments (:4/:74/:195/:221 if still
    present) no longer list a Windows RUN channel that should be PRINT.
  - TARGET: the PRINT narrative names Chocolatey (admin); no comment claims Chocolatey is RUN.
  - DEPENDENCIES: Tasks 1-2.

Task 5: MODIFY internal/cmd/upgrade.go — winget→chocolatey in comments (:4, :79)
  - LINE 4:  "Homebrew/Scoop/winget/npm/mise/asdf/Nix/AUR/go-install"  →  ".../Scoop/chocolatey/npm/..."
  - LINE 79: "(Homebrew, Scoop, winget, npm, mise, asdf, Nix, AUR, go install)"  →  "(Homebrew, Scoop,
            chocolatey, npm, mise, asdf, Nix, AUR, go install)"  [USER-FACING Long help — Mode A docs]
  - DEPENDENCIES: none.

Task 6: MODIFY internal/cmd/upgrade_run.go — winget→chocolatey in comment (:266)
  - LINE 266: "brew/scoop/winget/"  →  "brew/scoop/chocolatey/"
  - DEPENDENCIES: none.

Task 7: MODIFY internal/upgrade/delegate_test.go — 3 edits
  - EDIT TestDelegate_RunArgvPerChannel (:54 table): DELETE the winget row at :62
        {"winget", ChannelWinget, [][]string{{"winget", "upgrade", "stagecoach"}}},
    (If S1 renamed it, delete the ChannelChocolatey/choco row that replaced it — runArgv has NO
    Chocolatey case after Task 2, so it must not be in the RUN-argv table.)
  - EDIT TestDelegate_PrintChannels (:142 table): ADD a chocolatey row (rides the existing table):
        {
            name:        "chocolatey",
            ch:          ChannelChocolatey,
            primary:     "choco upgrade stagecoach -y",
            containsAll: []string{"choco upgrade stagecoach -y", "admin", "FR-U1"},
        },
  - EDIT TestDelegate_NeverSudo (:336): REMOVE ChannelWinget from the runChannels slice at :338
        ChannelBrew, ChannelScoop, ChannelNpm, ChannelMise, ChannelAsdf, ChannelGoInstall,  // 6 (was 7)
  - DEPENDENCIES: Tasks 1-3.

Task 8: VERIFY — green tests + the grep gate
  - go build ./...  (confirms S1+S2 compile together)
  - go test ./internal/upgrade/... -v  (EXPECT green: PrintChannels/chocolatey passes; RunArgv has no winget; NeverSudo has 6)
  - rg -n winget internal/upgrade/delegate.go internal/upgrade/delegate_test.go internal/cmd/upgrade.go internal/cmd/upgrade_run.go  (EXPECT ZERO hits)
  - make test && make lint
```

### Implementation Patterns & Key Details

```go
// PATTERN: the PRINT switch arm (add ChannelChocolatey to the existing list)
case ChannelAUR, ChannelNix, ChannelDeb, ChannelRpm, ChannelChocolatey:
    primary, full := printCommand(ch)
    fmt.Fprintln(opts.out(), full)
    return DelegateResult{Ran: false, Command: primary, ExitCode: 0}, nil

// PATTERN: the printCommand case (mirror AUR/Nix/Deb/Rpm: primary + a "# (...)" full)
case ChannelChocolatey:
    primary = "choco upgrade stagecoach -y"
    full = "choco upgrade stagecoach -y\n# (choco owns the binary under ProgramData\\chocolatey; run as admin — FR-U1/FR-U4)"
    return primary, full

// PATTERN: the PrintChannels test row (rides the existing table; asserts Ran:false/ExitCode:0/Command/Exec-NOT-called)
{name: "chocolatey", ch: ChannelChocolatey, primary: "choco upgrade stagecoach -y",
 containsAll: []string{"choco upgrade stagecoach -y", "admin", "FR-U1"}}
```

### Integration Points

```yaml
NO struct / type / import / public-API changes. Switch/list/comment/test edits only.

CODE:
  - internal/upgrade/delegate.go — PRINT arm(+Chocolatey) + runArgv(-winget) + printCommand(+Chocolatey) + 4 comments
  - internal/cmd/upgrade.go — 2 winget→chocolatey comments (:4, :79 Long help)
  - internal/cmd/upgrade_run.go — 1 winget→chocolatey comment (:266)
TESTS:
  - internal/upgrade/delegate_test.go — RunArgv(-winget row) + NeverSudo(-ChannelWinget) + PrintChannels(+chocolatey)

CONSUMED (from S1, must land first):
  - ChannelChocolatey = "chocolatey" (internal/upgrade/detect.go)

UNCHANGED: detect.go/detect_test.go (S1); .goreleaser.yaml/release.yml/install.ps1 (P1.M2);
  docs/cli.md/README.md/docs/packaging.md (P1.M3); every non-Windows channel; existing PRINT channels.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...     # S1+S2 compile together (ChannelChocolatey resolves; no stale ChannelWinget)
go vet ./...
gofmt -l internal/upgrade/ internal/cmd/
# Expected: empty.
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The upgrade package — delegate + delegate_test are the changed surface
go test ./internal/upgrade/... -v
# Expected: ALL pass — PrintChannels/chocolatey (Ran:false, ExitCode:0, Command="choco upgrade stagecoach -y",
#           Exec NOT called, Out has the FR-U1/admin comment); RunArgvPerChannel has NO winget row;
#           NeverSudo iterates 6 RUN channels (no ChannelWinget).

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# (S2 is delegation-table logic with an injectable Exec seam — the unit suite IS the proof.
#  The end-to-end "choco install → stagecoach upgrade PRINTs choco upgrade -y" is covered by the
#  PrintChannels/chocolatey row + S1's detection test. No binary/network behavior to smoke here.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# THE grep gate — zero winget hits in the 4 S2 files
rg -n winget internal/upgrade/delegate.go internal/upgrade/delegate_test.go internal/cmd/upgrade.go internal/cmd/upgrade_run.go
# Expected: EMPTY. (detect.go/detect_test.go are S1's grep scope — they may still have winget if S1 hasn't
#           landed; that's S1's gate, not S2's.)

# Grep guard: ChannelChocolatey wired into the PRINT path (not RUN)
rg -n "ChannelChocolatey" internal/upgrade/delegate.go
# Expected: PRINT switch arm + printCommand case (2 hits). NOT in runArgv, NOT in NeverSudo runChannels.

# Grep guard: runArgv has NO winget/Chocolatey case (Chocolatey is PRINT-only)
rg -n "case ChannelWinget|case ChannelChocolatey" internal/upgrade/delegate.go
# Expected: only `case ChannelChocolatey:` in printCommand (1 hit). NO ChannelChocolatey in runArgv,
#           NO ChannelWinget anywhere. (The parallel editor left an INCORRECT runArgv ChannelChocolatey
#           case — Task 2 deletes it; this grep confirms the deletion landed.)

# Grep guard: the choco print command + FR-U1/FR-U4 comment
rg -n "choco upgrade stagecoach -y" internal/upgrade/delegate.go
# Expected: printCommand primary (1 hit).

# Scope-boundary guard: detect.go NOT edited by S2 (S1 owns it)
git diff --name-only | grep detect
# Expected: empty (S2 touches delegate.go/delegate_test.go/upgrade.go/upgrade_run.go only).

# Scope-boundary guard: no non-Windows channel changed
git diff internal/upgrade/delegate.go | grep -E '^[-+].*ChannelBrew|ChannelScoop|ChannelNpm|ChannelMise|ChannelAsdf|ChannelGoInstall|ChannelAUR|ChannelNix|ChannelDeb|ChannelRpm'
# Expected: empty (only ChannelChocolatey/ChannelWinget lines changed).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean (S1+S2 compile together)
- [ ] `go vet ./...` clean
- [ ] `gofmt -l internal/upgrade/ internal/cmd/` empty
- [ ] `make lint` zero errors
- [ ] `go test ./internal/upgrade/...` green; `make test` green

### Feature Validation
- [ ] `ChannelChocolatey` in the Delegate() PRINT switch arm
- [ ] runArgv() has NO winget/Chocolatey case
- [ ] printCommand() `case ChannelChocolatey:` emits `choco upgrade stagecoach -y` + FR-U1/FR-U4 comment
- [ ] Delegate(ctx, ChannelChocolatey, opts) → Ran:false, ExitCode:0, Command=primary, Exec NOT called
- [ ] delegate_test.go: winget RUN-table row deleted; ChannelWinget removed from NeverSudo slice; chocolatey PrintChannels row added
- [ ] upgrade.go(:4,:79) + upgrade_run.go(:266) winget→chocolatey
- [ ] `rg -n winget` (the 4 S2 files) → empty

### Scope-Boundary Validation
- [ ] detect.go / detect_test.go UNCHANGED (S1)
- [ ] .goreleaser.yaml / release.yml / install.ps1 UNCHANGED (P1.M2)
- [ ] docs/cli.md / README.md / docs/packaging.md UNCHANGED (P1.M3) — upgrade.go:79 Long help IS in-scope (Mode A)
- [ ] No non-Windows RUN channel changed; no existing PRINT channel changed
- [ ] No new types / imports

### Code Quality
- [ ] printCommand Chocolatey comment explains WHY (FR-U1 ownership + FR-U4 admin), mirroring AUR/deb/rpm style
- [ ] NeverSudo runChannels has 6 channels (Chocolatey correctly absent — it's PRINT)
- [ ] PrintChannels chocolatey row rides the existing table (not a separate test)

---

## Anti-Patterns to Avoid

- ❌ Don't add a `case ChannelChocolatey:` to runArgv — Chocolatey is PRINT-only (admin — FR-U4; choco owns the binary — FR-U1). The runArgv winget case is DELETED, not renamed.
- ❌ Don't add ChannelChocolatey to NeverSudo's `runChannels` slice — that slice is RUN channels only; Chocolatey is PRINT. Remove ChannelWinget (→ 6 channels); do not "replace" it with ChannelChocolatey.
- ❌ Don't spawn a separate printCommand test — ride the existing `TestDelegate_PrintChannels` table (add a chocolatey row). It already asserts Ran:false/ExitCode:0/Command/Exec-NOT-called/Out-contains-all.
- ❌ Don't leave the printCommand Chocolatey comment as a bare "run this" — mirror AUR/.deb/.rpm: explain WHY (choco owns the binary; needs admin). Cite FR-U1/FR-U4.
- ❌ Don't edit detect.go/detect_test.go — S1 owns the const rename + pmProbe + pathHeuristic. delegate.go is S2's only upgrade-package code file.
- ❌ Don't touch .goreleaser.yaml/release.yml/install.ps1 (P1.M2) or docs/cli.md/README.md/docs/packaging.md (P1.M3). The upgrade.go:79 Long help IS in-scope (Mode A — user-facing, rides with the work).
- ❌ Don't change any non-Windows channel (brew/scoop/npm/mise/asdf/go-install) or the existing PRINT channels (AUR/Nix/Deb/Rpm).
- ❌ Don't trust pre-S1 line numbers blindly — S1's detect.go rename doesn't shift delegate.go, but anchor on constructs (grep the case names) since the file may be mid-edit. The winget case at :273-274 is the delete target; confirm by grep.
- ❌ Don't forget S1 must land first — `ChannelChocolatey` is defined in detect.go; delegate.go won't compile until S1+S2 are both in. Confirm S1 landed before asserting a green build.

---

## Confidence Score: 9/10

One-pass success is very high: mechanical switch/list/comment/test edits with every site verified (the
PRINT arm, the runArgv winget case, the printCommand case structure, the 3 test sites, and the 3 comment
files), the printCommand and PrintChannels-test assertion shapes quoted verbatim, and the architecture
doc specifying each change line-for-line. The -1 is for the S1 compile-coupling: delegate.go references
the const S1 renames, so S2 cannot produce a green build in isolation — the implementer must confirm S1
landed and use `ChannelChocolatey` (not ChannelWinget), and the two must merge together. Mitigated by the
explicit prerequisite check + the grep gate (zero winget hits confirms the rename is complete across both).