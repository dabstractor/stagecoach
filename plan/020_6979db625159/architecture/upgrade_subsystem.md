# Phase 1 — Upgrade Subsystem Findings (detect.go + delegate.go)

## File: `internal/upgrade/detect.go` (444 lines)

### Channel constant block (lines 39-50)
```go
const (
	ChannelBrew      Channel = "brew"
	ChannelScoop     Channel = "scoop"
	ChannelWinget    Channel = "winget"        // ← RENAME to ChannelChocolatey = "chocolatey"
	ChannelAUR       Channel = "aur"
	ChannelNpm       Channel = "npm"
	ChannelMise      Channel = "mise"
	ChannelAsdf      Channel = "asdf"
	ChannelNix       Channel = "nix"
	ChannelGoInstall Channel = "go-install"
	ChannelDeb       Channel = "deb"
	ChannelRpm       Channel = "rpm"
	ChannelDirect    Channel = "direct"
)
```

### validChannel() — lines 62-68
```go
func validChannel(s string) bool {
	switch Channel(s) {
	case ChannelBrew, ChannelScoop, ChannelWinget, ChannelAUR, ChannelNpm,
		ChannelMise, ChannelAsdf, ChannelNix, ChannelGoInstall, ChannelDeb, ChannelRpm, ChannelDirect:
		return true
	}
	return false
}
```
**Change:** `ChannelWinget` → `ChannelChocolatey`

### knownChannelList() — lines 240-245 (the "allowed-values" the PRD references)
```go
func knownChannelList() string {
	return strings.Join([]string{
		string(ChannelBrew), string(ChannelScoop), string(ChannelWinget), string(ChannelAUR),
		string(ChannelNpm), string(ChannelMise), string(ChannelAsdf), string(ChannelNix),
		string(ChannelGoInstall), string(ChannelDeb), string(ChannelRpm), string(ChannelDirect),
	}, ", ")
}
```
**Change:** `string(ChannelWinget)` → `string(ChannelChocolatey)`

### pmProbes table — lines 267-276
```go
var pmProbes = []pmProbe{
	{channel: ChannelBrew,   goos: []string{"darwin","linux"}, name: "brew",   args: []string{"list","stagecoach"},       confirm: exit0Confirm},
	{channel: ChannelAUR,    goos: []string{"linux"},          name: "pacman", args: []string{"-Q","stagecoach-bin"},    confirm: exit0Confirm},
	{channel: ChannelDeb,    goos: []string{"linux"},          name: "dpkg",   args: []string{"-s","stagecoach"},        confirm: exit0Confirm},
	{channel: ChannelRpm,    goos: []string{"linux"},          name: "rpm",    args: []string{"-q","stagecoach"},        confirm: exit0Confirm},
	{channel: ChannelScoop,  goos: []string{"windows"},        name: "scoop",  args: []string{"prefix","stagecoach"},    confirm: exit0Confirm},
	{channel: ChannelWinget, goos: []string{"windows"},        name: "winget", args: []string{"list"},                  confirm: grepConfirm("stagecoach")},
	{channel: ChannelNpm,    goos: nil,                        name: "npm",    args: []string{"ls","-g","--depth=0"},   confirm: grepConfirm("stagecoach")},
	{channel: ChannelMise,   goos: nil,                        name: "mise",   args: []string{"ls"},                     confirm: grepConfirm("stagecoach")},
	{channel: ChannelAsdf,   goos: nil,                        name: "asdf",   args: []string{"list"},                   confirm: grepConfirm("stagecoach")},
}
```
**Change:** Replace the winget row with:
```go
{channel: ChannelChocolatey, goos: []string{"windows"}, name: "choco", args: []string{"list","--local-only","stagecoach"}, confirm: exit0Confirm},
```
**Key:** Uses `exit0Confirm` (choco exits 0 iff installed), NOT `grepConfirm`.

### pathHeuristics table — lines 350-353
```go
var pathHeuristics = []pathHeuristic{
	{prefix: "/opt/homebrew/Cellar/", channel: ChannelBrew},
	{prefix: "/usr/local/Cellar/", channel: ChannelBrew},
	{prefix: `\scoop\shims\`, channel: ChannelScoop},
	{prefix: "/nix/store/", channel: ChannelNix},
}
```
**Add:** `{prefix: "ProgramData/chocolatey/", channel: ChannelChocolatey}`
**Note:** Use forward slashes for cross-GOOS normalization (same convention as the matching code at detect.go:385-392 that normalizes both sides to `/`).

### Narrative comments mentioning winget
Lines: 7, 87, 150, 265, 283, 311 — all need "winget" → "chocolatey" or reworded.

---

## File: `internal/upgrade/delegate.go` (384 lines)

### Delegate() dispatch switch — line 205+
```go
case ChannelDirect:
    // ... self-swap sentinel ...
case ChannelAUR, ChannelNix, ChannelDeb, ChannelRpm:    // ← ADD ChannelChocolatey
    // PRINT channels
    primary, full := printCommand(ch)
    fmt.Fprintln(opts.out(), full)
    return DelegateResult{Ran: false, Command: primary, ExitCode: 0}, nil
default:
    // RUN channels: brew/scoop/winget/npm/mise/asdf/go-install  ← REMOVE winget from comment
```

### runArgv() — line 259+ (REMOVE winget case)
```go
case ChannelWinget:                                      // ← DELETE lines 273-274
    return [][]string{{"winget", "upgrade", "stagecoach"}}
```

### printCommand() — line 342+ (ADD Chocolatey case)
After the `case ChannelRpm:` arm (line 367-376), add:
```go
case ChannelChocolatey:
    primary = "choco upgrade stagecoach -y"
    full = "choco upgrade stagecoach -y\n# (choco owns the binary; admin needed — FR-U1/FR-U4)"
    return primary, full
```
Style mirrors the AUR/.deb/.rpm comments.

### Narrative comments mentioning winget
Lines: 4, 74, 195, 221 — update to name Chocolatey in PRINT set, drop winget from RUN list.

---

## File: `internal/upgrade/detect_test.go` (457 lines)

### "winget installed (windows)" case — lines 220-229
Convert to `"chocolatey installed (windows)"`:
```go
{
    name: "chocolatey installed (windows)", goos: "windows",
    canned: func(n string, _ []string) (string, int, error) {
        if n == "choco" {
            return "stagecoach 1.0.0", 0, nil  // exit0Confirm checks exit code only
        }
        return "", 1, nil
    },
    want: ChannelChocolatey,
},
```
**Key difference:** choco uses `exit0Confirm`, so the canned response exits 0 (not a grep match).

### GOOS-gating assertions — lines 307-322
- Line 311: `if r.called("winget")` → `if r.called("choco")`
- Line 312: error message text
- Line 322: banned list `"winget"` → `"choco"`
- Lines 13, 27: comment mentions of winget

---

## File: `internal/upgrade/delegate_test.go` (409 lines)

### RUN-table winget row — line 62
DELETE the winget row from the runArgv table test:
```go
{"winget", ChannelWinget, [][]string{{"winget", "upgrade", "stagecoach"}}},  // ← REMOVE
```

### Channel-set assertion — line 338
Remove ChannelWinget from the runChannels slice:
```go
runChannels := []Channel{
    ChannelBrew, ChannelScoop, ChannelNpm, ChannelMise, ChannelAsdf, ChannelGoInstall,  // ← was 7, now 6
}
```

### ADD new printCommand test for Chocolatey
Add a test asserting ChannelChocolatey is a PRINT channel:
- `Delegate(ctx, ChannelChocolatey, opts)` → `Ran: false`, `ExitCode: 0`, `Command: "choco upgrade stagecoach -y"`
- Output contains the full command with the FR-U1/FR-U4 comment.

---

## Files: `internal/cmd/upgrade.go` and `internal/cmd/upgrade_run.go`

### upgrade.go
- Line 4: `// (Homebrew/Scoop/winget/npm/...` → `chocolatey`
- Line 79: `(Homebrew, Scoop, winget, npm, ...` → `chocolatey`

### upgrade_run.go
- Line 266: `// runDelegate ... (every non-direct channel: brew/scoop/winget/...` → `chocolatey`