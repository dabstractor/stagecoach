# Research: P1.M2.T1.S1 — detectChannel() cascade (FR-U2)

New file `internal/upgrade/detect.go` joining the existing package `upgrade` (version.go, releases.go,
and the in-flight download.go from parallel P1.M1.T3.S2). Implements the FR-U2 install-method detection
cascade: explicit override → package-manager DB queries → path heuristics → default `direct`. All
environment-touching seams (subprocess exec, os.Executable path, GOOS, env getter, verbose logger) are
injectable so CI — which has NONE of brew/scoop/winget/pacman/npm/mise/asdf installed — can exercise
every branch against canned outputs.

All claims verified against the v3.0 PRD (§9.29 FR-U2, FR-U3), architecture/external_deps.md §7, and the
existing upgrade package (releases.go conventions).

---

## 0. THE SPEC — FR-U2 cascade (PRD §9.29 line 595 + external_deps.md §7)

Resolve the channel, **highest-priority signal first**:

| Tier | Signal | Source |
|------|--------|--------|
| (a) | Explicit override — `--install-method <m>` / `STAGECOACH_INSTALL_METHOD` | flag (cmd layer P1.M4) + env (npm wrapper sets it, external_deps §3) |
| (b) | Package-manager DB query — `brew list stagecoach`, `scoop list stagecoach`, `winget list`, `pacman -Q stagecoach-bin`, `npm ls -g --depth=0`, `mise ls`, `asdf list` — first confirming query wins | exec via Runner seam, ~3s timeout each, GOOS-gated |
| (c) | Path heuristics on realpath(os.Executable()) — Homebrew Cellar, Scoop `shims`, npm global `node_modules`, Nix `/nix/store`, `$GOPATH/bin`, AUR `/usr/bin` | filepath.EvalSymlinks + prefix match |
| (d) | default `direct` (the ONLY self-swap-eligible channel, FR-U1/U5) | fallback |

Best-effort; logged at `--verbose`; **ambiguous → `direct`**; emits a hint to pin `--install-method`
if the guess is wrong. **Read-only** — queries never mutate.

## The 10 channels (FR-U2/FR-U3)

brew, scoop, winget, **aur** (pacman/AUR — FR-U3 calls it "AUR"), npm, mise, asdf, nix, go-install,
direct. NOTE: use the identifier `"aur"` (not `"pacman"`) so detect.go stays consistent with the future
`delegate()` dispatcher (P1.M2.T2.S1), which switches on FR-U3's "AUR" row.

---

## 1. PACKAGE CONVENTIONS TO MATCH (verified in releases.go / P1.M1.T3.S2 PRP)

- **Package `upgrade`, stdlib-only** — NO new go.mod require (context, os, os/exec, path/filepath,
  runtime, strings, fmt, time, errors). Walled off: imports NO `internal/*` package (FR-U12).
- **File comment, NOT a package doc.** releases.go owns the package doc (`// releases.go implements…`);
  the parallel download.go PRP explicitly says "download.go gets a file comment only (NO competing
  package doc)". detect.go follows the SAME rule — start with `// detect.go implements…`, not `// Package
  upgrade …`.
- **Sentinel errors** via `var ErrX = errors.New("upgrade: …")`, wrapped with `%w` at use sites
  (errors.Is-reachable). Mirror releases.go's `ErrNoReleases`/`ErrHTTP` style.
- **Injectable-struct pattern** (mirror `Client` in releases.go): a struct with fields the caller/test
  sets, methods on `*Struct`. Detection → a `Detector` struct.
- **Tests in `package upgrade`** (internal test package), table-driven, canned outputs (mirror
  releases_test.go's `newFakeClient`/`cannedLatest` helper style). NO real brew/scoop/etc. in CI.
- **exec.CommandContext + ctx timeout** is the house subprocess pattern (internal/git/git.go,
  internal/provider/executor.go). The Runner seam's production impl wraps it.

---

## 2. THE API DESIGN

### Channel type + constants
```go
type Channel string
const (
    ChannelBrew      Channel = "brew"
    ChannelScoop     Channel = "scoop"
    ChannelWinget    Channel = "winget"
    ChannelAUR       Channel = "aur"        // Arch/AUR (pacman-owned); FR-U3 "AUR"
    ChannelNpm       Channel = "npm"
    ChannelMise      Channel = "mise"
    ChannelAsdf      Channel = "asdf"
    ChannelNix       Channel = "nix"
    ChannelGoInstall Channel = "go-install"
    ChannelDirect    Channel = "direct"     // ONLY self-swap-eligible channel (FR-U1/U5)
)
// validChannel(s) — used to validate the explicit override (tier a).
```
The string values are STABLE identifiers the delegation dispatcher (P1.M2.T2) switches on — do NOT rename.

### Runner — the injectable exec seam (tier b)
```go
type Runner interface {
    Run(ctx context.Context, name string, args ...string) (stdout string, exitCode int, err error)
}
```
- `err != nil` ⇒ start/LookPath failure ⇒ **the PM binary is absent → skip this probe** (NOT a Detect error).
- `exitCode` ⇒ the process exit (0 = success). A non-zero exit (e.g. `brew list` exit 1 = not installed)
  ⇒ skip this PM.
- `stdout` ⇒ parsed by the probe's confirm predicate for ownership.

Production impl `osRunner` wraps `exec.CommandContext` with a **per-query timeout** (default 3s,
external_deps §7 "these queries must not hang"):
```go
type osRunner struct{ timeout time.Duration } // 0 ⇒ 3s default
func (r *osRunner) Run(ctx context.Context, name string, args ...string) (string, int, error) {
    // ctx = context.WithTimeout(ctx, r.timeout or 3s); exec.CommandContext; capture stdout;
    // err = LookPath/start failure (NOT a non-zero exit — map *exec.ExitError to exitCode, nil err).
}
```
**GOTCHA**: a non-zero exit is `*exec.ExitError` — extract its `ExitCode()` and return it with `err==nil`
so the probe logic can treat "not installed" as a skip, not an error. (Mirror executor.go's exit handling.)

### Detector — the injectable resolver
```go
type Detector struct {
    Exec     Runner              // tier (b); nil ⇒ skip PM probes. Production: &osRunner{3*time.Second}.
    ExePath  string              // os.Executable() result; "" ⇒ skip tier (c). Tests inject a fake path.
    GOOS     string              // runtime.GOOS; gates which PM probes run (winget⇒windows, etc.).
    Override string              // --install-method flag value ("" ⇒ unset); cmd layer (P1.M4) passes it.
    Env      func(string) string // os.Getenv; reads STAGECOACH_INSTALL_METHOD. nil ⇒ skip env override.
    Log      func(string)        // --verbose logger (cmd layer wires ui.Verbose); nil ⇒ no-op.
}
func (d *Detector) Detect(ctx context.Context) (Channel, string, error)
```
`Detect` returns `(channel, evidence, err)`. `evidence` is a short human-readable string naming the
confirming tier/signal (e.g. `"--install-method override"`, `"brew list stagecoach (exit 0)"`,
`"path: /opt/homebrew/Cellar/..."`, `"default"`) — surfaced at `--verbose` and in the ambiguous-hint.

---

## 3. THE CASCADE (Detect body)

```go
func (d *Detector) Detect(ctx context.Context) (Channel, string, error) {
    // (a) explicit override — flag beats env; validate it's a known channel (else error, not silent direct)
    if ch, ev, ok := d.detectOverride(); ok { return ch, ev, nil }   // may return err for unknown channel

    // (b) PM DB queries — GOOS-gated table; first confirming probe wins
    if d.Exec != nil {
        if ch, ev, ok := d.detectPackageManager(ctx); ok { return ch, ev, nil }
    }

    // (c) path heuristics on realpath(ExePath)
    if ch, ev, ok := d.detectPath(); ok { return ch, ev, nil }

    // (d) default direct (+ ambiguous hint per FR-U2)
    d.log("install method ambiguous; defaulting to direct (pin --install-method if wrong)")
    return ChannelDirect, "default (ambiguous)", nil
}
```

### Tier (a) — override
```go
// flag first
if d.Override != "" {
    if !validChannel(d.Override) { return ..., fmt.Errorf("upgrade: unknown --install-method %q (want one of ...)", d.Override) }
    return Channel(d.Override), "--install-method override", nil
}
// then env
if d.Env != nil {
    if v := d.Env("STAGECOACH_INSTALL_METHOD"); v != "" {
        if !validChannel(v) { return ..., fmt.Errorf("upgrade: unknown STAGECOACH_INSTALL_METHOD=%q ...", v) }
        return Channel(v), "STAGECOACH_INSTALL_METHOD=" + v, nil
    }
}
```
**GOTCHA**: an invalid override is a hard error (the user explicitly asked for something wrong), NOT a
silent fallback to direct. Only tiers (b)/(c) ambiguity → direct.

### Tier (b) — PM probes (table-driven, GOOS-gated)
```go
type pmProbe struct {
    channel Channel
    goos    []string              // empty ⇒ all GOOS
    name    string
    args    []string
    confirm func(stdout string, exitCode int) bool
}
var pmProbes = []pmProbe{
    {ChannelBrew,   []string{"darwin","linux"}, "brew",  []string{"list","stagecoach"},      brewConfirm},
    {ChannelScoop,  []string{"windows"},        "scoop", []string{"prefix","stagecoach"},    exit0Confirm},
    {ChannelWinget, []string{"windows"},        "winget",[]string{"list"},                   grepConfirm("stagecoach")},
    {ChannelAUR,    []string{"linux"},          "pacman",[]string{"-Q","stagecoach-bin"},    exit0Confirm},
    {ChannelNpm,    nil,                        "npm",   []string{"ls","-g","--depth=0"},    grepConfirm("stagecoach")},
    {ChannelMise,   nil,                        "mise",  []string{"ls"},                     grepConfirm("stagecoach")},
    {ChannelAsdf,   nil,                        "asdf",  []string{"list"},                   grepConfirm("stagecoach")},
}
```
Loop: for each probe whose `goos` admits `d.GOOS` (or is nil), `Run`; if `err != nil` (PM absent) or
`exitCode != 0` (not installed) → log + skip; if `confirm(stdout, exitCode)` → return the channel +
evidence. nix and go-install are NOT in this table (they have no ownership query — detected by path).

`confirm` predicates: `exit0Confirm` (exit 0 ⇒ owned, e.g. brew/scoop/pacman); `grepConfirm(needle)`
(stdout contains needle, e.g. winget/npm/mise/asdf list everything and we grep). `brewConfirm` can be
`exit0Confirm` (brew list <pkg> exits 0 iff installed).

### Tier (c) — path heuristics
```go
func (d *Detector) detectPath() (Channel, string, bool) {
    if d.ExePath == "" { return "", "", false }
    real, err := filepath.EvalSymlinks(d.ExePath)  // resolve realpath (external_deps §7)
    if err != nil { real = d.ExePath }              // tolerate — fall back to the raw path
    real = filepath.Clean(real)
    for _, h := range pathHeuristics { if strings.HasPrefix(real, h.prefix) { return h.channel, "path: " + h.prefix, true } }
    return "", "", false
}
```
Path-roots table (external_deps §7; GOOS-aware via the prefix itself — Unix `/…` vs Windows `…\scoop\shims\`):
```go
var pathHeuristics = []struct{ prefix string; channel Channel }{
    {"/opt/homebrew/Cellar/", ChannelBrew},
    {"/usr/local/Cellar/", ChannelBrew},
    {`\scoop\shims\`, ChannelScoop},          // Windows (case-insensitive? lower real for match)
    {"/nix/store/", ChannelNix},
    {"/usr/bin/", ChannelAUR},                // system-managed (Arch/AUR) — AMBIGUOUS; per FR-U2 default→direct if unconfirmed
    // npm global node_modules + $GOPATH/bin (go-install) — computed from env at Detect time (see below)
}
```
**npm / go-install path detection needs env**: npm global root (`npm root -g` or the conventional
`node_modules/...stagecoach...` path) and `$GOPATH/bin` (go install). These are best resolved by reading
`d.Env("GOPATH")` (default `~/go/bin`) for go-install, and a conventional-prefix match for npm. Keep
this best-effort — if ambiguous, fall through to direct.

**GOTCHA**: `/usr/bin/` is genuinely ambiguous (could be a manual copy, not AUR). external_deps §7 lists
it as an AUR heuristic, but FR-U2 says ambiguous → direct. Decision: include `/usr/bin/` as an AUR hint
but ONLY when a pacman probe (tier b) ALSO confirms — otherwise treat `/usr/bin/` as ambiguous → direct.
(Simplest: leave `/usr/bin/` OUT of the prefix table; let the pacman query (tier b) own AUR detection.
This avoids a false-positive AUR delegation on a manual `/usr/bin` copy. Document this choice.)

---

## 4. TESTS (detect_test.go, package upgrade)

Mirror releases_test.go: `package upgrade`, table-driven, a canned Runner impl. Cases:

| Test | Setup | Assert |
|------|-------|--------|
| override flag wins | Override="npm", Env returns "brew", Exec canned brew-installed | returns ChannelNpm, evidence names override; Exec.Run NOT called |
| override env wins | Override="", Env("STAGECOACH_INSTALL_METHOD")="mise", Exec canned | ChannelMise |
| override invalid → error | Override="snap" | err non-nil, mentions "unknown --install-method" |
| each PM probe confirms | table: per channel, Exec canned to return that PM's "installed" stdout+exit0 | returns that channel |
| PM not-installed → skip | Exec canned exit 1 for brew; nothing else confirms | falls through (direct or path) |
| GOOS gating | d.GOOS="linux"; winget probe must NOT run (assert Exec never called with "winget") | winget skipped |
| path: brew Cellar | ExePath="/opt/homebrew/Cellar/stagecoach/1.0/stagecoach", Exec=nil | ChannelBrew |
| path: nix store | ExePath="/nix/store/abc-stagecoach/bin/stagecoach" | ChannelNix |
| path: go-install | ExePath=$GOPATH-derived, Env("GOPATH") set | ChannelGoInstall |
| default direct | no override, Exec=nil/no-confirms, ExePath="/home/me/bin/stagecoach" | ChannelDirect, evidence "default (ambiguous)" |
| never mutates | (property) all probes are read-only — Run is called with query-only args (no install/upgrade verbs) | grep guards |

Canned Runner:
```go
type fakeRunner struct {
    canned func(name string, args []string) (string, int, error)
    calls  []string // record invocations for GOOS-gating / never-mutates assertions
}
func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (string, int, error) {
    f.calls = append(f.calls, name+" "+strings.Join(args, " "))
    return f.canned(name, args)
}
```

---

## 5. SCOPE FENCES

- **ONLY `internal/upgrade/detect.go` + `internal/upgrade/detect_test.go`.** No edit to releases.go /
  version.go / download.go (parallel P1.M1.T3.S2). No edit to internal/cmd (the upgrade command is P1.M4).
- **NO delegation logic.** detect.go only RESOLVES the channel + evidence. The channel→updater table +
  run-vs-print is `delegate()` in P1.M2.T2.S1. detect.go must not run any updater.
- **NO network.** Detection is local (PM DB queries + path). The GitHub Releases client (releases.go) is
  a different concern (--check / direct-swap resolve). detect.go imports no net/http.
- **NO internal/* imports** (FR-U12 walled-off). Stdlib only.
- **NO new go.mod require.**
- **File comment, not package doc** (releases.go owns the package doc).

---

## 6. PARALLEL-OVERLAP CHECK

Parallel sibling **P1.M1.T3.S2** adds `internal/upgrade/download.go` (SelectAsset/VerifySHA256/
DownloadFile/FetchChecksums/DownloadAndVerifyArchive). It does NOT touch detect.go and shares no symbol
with it (download = GitHub archive fetch; detect = local install-method). Both are additive files in
`package upgrade`. No overlap, no conflict regardless of order. Both follow the same "file comment, not
package doc" rule (releases.go owns the package doc).

---

## 7. Validation (Makefile)

- Build: `go build ./...`
- Vet: `go vet ./internal/upgrade/...`
- Format: `gofmt -l internal/upgrade/detect.go internal/upgrade/detect_test.go` (empty)
- Focused: `go test ./internal/upgrade/ -run 'Detect' -v` (and `-run 'Channel'` for validChannel)
- Race: `go test -race ./internal/upgrade/...`
- Full: `make test`; lint: `make lint` (golangci-lint v1.61 — staticcheck/gosimple/govet/errcheck/
  ineffassign/unused; `unused` clean because tests read every symbol)
- Cross-GOOS: tests inject d.GOOS (no build tags needed) — but confirm `GOOS=windows go test ./internal/upgrade/`
  compiles+passes (the Windows path prefixes use backslashes; the matching must be GOOS-aware).