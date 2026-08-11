# Upgrade Subsystem — Architecture Research (BUG-004, BUG-007, BUG-008)

## Scope
Three bugs in the `stagecoach upgrade` install-method detection + GitHub releases path:
- **BUG-004 (Major)**: Production detection runner (`cmdRunner.Run`) lacks per-query timeout + WaitDelay.
- **BUG-007 (Minor)**: pathHeuristics omits the Linuxbrew Cellar root.
- **BUG-008 (Minor)**: releases.go interpolates `c.Repo` unescaped in request paths.

All claims below are verified against the actual source at commit time (2025-08-11).

---

## BUG-004: cmdRunner.Run has no per-query timeout or WaitDelay

### Verified Code

**File**: `internal/cmd/upgrade_run.go`

**`cmdRunner` struct** (line 143):
```go
type cmdRunner struct{}
```

**`cmdRunner.Run`** (line 152):
```go
func (cmdRunner) Run(ctx context.Context, name string, args ...string) (string, int, error) {
	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return buf.String(), 0, cerr
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return buf.String(), ee.ExitCode(), nil
		}
		return "", 0, err
	}
	return buf.String(), 0, nil
}
```

**Key observations**:
- Uses `exec.CommandContext(ctx, name, args...)` directly — NO `context.WithTimeout` wrapper.
- NO `cmd.WaitDelay` field is set.
- The source comment (line 141) explicitly acknowledges this: "There is NO per-query timeout here (unlike osRunner's 3s) because the command's ctx already bounds the whole upgrade; a hung PM surfaces as ctx.Err()."
- grep confirms: **zero** matches for `WaitDelay` or `WithTimeout` in `upgrade_run.go`.

**Contrast — `osRunner.Run`** in `internal/upgrade/detect.go` (line 121):
```go
func (r *osRunner) Run(ctx context.Context, name string, args ...string) (string, int, error) {
	timeout := r.timeout
	if timeout == 0 {
		timeout = defaultQueryTimeout // 3 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)   // line 126 — HAS per-query timeout
	defer cancel()

	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &out
	cmd.WaitDelay = timeout                              // line 140 — HAS WaitDelay
	// ... rest mirrors cmdRunner ...
}
```

**`osRunner` is unexported** (`type osRunner struct` at line 105) — package `cmd` cannot reference it.

**Root context** — `cmd/stagecoach/main.go` line 62:
```go
ctx, _ := signal.Install(context.Background(), signal.Options{...})
```
`context.Background()` has **no deadline** — only signal-cancelable (Ctrl-C). So the "ctx already bounds the whole upgrade" rationale is false: there is no deadline, only cancellation.

### Why osRunner Cannot Be Used from Package cmd
`osRunner` is lowercase-unexported in package `upgrade`. Package `cmd` imports `upgrade` but cannot construct `&upgrade.osRunner{}`. This is why `cmdRunner` exists as an "identical-contract twin" (per the file's extensive doc comment at lines 136-141).

### How cmdRunner Is Wired
`prodDetect` (line 176) builds the production `Detector`:
```go
d := &upgrade.Detector{
	Exec:     cmdRunner{},   // NON-NIL (nil skips PM probes — detect.go). osRunner is unexported.
	ExePath:  exe,
	GOOS:     runtime.GOOS,
	Override: override,
	Env:      os.Getenv,
	Log:      log,
}
```
The `Detector.Exec` field is of type `upgrade.Runner` (the interface). The tier-(b) PM probes call `d.Exec.Run(ctx, p.name, p.args...)` (detect.go:332).

### The 9 PM Probes (pmProbes table, detect.go:267-277)
```
brew list stagecoach        (darwin, linux)
pacman -Q stagecoach-bin    (linux)
dpkg -s stagecoach          (linux)
rpm -q stagecoach           (linux)
scoop prefix stagecoach     (windows)
choco list --local-only ...  (windows)
npm ls -g --depth=0          (all)
mise ls                      (all)
asdf list                    (all)
```
Any of these can hang (brew DB refresh, broken PM, NFS home). Without a per-query timeout, each hangs indefinitely.

### Fix Options (from PRD Recommendations)
1. **Export `osRunner`** as `OsRunner` (or add a constructor `upgrade.NewOsRunner()`) so `prodDetect` can use it instead of `cmdRunner`. Minimal change to `detect.go` (rename type + method receiver), then change `upgrade_run.go` line 183 to `Exec: upgrade.NewOsRunner()` (or similar). The `cmdRunner` twin becomes dead code that can be deleted.
2. **Replicate** the 3s `context.WithTimeout` + `cmd.WaitDelay` in `cmdRunner.Run` directly. Keeps `osRunner` unexported; duplicates the logic. More lines but zero change to package `upgrade`.

**Recommendation**: Option 1 (export) is cleaner — eliminates the duplicate twin. Option 2 (replicate) is lower-risk for the test wall (no rename ripple). Both are correct.

### Test Patterns
- `internal/upgrade/detect_test.go`: `TestOsRunner_PerQueryTimeout` (line ~380) tests `osRunner` directly with a 200ms timeout and `sleep 5`, asserting `err != nil` and `elapsed < 2s`. This is the exact regression test pattern for BUG-004.
- `internal/upgrade/detect_test.go`: `TestOsRunner_NonZeroExitIsNotError`, `TestOsRunner_AbsentBinary_StartError`, `TestOsRunner_ExitZero` test the Runner contract via `osRunner` directly.
- `internal/cmd/upgrade_test.go`: Tests focus on flag validation and the no-bootstrap guarantee. There is NO test exercising `cmdRunner.Run` directly (it's only reached through `prodDetect` which tests override wholesale).

**Regression test for BUG-004**: Either replicate `TestOsRunner_PerQueryTimeout` as a `cmdRunner` test in `internal/cmd/`, or — if `osRunner` is exported — the existing test already covers the shared code path.

---

## BUG-007: pathHeuristics omits Linuxbrew Cellar root

### Verified Code

**File**: `internal/upgrade/detect.go`, `pathHeuristics` table (lines 367-374):
```go
var pathHeuristics = []pathHeuristic{
	{prefix: "/opt/homebrew/Cellar/", channel: ChannelBrew},       // Apple Silicon
	{prefix: "/usr/local/Cellar/", channel: ChannelBrew},          // Intel macOS
	{prefix: `\scoop\shims\`, channel: ChannelScoop},              // Windows
	{prefix: "/nix/store/", channel: ChannelNix},                  // Nix
	{prefix: "ProgramData/chocolatey/", channel: ChannelChocolatey}, // Windows
}
```

**Missing**: `/home/linuxbrew/.linuxbrew/Cellar/` — the Linuxbrew (Homebrew on Linux) Cellar root.

**Matching logic** (detect.go:397-407 in `detectPath`):
```go
matchPath := strings.ReplaceAll(lower, "\\", "/")
for _, h := range pathHeuristics {
	prefix := strings.ReplaceAll(strings.ToLower(h.prefix), "\\", "/")
	if strings.Contains(matchPath, prefix) {
		return h.channel, "path: " + h.prefix, true
	}
}
```
The match is separator-agnostic (`Contains` after normalizing backslashes to forward slashes) and case-insensitive (`lower`). So adding the Linuxbrew entry with forward slashes will match correctly on all platforms.

### Fix
Add one entry to `pathHeuristics`:
```go
{prefix: "/home/linuxbrew/.linuxbrew/Cellar/", channel: ChannelBrew},
```
Insert it next to the other two `Cellar` entries (lines 369-370).

### Test Pattern
`TestDetect_Path_BrewCellar` (detect_test.go ~line 270) loops over both existing prefixes:
```go
for _, p := range []string{
	"/opt/homebrew/Cellar/stagecoach/1.0/bin/stagecoach",
	"/usr/local/Cellar/stagecoach/1.0/bin/stagecoach",
} {
	d := &Detector{ExePath: p, GOOS: "darwin"}
	ch, ev, ok := d.detectPath()
	if !ok || ch != ChannelBrew { ... }
}
```
**Regression test**: Add `"/home/linuxbrew/.linuxbrew/Cellar/stagecoach/1.0/bin/stagecoach"` to that slice (GOOS should be `"linux"` for the Linuxbrew case, or the loop should be split to test each prefix with its native GOOS).

---

## BUG-008: c.Repo unescaped in releases.go request paths

### Verified Code

**File**: `internal/upgrade/releases.go`

Three call sites where `c.Repo` is interpolated raw into the request path:

1. **Line 179** — `LatestStable`:
```go
body, err := c.do(ctx, "/repos/"+c.Repo+"/releases/latest")
```

2. **Line 198** — `ReleaseByTag` (tag IS escaped, repo is NOT):
```go
path := "/repos/" + c.Repo + "/releases/tags/" + url.PathEscape(tag)
```

3. **Line 223** — `LatestAdmittingPrereleases`:
```go
body, err := c.do(ctx, "/repos/"+c.Repo+"/releases")
```

Additionally, line 249 uses `c.Repo` in an error format string (not a URL, so no escaping needed):
```go
return Release{}, fmt.Errorf("/repos/%s/releases: %w", c.Repo, ErrNoReleases)
```

**`url.PathEscape` is already imported** (line 10: `"net/url"`) and used at line 198 for the tag. So the fix adds zero imports.

### Fix
Apply `url.PathEscape(c.Repo)` at lines 179, 198, and 223:
```go
// Line 179:
body, err := c.do(ctx, "/repos/"+url.PathEscape(c.Repo)+"/releases/latest")
// Line 198:
path := "/repos/" + url.PathEscape(c.Repo) + "/releases/tags/" + url.PathEscape(tag)
// Line 223:
body, err := c.do(ctx, "/repos/"+url.PathEscape(c.Repo)+"/releases")
```

### Severity Assessment
Low / defense-in-depth. `c.Repo` is sourced from config (`config.SourceRepo`), which is normally `"dabstractor/stagecoach"` — no characters requiring escaping. But GitHub `owner/repo` names technically allow some characters, and the inconsistency (tag escaped, repo not) is a latent bug.

### Test Pattern
`internal/upgrade/releases_test.go` uses an `httptest.Server`:
```go
return &Client{BaseURL: ts.URL, Repo: "owner/repo"}
```
The tests assert behavior (not exact URL paths). A regression test would use a repo value with a special character and assert no malformed request / correct escaping. Since the httptest server receives the raw path, the test can capture `r.URL.Path` and assert it matches the escaped form.

---

## Summary Table

| Bug | File | Line(s) | Severity | Fix Size | Test Pattern Exists |
|-----|------|---------|----------|----------|-------------------|
| BUG-004 | internal/cmd/upgrade_run.go | 152 | Major | ~5 lines | Yes — `TestOsRunner_PerQueryTimeout` in detect_test.go |
| BUG-007 | internal/upgrade/detect.go | 369-373 | Minor | 1 line | Yes — `TestDetect_Path_BrewCellar` |
| BUG-008 | internal/upgrade/releases.go | 179, 198, 223 | Minor | 3 edits | Partial — httptest pattern in releases_test.go |

## Architectural Notes for Implementers

1. **BUG-004 fix choice** (export vs replicate): The `osRunner` type and `defaultQueryTimeout` constant are both unexported in package `upgrade`. Exporting requires renaming `osRunner` → `OsRunner` (or adding a `NewOsRunner() Runner` constructor). The `Detector.Exec` field already accepts the `Runner` interface, so either approach plugs in seamlessly. If exporting, check that `osRunner` is not referenced by any test as a type literal (it IS — `detect_test.go` uses `&osRunner{timeout: 0}` directly, so a rename touches those tests too, or a constructor avoids that).

2. **BUG-007**: The Linuxbrew prefix is the standard install location documented by Homebrew on Linux (`/home/linuxbrew/.linuxbrew/`). The `Contains`-based matching means the entry just needs to be in the table — the existing separator-normalization handles the rest.

3. **BUG-008 — CRITICAL NUANCE (verified by running `url.PathEscape`)**: The Go stdlib doc says `url.PathEscape` "replaces special characters (including /) with %XX sequences". We ran the actual code: `url.PathEscape("owner/repo")` returns `"owner%2Frepo"`. GitHub's REST API expects literal `/repos/{owner}/{repo}/...` where the `/` between owner and repo is a real path separator. Applying `url.PathEscape(c.Repo)` to the combined `"owner/repo"` string would produce `/repos/owner%2Frepo/releases/latest` — **a 404** (GitHub treats `%2F` as an encoded slash within a single segment, not a separator). **The PRD's literal recommendation (`url.PathEscape(c.Repo)`) is therefore WRONG for a combined owner/repo string.** The correct fix is one of:
   - (a) Split `c.Repo` on `/`, `url.PathEscape` each segment, rejoin with `/`.
   - (b) Accept that valid GitHub owner/repo names are restricted to `[a-zA-Z0-9._-]` (none require escaping), so the only practical fix is upstream validation of the Repo field, and document the inconsistency as defense-in-depth documentation rather than a code change.
   - (c) If the repo value is trusted/validated upstream (config layer), BUG-008 may be a no-op in practice and could be downgraded to documentation-only.
   **The implementer MUST resolve this before applying a naive `url.PathEscape(c.Repo)`.**