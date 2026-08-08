# Research Findings — P1.M1.T1.S2 (delegate.go: Chocolatey RUN→PRINT + tests + comments)

## 1. S1 contract + compile coupling

S1 ("Implementing" in parallel) renames `ChannelWinget`→`ChannelChocolatey = "chocolatey"` in detect.go.
S1 has NOT landed yet (detect.go still has ChannelWinget). S2 consumes `ChannelChocolatey` from S1.
**COMPILE COUPLING**: the const lives in detect.go; delegate.go references it (`case ChannelWinget:`
at runArgv :273). After S1 renames the const, delegate.go WILL NOT COMPILE until S2 replaces
ChannelWinget. S1+S2 are tightly coupled — the build is green only after both land. S2 must use
`ChannelChocolatey` (the new name), not ChannelWinget.

## 2. delegate.go — the 4 edit sites (verified; anchor on constructs, lines drift)

**Delegate() switch (~:205-260):**
- `case ChannelDirect:` → ErrDirectSwap
- `case ChannelAUR, ChannelNix, ChannelDeb, ChannelRpm:` → PRINT (printCommand + Fprintln +
  `return DelegateResult{Ran:false, Command:primary, ExitCode:0}, nil`). **S2 ADDS ChannelChocolatey
  to this case list.** The PRINT narrative comment above it (~:210 "AUR needs root, Nix is
  declarative") should name Chocolatey (admin).
- `default:` → RUN (runArgv + exec). The RUN narrative comments (:221 "brew/scoop/winget/...") lose winget.

**runArgv() (~:259-300):** has `case ChannelWinget: return [][]string{{"winget","upgrade","stagecoach"}}`
at :273-274. **S2 DELETES this entire case** (2 lines). Other RUN cases (Brew/Scoop/Mise/GoInstall/
Npm/Asdf) UNCHANGED.

**printCommand() (~:342-384):** per-channel `case` returning (primary, full). AUR/Nix/Deb/Rpm each
have a `full` with a `# (...)` comment. After ChannelRpm, before `default:` (which returns "",""),
**S2 ADDS:**
```go
case ChannelChocolatey:
    primary = "choco upgrade stagecoach -y"
    full = "choco upgrade stagecoach -y\n# (choco owns the binary under ProgramData\\chocolatey; run as admin — FR-U1/FR-U4)"
    return primary, full
```
Mirrors the AUR/.deb/.rpm comment style (explains WHY: choco owns the binary + needs admin).

**Narrative comments naming winget (RUN list):** :4, :74, :195, :221 — drop "winget" from each RUN
channel list. (Chocolatey is no longer RUN; it's PRINT.)

## 3. delegate_test.go — 3 edit sites (verified)

**TestDelegate_RunArgvPerChannel (:54, table):** row at :62
`{"winget", ChannelWinget, [][]string{{"winget", "upgrade", "stagecoach"}}}`. **S2 DELETES this row.**

**TestDelegate_PrintChannels (:142, table):** the PRINT-channel test. Table shape:
`{name, ch, primary, containsAll []string}`. Assertions: Ran=false, ExitCode=0, Command==primary,
Exec NOT called (len(f.calls)==0), Out contains all containsAll substrings. Existing rows: aur/nix/
deb/rpm. **S2 ADDS a chocolatey row** (idiomatic — rides the existing table; satisfies the item's
"new printCommand test"):
```go
{
    name:        "chocolatey",
    ch:          ChannelChocolatey,
    primary:     "choco upgrade stagecoach -y",
    containsAll: []string{"choco upgrade stagecoach -y", "admin", "FR-U1"},
},
```

**TestDelegate_NeverSudo (:336):** `runChannels` slice at :338
`ChannelBrew, ChannelScoop, ChannelWinget, ChannelNpm, ChannelMise, ChannelAsdf, ChannelGoInstall`.
**S2 REMOVES ChannelWinget** → 6 RUN channels (brew/scoop/npm/mise/asdf/go-install). (Chocolatey is
PRINT now; NeverSudo iterates RUN channels only. The choco print string has no sudo anyway, but it
must leave the RUN slice since it's no longer RUN.)

## 4. upgrade.go + upgrade_run.go comments (verified)

- **upgrade.go:4** `// (Homebrew/Scoop/winget/npm/mise/asdf/Nix/AUR/go-install)` → winget→chocolatey
- **upgrade.go:79** `(Homebrew, Scoop, winget, npm, mise, asdf, Nix, AUR, go install)` → winget→chocolatey.
  **This is the cobra `Long` help text — USER-FACING via `stagecoach upgrade --help` (Mode A docs, rides with the work).**
- **upgrade_run.go:266** `// runDelegate implements ... (every non-direct channel: brew/scoop/winget/`
  → winget→chocolatey

## 5. Architecture confirmation

upgrade_subsystem.md §"File: internal/upgrade/delegate.go" specifies every change verbatim:
- :91 ADD ChannelChocolatey to PRINT case
- :97 REMOVE winget from RUN comment
- :100-102 DELETE runArgv winget case
- :106-109 ADD printCommand Chocolatey case (primary `choco upgrade stagecoach -y`)
- :117 narrative comments 4,74,195,221
- delegate_test.go :149-152 delete RUN-table winget row; :155-156 remove from runChannels; :163-165 add printCommand test

## 6. Scope boundaries (do NOT do)
- Do NOT touch detect.go/detect_test.go (S1 owns the const rename + pmProbe + pathHeuristic).
- Do NOT touch .goreleaser.yaml/release.yml/install.ps1 (P1.M2) or docs/packaging.md/docs/cli.md/
  README.md (P1.M3) — EXCEPT upgrade.go:79 Long help IS in-scope (Mode A, rides with the work).
- Do NOT change any non-Windows RUN channel (brew/scoop/npm/mise/asdf/go-install) or the existing
  PRINT channels (AUR/Nix/Deb/Rpm).
- Do NOT add new types/imports.

## 7. Validation

`go test ./internal/upgrade/...` green; `make test`/`make lint`. THE grep gate:
`rg -n winget internal/upgrade/delegate.go internal/upgrade/delegate_test.go internal/cmd/upgrade.go
internal/cmd/upgrade_run.go` → ZERO hits. (Note: detect.go/detect_test.go are S1's grep scope, not S2's.)