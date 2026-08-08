name: "P1.M1.T1.S1 — detect.go: rename ChannelWinget→ChannelChocolatey, swap pmProbe, add path heuristic, update tests"
description: >
  Replace the Winget install-detection channel with Chocolatey across internal/upgrade/detect.go +
  detect_test.go: rename the const ChannelWinget→ChannelChocolatey ("chocolatey"), swap the winget
  pmProbe row to a choco row using exit0Confirm (choco list --local-only <pkg> exits 0 iff installed —
  stronger than the old grepConfirm), add a Chocolatey pathHeuristic entry (ProgramData/chocolatey/),
  update validChannel/knownChannelList/all narrative comments, and convert the winget test case +
  GOOS-gating assertions. No non-Windows channel changes; no new types/imports. S1 = detect.go +
  detect_test.go ONLY (delegate.go is S2; release/docs are P1.M2/P1.M3).

---

## Goal

**Feature Goal**: Make `stagecoach upgrade`'s install-method detector (FR-U2) recognize a **Chocolatey**
install instead of a (dropped) Winget install, so a Windows user who ran `choco install stagecoach` is
detected as the Chocolatey channel (FR-U1: choco owns the binary under `ProgramData\chocolatey`; FR-U3/
FR-U4: PRINT `choco upgrade stagecoach -y`, do NOT self-swap). This subtask is the detect.go + test half;
the delegate-table move (RUN→PRINT) lands in S2.

**Deliverable**: (1) `ChannelChocolatey = "chocolatey"` replacing `ChannelWinget`; (2) the choco pmProbe
row (exit0Confirm, `choco list --local-only stagecoach`) replacing the winget row; (3) a Chocolatey
pathHeuristic entry; (4) validChannel/knownChannelList + all 6 narrative comments updated; (5) the
detect_test.go winget case → choco case + GOOS-gating assertions updated.

**Success Definition**:
- `go test ./internal/upgrade/...` is green.
- `rg -n winget internal/upgrade/detect.go internal/upgrade/detect_test.go` returns ZERO hits.
- The choco probe uses `exit0Confirm` (exit code, not stdout grep); the test's canned response returns
  exit 0 for "choco".
- A `ProgramData/chocolatey/` ExePath resolves to ChannelChocolatey via the path heuristic.
- No non-Windows channel (brew/scoop/npm/mise/asdf/aur/deb/rpm/nix/go-install/direct) behavior changes.

## Why

- **G27/§21.2 (v3.3)**: Chocolatey replaces Winget as the Windows package-manager channel because
  `microsoft/winget-pkgs` runs a Microsoft Defender install-gate that hard-blocks the unsigned binary
  every release; Chocolatey (goreleaser-native `chocolatey:` pipe) has no such tax. FR-U2 lists
  Chocolatey in the detection cascade (`choco list --local-only stagecoach` + `ProgramData\chocolatey`
  path heuristic); FR-U3 delegates Chocolatey to a PRINT (`choco upgrade stagecoach -y`); FR-U1/FR-U4
  forbid self-swap (admin + choco-owned binary). This subtask makes the DETECTOR recognize Chocolatey;
  S2 wires the delegation table.

## What

**User-visible behavior**: A Windows Chocolatey install is now detected as the `chocolatey` channel
(was: not recognized / would have fallen through to `direct` and self-swapped a choco-owned binary —
the exact FR-U1 violation the rename prevents).

**Technical change (detect.go + detect_test.go only):**
- Rename const + update the 2 switch/list sites + 6 narrative comments.
- Swap the pmProbe row (winget/grepConfirm → choco/exit0Confirm).
- Add a pathHeuristic row.
- Convert the test case + GOOS-gating assertions.

### Success Criteria
- [ ] `ChannelChocolatey = "chocolatey"` replaces `ChannelWinget`
- [ ] choco pmProbe uses `exit0Confirm` + args `["list","--local-only","stagecoach"]`
- [ ] `ProgramData/chocolatey/` pathHeuristic entry present (forward slashes)
- [ ] validChannel + knownChannelList reference ChannelChocolatey
- [ ] All 6 narrative comments name chocolatey/choco (no winget)
- [ ] detect_test.go: choco case (exit-0 canned, want ChannelChocolatey) + GOOS-gating assertions updated
- [ ] `rg -n winget internal/upgrade/detect.go internal/upgrade/detect_test.go` → empty
- [ ] `go test ./internal/upgrade/...` green; no non-Windows channel changed

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — every line number is verified, the exit0Confirm/grepConfirm distinction (the one logic change) is explained with the test mechanics, the forward-slash pathHeuristic convention is confirmed against the normalization code, and the scope boundaries (S2 = delegate.go) are explicit.

### Documentation & References

```yaml
- file: internal/upgrade/detect.go
  why: "THE file. Const block (39-50, ChannelWinget@41); validChannel switch (62-68, line 64); knownChannelList (240-245, line 242); pmProbes (266-276, winget row@272); pathHeuristics (349-353); normalization code (385-392 — confirms forward-slash convention); narrative comments @7,87,150,265,283,311."
  pattern: "exit0Confirm (line 280) checks exitCode==0 ONLY; grepConfirm (285) checks stdout contains needle. The choco probe switches from grepConfirm to exit0Confirm."
  gotcha: "Lines 265 + 283 narrate 'winget/npm/mise/asdf list everything + grep' — choco no longer belongs in that group (it's exit-0 now). Update BOTH narratives so the comment matches the new exit0Confirm semantics."

- file: internal/upgrade/detect_test.go
  why: "THE test file. TestValidChannel slice (47, ChannelWinget); the 'winget installed (windows)' case (221-229); TestDetect_GOOSGating (306-337, called('winget')@311, darwin banned list@322, windows banned list@335)."
  pattern: "fakeRunner.Run returns canned(name,args)=(stdout,exitCode,err). For the choco case: canned returns ('',0,nil) when n=='choco' (exit0Confirm checks exit code only); ('',1,nil) for every other name."
  gotcha: "The windows banned list (line 335: brew/pacman/dpkg/rpm) must NOT gain 'choco' — choco RUNS on windows. Only the linux/darwin banned lists (311-312, 322) swap winget→choco."

- file: internal/upgrade/detect.go (pathHeuristic normalization, 385-392)
  why: "Confirms the forward-slash convention: matchPath := ReplaceAll(lower,'\\\\','/'); prefix := ReplaceAll(ToLower(h.prefix),'\\\\','/'). So 'ProgramData/chocolatey/' matches a Clean'd 'C:\\ProgramData\\chocolatey\\...' path post-normalization. Same convention as the Scoop '\\scoop\\shims\\' entry."

- docfile: plan/020_6979db625159/architecture/upgrade_subsystem.md
  why: "The line-for-line change spec for detect.go (const/switch/list/pmProbes/pathHeuristics/comments)."
- docfile: plan/020_6979db625159/P1M1T1S1/research/verification_deltas.md
  why: "The verified line-number tables, the exit0Confirm/grepConfirm distinction, the fakeRunner mechanics, the forward-sslash rationale, and the scope boundaries. READ THIS before editing."
```

### Current Codebase tree (relevant slice)

```bash
internal/upgrade/
  detect.go            # THE file: const rename + pmProbe swap + pathHeuristic add + validChannel/knownChannelList + 6 comments
  detect_test.go       # TestValidChannel slice + winget case→choco + GOOS-gating assertions
  delegate.go          # S2 (RUN→PRINT for Chocolatey) — UNTOUCHED in S1
  upgrade.go           # S2 (comments) — UNTOUCHED in S1
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the ONE logic change — not just a rename): the choco probe uses exit0Confirm (exit code==0),
//   NOT grepConfirm (stdout contains "stagecoach"). `choco list --local-only stagecoach` exits 0 iff the
//   package is installed locally — a stronger, more precise check than grep. The test's canned response
//   must therefore return EXIT 0 for "choco" (stdout content is irrelevant to exit0Confirm).

// CRITICAL (narrative accuracy): lines 265 + 283 group winget WITH npm/mise/asdf as "list everything +
//   grep". After the swap, choco is exit-0 (like brew/scoop/pacman), NOT grep. Update BOTH narratives so
//   the comment does not claim choco "lists everything and we grep" — that would contradict exit0Confirm.

// GOTCHA (pathHeuristic separator): use FORWARD slashes in the prefix ("ProgramData/chocolatey/") — the
//   matching code (385-392) normalizes both sides to '/' before Contains, so a forward-slash prefix
//   matches a Clean'd backslash Windows path. This is the same convention as the Scoop entry. Either
//   separator works post-normalize, but forward slashes is the cleaner OS-agnostic form.

// GOTCHA (windows banned list): TestDetect_GOOSGating's windows banned list (line 335: brew/pacman/dpkg/rpm)
//   must NOT gain "choco" — choco is a WINDOWS probe and SHOULD run on GOOS=windows. Only the linux (311-312)
//   and darwin (322) banned lists swap winget→choco (choco must NOT run on linux/darwin).

// SCOPE: S1 is detect.go + detect_test.go ONLY. Do NOT edit delegate.go/upgrade.go (S2 owns the
//   RUN→PRINT delegation move), .goreleaser.yaml/release.yml/install.ps1 (P1.M2), or docs (P1.M3).
//   delegate.go/upgrade.go MAY still reference winget/ChannelWinget after S1 — that's S2's job.
```

## Implementation Blueprint

### Data models and structure

No struct/type changes. One const rename, one table-row replacement, one table-row addition, two
switch/list identifier renames, six comment edits, and the matching test edits. No new imports.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/upgrade/detect.go — const rename + validChannel + knownChannelList
  - LINE 41: ChannelWinget Channel = "winget"  →  ChannelChocolatey Channel = "chocolatey"
    UPDATE the const's doc/inline comment to cite FR-U2 (detection) + FR-U3 (PRINT channel: choco upgrade needs admin).
  - LINE 64 (validChannel switch): ChannelWinget  →  ChannelChocolatey
  - LINE 242 (knownChannelList): string(ChannelWinget)  →  string(ChannelChocolatey)
  - DEPENDENCIES: none.

Task 2: MODIFY internal/upgrade/detect.go — swap the pmProbe row (line 272)
  - REPLACE:
        {channel: ChannelWinget, goos: []string{"windows"}, name: "winget", args: []string{"list"}, confirm: grepConfirm("stagecoach")},
    WITH:
        {channel: ChannelChocolatey, goos: []string{"windows"}, name: "choco", args: []string{"list", "--local-only", "stagecoach"}, confirm: exit0Confirm},
  - CRITICAL: confirm is exit0Confirm (NOT grepConfirm). choco list --local-only <pkg> exits 0 iff installed.
  - DEPENDENCIES: Task 1.

Task 3: MODIFY internal/upgrade/detect.go — add the Chocolatey pathHeuristic (line 349-353)
  - ADD a row to the pathHeuristics slice:
        {prefix: "ProgramData/chocolatey/", channel: ChannelChocolatey},
  - Use FORWARD slashes (the matching code at 385-392 normalizes both sides to '/'). Matches the Scoop convention.
  - DEPENDENCIES: Task 1.

Task 4: MODIFY internal/upgrade/detect.go — update the 6 narrative comments
  - LINE 7:   "brew/scoop/winget/pacman/npm/mise/asdf"  →  "brew/scoop/chocolatey/pacman/npm/mise/asdf"
  - LINE 87:  "brew/scoop/winget"  →  "brew/scoop/chocolatey"
  - LINE 150: "winget⇒windows"  →  "chocolatey⇒windows" (or "choco⇒windows")
  - LINE 265: "winget/npm/mise/asdf list everything and we grep the listing"  →  REMOVE choco from the grep
              group; choco now uses exit0Confirm (like brew/scoop/pacman). Reword to list only npm/mise/asdf as the grep group.
  - LINE 283: "winget/npm/mise/asdf list every package and cannot exit-nonzero"  →  same: choco is now exit-0; reword.
  - LINE 311: "winget/scoop are Windows-only"  →  "choco/scoop are Windows-only"
  - DEPENDENCIES: Tasks 1-2 (so the comments match the new exit0Confirm semantics).

Task 5: MODIFY internal/upgrade/detect_test.go — TestValidChannel + the install case + GOOS-gating
  - LINE 47 (TestValidChannel slice): ChannelWinget  →  ChannelChocolatey
  - LINES 221-229 (the install case):
        name: "chocolatey installed (windows)", goos: "windows",
        canned: func(n string, _ []string) (string, int, error) {
            if n == "choco" { return "", 0, nil }   // exit0Confirm checks exit code ONLY; stdout irrelevant
            return "", 1, nil
        },
        want: ChannelChocolatey,
  - LINES 311-312 (linux GOOS-gating): r.called("winget") → r.called("choco"); error msg "winget probe" → "choco probe"
  - LINE 322 (darwin banned list): {"winget", "scoop", "pacman", "dpkg", "rpm"} → {"choco", "scoop", "pacman", "dpkg", "rpm"}
  - LINE 335 (windows banned list): UNCHANGED — {"brew", "pacman", "dpkg", "rpm"} (choco is NOT banned on windows)
  - LINES 13, 27 (comments): winget → chocolatey / choco
  - DEPENDENCIES: Tasks 1-4.

Task 6: VERIFY — green tests + zero winget hits in the two files
  - RUN: go test ./internal/upgrade/...  (EXPECT green)
  - RUN: rg -n winget internal/upgrade/detect.go internal/upgrade/detect_test.go  (EXPECT zero hits)
  - DEPENDENCIES: Tasks 1-5.
```

### Implementation Patterns & Key Details

```go
// PATTERN: the choco pmProbe — exit0Confirm, not grepConfirm (the substantive logic change)
{channel: ChannelChocolatey, goos: []string{"windows"}, name: "choco",
 args: []string{"list", "--local-only", "stagecoach"}, confirm: exit0Confirm},
// exit0Confirm (_ string, exitCode int) bool { return exitCode == 0 }  — checks exit code ONLY.

// PATTERN: the pathHeuristic — forward slashes (matches the 385-392 normalization)
{prefix: "ProgramData/chocolatey/", channel: ChannelChocolatey},
// matchPath = ReplaceAll(ToLower(cleaned), "\\", "/"); prefix = ReplaceAll(ToLower(h.prefix), "\\", "/")
// ⇒ "c:/programdata/chocolatey/bin/stagecoach.exe" CONTAINS "programdata/chocolatey/" ✓

// PATTERN: the test canned response — exit 0 for choco (exit0Confirm), exit 1 for everything else
canned: func(n string, _ []string) (string, int, error) {
    if n == "choco" { return "", 0, nil }
    return "", 1, nil
}
```

### Integration Points

```yaml
NO struct / type / import / public-API changes. Const rename + table edits + comment edits + test edits.

CODE:
  - internal/upgrade/detect.go — const(41) + validChannel(64) + knownChannelList(242) + pmProbe(272) + pathHeuristic(add) + 6 comments
TESTS:
  - internal/upgrade/detect_test.go — TestValidChannel(47) + install case(221-229) + GOOS-gating(311,322) + 2 comments

UNCHANGED: every non-Windows channel; delegate.go/upgrade.go (S2); release plumbing (P1.M2); docs (P1.M3).

DOWNSTREAM (consumes ChannelChocolatey — do NOT implement in S1):
  - P1.M1.T1.S2: delegate.go moves Chocolatey RUN→PRINT (FR-U4 admin); upgrade.go comments.
  - P1.M2.*: .goreleaser.yaml chocolateys pipe; delete winget CI job; install.ps1.
  - P1.M3.*: docs/packaging.md + docs/cli.md + README.md.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...
go vet ./...
gofmt -l internal/upgrade/
# Expected: empty.
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The upgrade package — detect + detect_test are the changed surface
go test ./internal/upgrade/... -v
# Expected: ALL pass, incl. the renamed "chocolatey installed (windows)" case and the updated GOOS-gating
#           assertions (choco not called on linux/darwin; choco IS callable on windows).

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# (S1 is detection logic with no I/O beyond the injectable Exec seam — the unit suite IS the proof.
#  The full end-to-end "choco install → stagecoach upgrade PRINTs choco upgrade" path lands in S2's
#  delegate-table wiring + its tests. S1's gate is: detect recognizes Chocolatey + tests green.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# THE grep gate — zero winget hits in the two S1 files
rg -n winget internal/upgrade/detect.go internal/upgrade/detect_test.go
# Expected: EMPTY (no straggler winget reference).

# Grep guard: ChannelChocolatey is wired everywhere ChannelWinget was (in detect.go)
rg -n "ChannelChocolatey" internal/upgrade/detect.go
# Expected: const(41) + validChannel(64) + knownChannelList(242) + pmProbe(272) + pathHeuristic(add) = 5 hits.

# Grep guard: the choco probe uses exit0Confirm (the logic change), not grepConfirm
rg -n "choco.*exit0Confirm|exit0Confirm" internal/upgrade/detect.go | grep choco
# Expected: the choco pmProbe row with confirm: exit0Confirm.

# Grep guard: the pathHeuristic entry exists with forward slashes
rg -n "ProgramData/chocolatey" internal/upgrade/detect.go
# Expected: the pathHeuristic row.

# Scope-boundary guard: delegate.go/upgrade.go NOT edited in S1 (S2 owns them — they MAY still say winget)
git diff --name-only
# Expected: only internal/upgrade/detect.go + internal/upgrade/detect_test.go.

# Non-Windows-channel guard: brew/scoop/npm/mise/asdf rows unchanged
git diff internal/upgrade/detect.go | grep -E '^[-+].*ChannelBrew|ChannelScoop|ChannelNpm|ChannelMise|ChannelAsdf|ChannelAUR|ChannelDeb|ChannelRpm|ChannelNix|ChannelGoInstall|ChannelDirect'
# Expected: empty (no non-Windows channel row changed).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `gofmt -l internal/upgrade/` empty
- [ ] `make lint` zero errors
- [ ] `go test ./internal/upgrade/...` green; `make test` green

### Feature Validation
- [ ] `ChannelChocolatey = "chocolatey"` replaces `ChannelWinget`
- [ ] choco pmProbe uses `exit0Confirm` + `choco list --local-only stagecoach`
- [ ] `ProgramData/chocolatey/` pathHeuristic entry (forward slashes)
- [ ] validChannel + knownChannelList reference ChannelChocolatey
- [ ] All 6 narrative comments updated (incl. the grep-group narratives at 265/283)
- [ ] detect_test.go: choco case (exit-0 canned, want ChannelChocolatey) + GOOS-gating updated
- [ ] `rg -n winget internal/upgrade/detect.go internal/upgrade/detect_test.go` → empty

### Scope-Boundary Validation
- [ ] delegate.go / upgrade.go UNCHANGED (S2)
- [ ] .goreleaser.yaml / release.yml / install.ps1 UNCHANGED (P1.M2)
- [ ] docs UNCHANGED (P1.M3)
- [ ] No non-Windows channel row changed
- [ ] No new types / imports

### Code Quality
- [ ] Comments match the new exit0Confirm semantics (no stale "choco lists everything + grep" claim)
- [ ] pathHeuristic prefix uses forward slashes (the normalization convention)
- [ ] windows banned list correctly excludes choco (it runs on windows)

---

## Anti-Patterns to Avoid

- ❌ Don't keep `grepConfirm` for the choco probe — `choco list --local-only <pkg>` exits 0 iff installed; use `exit0Confirm`. Keeping grepConfirm would be a weaker check and would make the test's canned stdout matter (it shouldn't).
- ❌ Don't leave lines 265/283 claiming choco "lists everything and we grep" — choco is now exit-0; the narrative must match exit0Confirm or it misleads the next reader.
- ❌ Don't add "choco" to the windows banned list (line 335) — choco is a WINDOWS probe and must be allowed to run on GOOS=windows. Only linux/darwin ban it.
- ❌ Don't use backslashes in the pathHeuristic prefix — the matching code normalizes to '/', and forward slashes is the cleaner OS-agnostic form (matches the Scoop convention's intent). Either works post-normalize, but forward slashes avoids confusion.
- ❌ Don't edit delegate.go/upgrade.go — the RUN→PRINT delegation move is S2. Those files MAY still reference winget after S1; that's expected and S2's job.
- ❌ Don't touch .goreleaser.yaml/release.yml/install.ps1 (P1.M2) or docs (P1.M3) — S1 is detect.go + detect_test.go ONLY.
- ❌ Don't change any non-Windows channel row — brew/scoop/npm/mise/asdf/aur/deb/rpm/nix/go-install/direct are all unchanged.
- ❌ Don't forget the test's canned response must return EXIT 0 for "choco" (exit0Confirm checks exit code, not stdout) — returning non-zero would make the case fail.

---

## Confidence Score: 9/10

One-pass success is very high: a mechanical rename + one table-row swap + one table-row add + comment/test
edits, with every line number verified, the exit0Confirm/grepConfirm distinction (the one logic change)
explained with the test mechanics, and the forward-slash pathHeuristic convention confirmed against the
normalization code. The -1 is for the two narrative comments (lines 265/283) that group winget with the
grep-confirm channels — an implementer doing a pure find-replace of "winget"→"choco" would leave a
semantically-wrong comment ("choco lists everything and we grep") that contradicts the new exit0Confirm.
Mitigated by calling out both narratives explicitly in Task 4 + the anti-pattern, and the grep gate
catches stragglers but NOT semantic-comment drift (hence the explicit Task 4 rewording).