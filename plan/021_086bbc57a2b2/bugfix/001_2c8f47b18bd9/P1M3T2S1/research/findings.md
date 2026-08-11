# Research Findings — P1.M3.T2.S1: Add Linuxbrew Cellar root to pathHeuristics (BUG-007)

## 0. The bug — one missing table row

`internal/upgrade/detect.go` `pathHeuristics` (the FR-U2 tier-(c) install-root → channel table) maps the
three Homebrew Cellar roots… but only TWO of them:
```go
var pathHeuristics = []pathHeuristic{
	{prefix: "/opt/homebrew/Cellar/", channel: ChannelBrew},   // Apple Silicon macOS
	{prefix: "/usr/local/Cellar/", channel: ChannelBrew},       // Intel macOS
	{prefix: `\scoop\shims\`, channel: ChannelScoop},
	{prefix: "/nix/store/", channel: ChannelNix},
	{prefix: "ProgramData/chocolatey/", channel: ChannelChocolatey},
}
```
Missing: `/home/linuxbrew/.linuxbrew/Cellar/` (Linuxbrew on Linux). A Linuxbrew install of stagecoach
resolves to that prefix, hits NO row, and falls through to `ChannelDirect` — so `stagecoach upgrade`
SELF-SWAPS a brew-managed binary instead of delegating to `brew upgrade stagecoach` (FR-U1/U3). Exactly
the fighting-a-package-manager failure the delegate-first design exists to prevent.

## 1. The fix — one row, adjacent to the other Cellar entries

Add the row right after `/usr/local/Cellar/` (keep the three brew roots together for readability):
```go
var pathHeuristics = []pathHeuristic{
	{prefix: "/opt/homebrew/Cellar/", channel: ChannelBrew},             // Apple Silicon macOS
	{prefix: "/usr/local/Cellar/", channel: ChannelBrew},                // Intel macOS
	{prefix: "/home/linuxbrew/.linuxbrew/Cellar/", channel: ChannelBrew}, // Linuxbrew on Linux (BUG-007)
	{prefix: `\scoop\shims\`, channel: ChannelScoop},
	{prefix: "/nix/store/", channel: ChannelNix},
	{prefix: "ProgramData/chocolatey/", channel: ChannelChocolatey},
}
```
(The inline `// Apple Silicon` / `// Intel macOS` / `// Linuxbrew` annotations are the contract's
"optionally add a code comment noting the three Homebrew Cellar roots" — they aid the next reader and
mirror the §BUG-007 doc's annotated example.)

## 2. Why the matching logic needs NO change — detectPath is GOOS-agnostic + cross-platform

`detectPath` (detect.go:378) matches a prefix via `strings.Contains` AFTER normalizing BOTH sides to `/`
and lowercasing (detect.go:401-404):
```go
matchPath := strings.ReplaceAll(lower, "\\", "/")
for _, h := range pathHeuristics {
	prefix := strings.ReplaceAll(strings.ToLower(h.prefix), "\\", "/")
	if strings.Contains(matchPath, prefix) {
		return h.channel, "path: " + h.prefix, true
	}
}
```
So `/home/linuxbrew/.linuxbrew/Cellar/stagecoach/1.0/bin/stagecoach` (the realpath of a Linuxbrew install)
Contains `/home/linuxbrew/.linuxbrew/Cellar/` → returns `ChannelBrew`. The match is GOOS-INDEPENDENT (the
tier-(c) heuristic is cross-GOOS deterministic by design — a darwin/linux ExePath resolves identically on
any test host). **No change to detectPath itself.** The table row is the entire fix.

## 3. The struct + constant (verified)

- `type pathHeuristic struct { prefix string; channel Channel }` (detect.go:356).
- `ChannelBrew Channel = "brew"` (detect.go:39). The new row reuses it — no new constant.

## 4. The regression test — mirror TestDetect_Path_NixStore (dedicated, GOOS=linux)

The existing Cellar test `TestDetect_Path_BrewCellar` (detect_test.go:374) loops over the two macOS
Cellar roots with `GOOS: "darwin"`. Adding the Linuxbrew path THERE would be semantically odd (Linuxbrew
is a Linux install) and would require restructuring that loop's hardcoded GOOS. The cleaner, established
pattern is a DEDICATED test mirroring `TestDetect_Path_NixStore` (detect_test.go:388, a one-assertion
test with `GOOS: "linux"`):
```go
func TestDetect_Path_LinuxbrewCellar(t *testing.T) {
	// BUG-007: the Linuxbrew Cellar root (/home/linuxbrew/.linuxbrew/Cellar/) must detect as brew,
	// not fall through to direct. detectPath is GOOS-agnostic (cross-GOOS deterministic).
	d := &Detector{ExePath: "/home/linuxbrew/.linuxbrew/Cellar/stagecoach/1.0/bin/stagecoach", GOOS: "linux"}
	ch, ev, ok := d.detectPath()
	if !ok || ch != ChannelBrew {
		t.Errorf("detectPath linuxbrew = %q,%q,%v, want brew,true", ch, ev, ok)
	}
}
```
(Acceptable alternative: append the Linuxbrew path to the existing TestDetect_Path_BrewCellar loop — it
matches under any GOOS — but the dedicated test is clearer and matches the per-heuristic test convention.)
This test FAILS on the current code (detectPath returns `ChannelDirect, "", false` → the `!ok` branch
fires) and PASSES after the row is added — a true regression guard.

## 5. No conflict with the parallel P1.M3.T1.S1 (BUG-004)

The parallel item (read its PRP) fixes BUG-004: the production `cmdRunner.Run`
(internal/cmd/upgrade_run.go:152) lacks osRunner's 3s timeout + WaitDelay. Its detect.go change is
EXPORTING the `defaultQueryTimeout` constant → `DefaultQueryTimeout` (3 spots, around line ~121). My edit
is the `pathHeuristics` table (~line 370). **Different regions of the same file** — no textual overlap.
Both are surgical; neither touches the other's code. (Do NOT touch `defaultQueryTimeout`/`DefaultQueryTimeout`
— that's P1.M3.T1.S1's.) If both land in the same merge, there is no conflict (distinct line ranges).

## 6. Scope boundaries

- **BUG-008** (planned, P1.M3.T3.S1) — escape `c.Repo` in releases.go URLs. Different file
  (internal/upgrade/releases.go). Zero overlap.
- **P1.M3.T4.S1** (planned) — docs sync (docs/packaging.md + README). This item: Mode A (code comment
  only, no user-facing doc surface change per the contract). The pathHeuristics godoc already cites
  `/opt/homebrew/Cellar/` as the Unix-root example; optionally broaden the example wording, but no README.
- This item touches ONLY: `internal/upgrade/detect.go` (1 table row + inline comments) +
  `internal/upgrade/detect_test.go` (1 dedicated test). NO edit to detectPath, releases.go, delegate.go,
  go.mod, or any PRD/task file.

## 7. Validation commands (verified)

```bash
go build ./...                          # the row compiles
go vet ./internal/upgrade/...
gofmt -l internal/upgrade/detect.go internal/upgrade/detect_test.go   # empty
go test ./internal/upgrade/ -run 'TestDetect_Path_LinuxbrewCellar' -v  # the new test GREEN
go test ./internal/upgrade/ -race        # full upgrade regression (all detect/download/resolve/... tests)
make test ; make lint
git status --porcelain                   # ONLY detect.go + detect_test.go
```

`internal/upgrade` is NOT in the coverage-gate list (Makefile gates `internal/{git,provider,generate,
config}` only) — no coverage-threshold pressure.

## 8. The user-visible effect (FR-U1/U3)

Before: `stagecoach upgrade` on a Linuxbrew install → tier-(c) misses → `ChannelDirect` → the
self-swap path runs → overwrites the brew-managed binary (reverted on `brew upgrade`, corrupting brew's
bookkeeping — the exact v2.1-rejection failure mode). After: tier-(c) matches → `ChannelBrew` → the
dispatcher delegates to `brew upgrade stagecoach` (FR-U3 brew row). One row closes the gap.