# Research Notes — P1.M1.T1.S1 (detect.go: Winget→Chocolatey rename + pmProbe swap + path heuristic + tests)

Verification against the CURRENT working tree. The task description + `architecture/upgrade_subsystem.md`
are accurate (line-for-line). These notes record the verified anchors + the exit0Confirm/grepConfirm
distinction (the one substantive logic change).

## VERIFIED — detect.go (444 lines), all winget references
| Line | Content | Change |
|------|---------|--------|
| 7 | narrative "brew/scoop/winget/pacman/npm/mise/asdf" | winget→chocolatey |
| 41 | `ChannelWinget Channel = "winget"` | → `ChannelChocolatey Channel = "chocolatey"` + FR-U2/U3 PRINT doc |
| 64 | validChannel switch lists `ChannelWinget` | → `ChannelChocolatey` |
| 87 | narrative "brew/scoop/winget" | winget→chocolatey |
| 150 | narrative "winget⇒windows" | winget→chocolatey⇒windows |
| 242 | knownChannelList `string(ChannelWinget)` | → `string(ChannelChocolatey)` |
| 265 | narrative "winget/npm/mise/asdf list everything... grep" | choco moved OUT of the grep group (it's now exit0) |
| 272 | pmProbes winget row: `{channel: ChannelWinget, goos: ["windows"], name: "winget", args: ["list"], confirm: grepConfirm("stagecoach")}` | REPLACE (see below) |
| 283 | narrative "winget/npm/mise/asdf list every package" | choco no longer "lists every package" (exit0 now) |
| 311 | GOOS-gating comment "winget/scoop are Windows-only" | winget→choco |

## VERIFIED — the pmProbes swap (line 272) — the ONE substantive logic change
BEFORE (winget): `{channel: ChannelWinget, goos: []string{"windows"}, name: "winget", args: []string{"list"}, confirm: grepConfirm("stagecoach")}`
- `grepConfirm("stagecoach")` checks STDOUT contains "stagecoach" (winget list dumps everything).

AFTER (choco): `{channel: ChannelChocolatey, goos: []string{"windows"}, name: "choco", args: []string{"list", "--local-only", "stagecoach"}, confirm: exit0Confirm}`
- `exit0Confirm` checks EXIT CODE == 0 ONLY (line 280: `func exit0Confirm(_ string, exitCode int) bool { return exitCode == 0 }`).
- `choco list --local-only stagecoach` filters by package name + local-only scope; choco exits 0 iff the
  named package is installed locally. This is a STRONGER, more precise check than grep (no false-positive
  on a package whose name merely contains "stagecoach"). The stdout content is now IRRELEVANT to the
  confirm predicate — only exit code matters.

NOTE: line 265's narrative ("winget/npm/mise/asdf list everything and we grep") must be UPDATED — choco
no longer belongs in the "list everything + grep" group. It joins the brew/scoop/pacman exit-0 group.
Line 283's "winget/npm/mise/asdf list every package" likewise.

## VERIFIED — pathHeuristics (line 349-353) + the forward-slash normalization convention
Current table:
```go
var pathHeuristics = []pathHeuristic{
	{prefix: "/opt/homebrew/Cellar/", channel: ChannelBrew},
	{prefix: "/usr/local/Cellar/", channel: ChannelBrew},
	{prefix: `\scoop\shims\`, channel: ChannelScoop},
	{prefix: "/nix/store/", channel: ChannelNix},
}
```
ADD: `{prefix: "ProgramData/chocolatey/", channel: ChannelChocolatey}`
WHY forward slashes: the matching code (detect.go:385-392) normalizes BOTH sides to '/':
```go
matchPath := strings.ReplaceAll(lower, "\\", "/")
for _, h := range pathHeuristics {
	prefix := strings.ReplaceAll(strings.ToLower(h.prefix), "\\", "/")
	if strings.Contains(matchPath, prefix) { ... }
}
```
So a Chocolatey install path `C:\ProgramData\chocolatey\bin\stagecoach.exe` → Clean'd → lowercased →
backslashes→'/' → `c:/programdata/chocolatey/bin/stagecoach.exe` CONTAINS `programdata/chocolatey/`.
The prefix `ProgramData/chocolatey/` (forward slashes) matches after the same normalization. This is
EXACTLY the Scoop convention (`\scoop\shims\` also normalizes to `/scoop/shims/`). Use forward slashes
to match the existing Chocolatey-real-path form most directly. (Either separator works post-normalize,
but forward slashes is the cleaner, OS-agnostic form.)

## VERIFIED — detect_test.go winget references
| Line | Content | Change |
|------|---------|--------|
| 13 | comment "brew/scoop/winget/pacman/npm/mise/asdf" | winget→chocolatey |
| 27 | comment `called("winget")` | winget→choco |
| 47 | TestValidChannel slice `ChannelWinget` | → `ChannelChocolatey` |
| 221 | `name: "winget installed (windows)"` | → "chocolatey installed (windows)" |
| 223 | `if n == "winget" {` | → `if n == "choco" {` |
| 224 | `return "stagecoach 1.0.0", 0, nil` | return `("", 0, nil)` — exit0Confirm checks exit ONLY; stdout irrelevant (but exit 0 required) |
| 228 | `want: ChannelWinget` | → `want: ChannelChocolatey` |
| 311 | `if r.called("winget") {` | → `r.called("choco")` |
| 312 | error msg "winget probe" | → "choco probe" |
| 322 | banned `{"winget", "scoop", "pacman", "dpkg", "rpm"}` (darwin) | → `{"choco", "scoop", ...}` |
| 335 | banned `{"brew", "pacman", "dpkg", "rpm"}` (windows) | UNCHANGED — choco is NOT banned on windows (it runs there) |

## VERIFIED — the fakeRunner.canned contract (the test mechanics)
`fakeRunner.Run` returns `f.canned(name, args)` → `(string stdout, int exitCode, error)`.
- For the choco case: canned returns `(_, 0, nil)` when `n == "choco"` (exit 0 ⇒ exit0Confirm true).
  The stdout string is irrelevant (exit0Confirm ignores it). `return "", 0, nil` is cleanest.
- For all OTHER names in that case: `return "", 1, nil` (exit 1 ⇒ not installed ⇒ other probes skip).

## VERIFIED — no new imports / no new types
detect.go already imports strings (used by knownChannelList, the narrative, etc.). The changes are:
a const rename, a table-row replacement, a table-row ADDITION, switch/list identifier renames, and
comment edits. No new types, no new imports. The pathHeuristic struct already exists.

## SCOPE BOUNDARIES (sibling subtasks — do NOT implement here)
- **P1.M1.T1.S2**: delegate.go — move Chocolatey from the RUN table to the PRINT table (FR-U4: choco
  needs admin ⇒ PRINT not RUN; FR-U1: choco owns the binary ⇒ no self-swap). Add a printCommand case,
  remove any runArgv case. + upgrade.go comment updates. DIFFERENT file.
- **P1.M2.*** : release plumbing (.goreleaser.yaml chocolateys pipe, delete winget CI job, install.ps1).
- **P1.M3.*** : docs/packaging.md, docs/cli.md, README.md sync.
- Do NOT: edit delegate.go/upgrade.go (S2); edit .goreleaser.yaml/release.yml/install.ps1 (P1.M2); edit
  docs (P1.M3); change any non-Windows channel (brew/scoop/npm/mise/asdf/aur/deb/rpm/nix/go-install/direct
  are all UNCHANGED); or alter the Channel type itself. S1 = detect.go + detect_test.go ONLY.

## THE GREP GATE
After the change: `rg -n winget internal/upgrade/detect.go internal/upgrade/detect_test.go` MUST return
ZERO hits. This is the explicit completeness gate (no straggler winget reference). Note: other files in
internal/upgrade/ (delegate.go, upgrade.go) MAY still reference winget/ChannelWinget — those are S2's
job; S1's grep gate is scoped to detect.go + detect_test.go.