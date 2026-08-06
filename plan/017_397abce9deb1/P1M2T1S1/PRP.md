name: "P1.M2.T1.S1 — detectChannel() cascade: override → PM DB → path heuristics → direct (FR-U2)"
description: >
  New file internal/upgrade/detect.go (package upgrade) implementing the §9.29 FR-U2 install-method
  detection cascade. Exports a Channel string type (10 constants: brew/scoop/winget/aur/npm/mise/asdf/
  nix/go-install/direct), an injectable Runner exec seam (interface + osRunner prod impl wrapping
  exec.CommandContext with a ~3s per-query timeout), a Detector struct (Exec/ExePath/GOOS/Override/Env/Log
  — all injectable so CI, which has none of these PMs, exercises every branch), and
  (d *Detector) Detect(ctx) (Channel, evidence string, err error) running the 4-tier cascade:
  (a) explicit override (--install-method flag > STAGECOACH_INSTALL_METHOD env, validated against the
  known channels — unknown = error, not silent direct); (b) GOOS-gated package-manager DB queries
  (brew/scoop/winget/pacman/npm/mise/asdf — first confirming query wins; nix/go-install have no query);
  (c) path heuristics on realpath(ExePath) (Homebrew Cellar, Scoop shims, Nix store, $GOPATH/bin,
  npm node_modules); (d) default direct (the only self-swap-eligible channel). Best-effort, read-only
  (never mutates), --verbose-logged, ambiguous→direct. Stdlib-only (no new go.mod require); walled off
  (no internal/* imports, FR-U12); file comment not package doc (releases.go owns it). Plus
  detect_test.go with table/canned-Runner tests. Does NOT add delegation logic (P1.M2.T2.S1), the
  upgrade command (P1.M4), or any network (releases.go is a separate concern).

---

## Goal

**Feature Goal**: Resolve how the running stagecoach binary was installed (FR-U2), returning one of 10
channels + a short evidence string, so the `stagecoach upgrade` dispatcher (P1.M2.T2.S1 `delegate()`) can
route to the channel's native updater (or self-swap only for `direct`). The cascade is best-effort,
read-only, --verbose-logged, and defaults to `direct` when ambiguous.

**Deliverable**:
1. **internal/upgrade/detect.go** (new, `package upgrade`): `Channel` type + 10 constants; `validChannel`;
   `Runner` interface + `osRunner` prod impl; `Detector` struct; `Detect` cascade; the `pmProbes` table +
   `pathHeuristics` table + confirm predicates.
2. **internal/upgrade/detect_test.go** (new, `package upgrade`): table-driven + canned-Runner tests
   (override wins; each PM probe confirms; PM-not-installed skips; GOOS gating; path heuristics; default
   direct; invalid-override error; never-mutates).

**Success Definition**:
- `Detect` with `Override="npm"` returns `ChannelNpm` and does NOT call `Exec.Run` (override short-circuits).
- `Detect` with a canned brew "installed" (exit 0) on GOOS=darwin returns `ChannelBrew`.
- `Detect` with `GOOS="linux"` never invokes the winget/scoop probes (GOOS-gated; assert via call recording).
- `Detect` with `ExePath="/opt/homebrew/Cellar/stagecoach/1.0/stagecoach"`, `Exec=nil` → `ChannelBrew` (path tier).
- `Detect` with nothing confirming → `ChannelDirect`, evidence `"default (ambiguous)"`.
- `Detect` with `Override="snap"` → non-nil error mentioning `"unknown --install-method"`.
- `go build ./...`, `go vet ./internal/upgrade/...`, `go test -race ./internal/upgrade/...`, `GOOS=windows
  go test ./internal/upgrade/...` all green; `gofmt -l` clean; `go.mod`/`go.sum` unchanged; no `internal/*` import.

## User Persona (if applicable)

**Target User**: The `stagecoach upgrade` command (P1.M4) and its dispatcher `delegate()` (P1.M2.T2.S1),
which call `Detect` to decide whether to delegate (brew/scoop/winget/npm/mise/asdf/go-install), print
(aur/nix), or self-swap (direct). End users never call Detect directly.

**Use Case**: `stagecoach upgrade` runs → `Detect(ctx)` resolves the channel → the dispatcher routes:
a brew install → `brew upgrade stagecoach`; a direct install → the verified atomic swap (FR-U5).

**User Journey**: user runs `stagecoach upgrade` → Detect checks `--install-method`/env (none) → queries
brew (`brew list stagecoach` → exit 0) → returns `ChannelBrew` + evidence `"brew list stagecoach (exit
0)"` → dispatcher delegates to `brew upgrade stagecoach`. The user's package manager is never fought.

**Pain Points Addressed**: FR-U1's core risk — self-overwriting a package-manager-owned binary that gets
reverted on the manager's next upgrade. Detect tells `upgrade` WHO owns the binary so it delegates
instead of fighting. Without Detect, `upgrade` cannot safely exist.

## Why

- **FR-U2 / §9.29**: the entire delegate-first architecture hinges on knowing the install method. Detect
  is the cascade that makes "never overwrite a package-manager-owned file" (FR-U1) enforceable.
- **Consistency**: it mirrors the existing upgrade package conventions (Client struct + injectable fields,
  sentinel errors, stdlib-only, file-comment-not-package-doc, httptest/canned tests) — no new pattern.
- **Bounded scope**: one new production file + its tests. No delegation, no command, no network, no swap.
  It lands independently (detection only queries/inspects; it needs nothing from P1.M3/P1.M4).

## What

**User-visible behavior**: None directly (no caller yet — `delegate()` is P1.M2.T2.S1, the command is
P1.M4). Internally, `Detector.Detect` becomes the authoritative install-method resolver.

**Technical change** (one new file + tests; verbatim API in the Blueprint):

```go
// internal/upgrade/detect.go
type Channel string  // brew|scoop|winget|aur|npm|mise|asdf|nix|go-install|direct
type Runner interface {
    Run(ctx context.Context, name string, args ...string) (stdout string, exitCode int, err error)
}
type osRunner struct{ timeout time.Duration }              // prod Runner (exec.CommandContext + timeout)
type Detector struct {
    Exec     Runner
    ExePath  string
    GOOS     string
    Override string
    Env      func(string) string
    Log      func(string)
}
func (d *Detector) Detect(ctx context.Context) (Channel, string, error)  // the 4-tier cascade
```

### Success Criteria
- [ ] `Channel` type + 10 constants exist; `validChannel` recognizes exactly those 10
- [ ] `Runner` interface + `osRunner` (exec.CommandContext + ~3s default timeout; non-zero exit → exitCode, err==nil)
- [ ] `Detector` struct with all 6 injectable fields; `nil`-safe (Exec nil ⇒ skip tier b; Log nil ⇒ no-op; Env nil ⇒ skip env override)
- [ ] `Detect` cascade: override(flag>env, validated) → PM probes (GOOS-gated, first-confirm-wins) → path heuristics (realpath + prefix) → default direct
- [ ] override unknown channel → error (not silent direct); tiers (b)/(c) ambiguity → direct + hint
- [ ] every probe is read-only (no install/upgrade/remove verbs); never mutates
- [ ] detect.go is a file comment (NOT a package doc); imports stdlib only; imports no `internal/*`
- [ ] `go.mod`/`go.sum` unchanged; `go build ./...` + cross-GOOS test green; `make test`/`make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim API (Channel/Runner/Detector/Detect), the exact cascade logic per tier, the
pmProbes table (per-PM command + GOOS gate + confirm predicate), the pathHeuristics roots table, the
osRunner exec+timeout+ExitError-handling gotcha, the "file comment not package doc" convention, the
canned-Runner test idiom to clone, the invalid-override-is-an-error rule, the /usr/bin ambiguity
decision, and the explicit scope fences.

### Documentation & References

```yaml
# MUST READ — the authoritative research (verbatim API + cascade + tables + gotchas + tests)
- docfile: plan/017_397abce9deb1/P1M2T1S1/research/findings.md
  why: "§2 the verbatim API (Channel/Runner/osRunner/Detector/Detect); §3 the cascade body + the
        pmProbes table (per-PM command/GOOS/confirm) + pathHeuristics roots; §4 the test matrix +
        canned fakeRunner; §1 the package conventions; §5 scope fences."
  critical: "§3 tier (b): a non-zero exit is *exec.ExitError → extract ExitCode(), return err==nil so the
             probe treats 'not installed' as a SKIP, not an error. §3 tier (a): an INVALID override is a
             hard error, not silent direct. §3 /usr/bin decision: leave it OUT of path prefixes (ambiguous;
             let the pacman query own AUR). §1: file comment, NOT package doc."

# MUST READ — the FR-U2 spec (the cascade) + FR-U3 (channel naming for delegate consistency)
- docfile: plan/017_397abce9deb1/prd_snapshot.md
  why: "§9.29 FR-U2 (line ~595) is the cascade spec; FR-U3 (line ~596) is the delegation table whose
        channel names detect.go's constants must match (use 'aur', not 'pacman')."
  section: "§9.29 Self-update — FR-U2 (Install-method detection cascade), FR-U3 (delegation table)"

# MUST READ — the per-channel detection signals (PM queries + path roots)
- docfile: plan/017_397abce9deb1/architecture/external_deps.md
  why: "§7 is the detection-cascade reference: the PM DB query invocations (brew list / scoop prefix /
        winget list / pacman -Q / npm ls -g / mise ls / asdf list), the path roots (Homebrew Cellar,
        Scoop shims, npm node_modules, /nix/store, $GOPATH/bin, /usr/bin), and the ~3s timeout mandate.
        §3 explains the npm wrapper's STAGECOACH_INSTALL_METHOD=npm (the tier-a env signal)."
  critical: "§7 'these queries must not hang — apply a short timeout' ⇒ osRunner default 3s. §7 lists
             /usr/bin as an AUR heuristic but flags ambiguity — see findings §3 for the decision to
             exclude it from path prefixes."

# MUST READ — the file being created (conventions to match)
- file: internal/upgrade/releases.go
  why: "THE pattern to mirror: package upgrade, stdlib-only, sentinel errors (var ErrX = errors.New(
        'upgrade: …')), the injectable-struct pattern (Client with HTTP/BaseURL/Repo/Token fields +
        methods on *Client), the file-comment header. releases.go OWNS the package doc — detect.go
        must NOT add a competing 'package upgrade' doc."
  pattern: "Client struct → Detector struct; Client.LatestStable → Detector.Detect; ErrHTTP sentinel →
            ErrUnknownChannel-style sentinels as needed."
  gotcha: "Do NOT write a `// Package upgrade` doc comment in detect.go — releases.go has it. Start
           detect.go with `// detect.go implements…` (a file comment), exactly as the parallel
           download.go does (per its PRP)."

# MUST READ — the test idiom to clone
- file: internal/upgrade/releases_test.go
  why: "package upgrade (internal test), table-driven, a canned fake (newFakeClient/cannedLatest) +
        t.Cleanup. Clone this style: a fakeRunner struct recording calls + returning canned
        (stdout, exitCode, err) per command."
  pattern: "func newFakeRunner(...) *fakeRunner; table cases []struct{...}; t.Run subtests; assert via
            the recorded call list for GOOS-gating/never-mutates checks."

# CONFIRMING — the exec.CommandContext + ExitError pattern (osRunner wraps this)
- file: internal/git/git.go
  why: "Lines ~459/491/532: the house subprocess pattern — exec.CommandContext(ctx, name, args...),
        capture stdout, map *exec.ExitError to a code. osRunner.Run mirrors this with a per-call
        context.WithTimeout."
  critical: "A non-zero process exit comes back as *exec.ExitError whose ExitCode() method gives the
             code; a start/LookPath failure is a different (non-ExitError) error. osRunner must return
             (stdout, code, nil) for the exit case and (\"\", 0, err) for the start-failure case, so the
             probe logic can distinguish 'PM absent' (skip) from 'not installed' (skip) from 'installed'
             (confirm)."

# CONTEXT — the parallel sibling (no overlap; same package, different file)
- docfile: plan/017_397abce9deb1/P1M1T3S2/PRP.md
  why: "Parallel sibling adds internal/upgrade/download.go (GitHub archive fetch + SHA256). It shares NO
        symbol with detect.go (download = network archive; detect = local install method). Both are
        additive files in package upgrade; both follow 'file comment, not package doc'. No conflict."
  critical: "Do NOT edit releases.go/version.go/download.go. detect.go is a standalone additive file."

# CONTEXT — the future consumer (LANDS LATER)
- file: internal/upgrade/  (delegate.go — P1.M2.T2.S1, not yet created)
  why: "P1.M2.T2.S1's delegate() will switch on the Channel values detect.go returns. So the Channel
        constant STRINGS are a contract — use exactly brew/scoop/winget/aur/npm/mise/asdf/nix/go-install/
        direct; do NOT rename (e.g. not 'pacman', not 'homebrew')."
```

### Current Codebase tree (relevant slice)

```bash
internal/upgrade/
  version.go          # READ-ONLY — CurrentSemver/Compare (P1.M1.T1.S2)
  releases.go         # READ-ONLY — Client + GitHub Releases metadata (P1.M1.T3.S1); OWNS the package doc
  releases_test.go    # READ-ONLY — the test idiom to clone
  download.go         # (parallel P1.M1.T3.S2, in-flight) — archive fetch; READ-ONLY / no overlap
  detect.go           # CREATE — Channel/Runner/Detector/Detect (THIS TASK)
  detect_test.go      # CREATE — canned-Runner tests (THIS TASK)
internal/cmd/
  upgrade.go          # READ-ONLY / not-yet-created — the command is P1.M4 (consumes Detect later)
go.mod                # READ-ONLY — stdlib only; NO new require
```

### Desired Codebase tree with files to be added

```bash
internal/upgrade/detect.go        # NEW — Channel type, 10 consts, Runner+osRunner, Detector, Detect cascade, pmProbes/pathHeuristics tables
internal/upgrade/detect_test.go   # NEW — table + canned-Runner tests (override/probe/path/default/error/gating)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (file comment, NOT package doc): releases.go owns the package doc (`// releases.go implements…`
//   followed by `package upgrade`). detect.go must start with a FILE comment (`// detect.go implements…`),
//   NOT a `// Package upgrade` doc — otherwise Go complains about duplicate package docs / the doc
//   tooling splits the package overview. The parallel download.go follows the same rule (per its PRP).

// CRITICAL (exec.ExitError vs start failure): osRunner.Run must distinguish (a) start/LookPath failure
//   (the PM binary is absent) → return ("", 0, err) so the probe SKIPS; (b) a non-zero process exit
//   (e.g. `brew list` exit 1 = not installed) → return (stdout, code, nil) so the probe reads exitCode
//   and skips; (c) exit 0 → return (stdout, 0, nil) so the confirm predicate runs. Conflating (a) and
//   (b) makes every "not installed" look like an error and breaks the cascade. Mirror git.go's exit handling.

// CRITICAL (invalid override is an ERROR, not silent direct): tiers (b)/(c) ambiguity → direct, but a
//   user-supplied --install-method=garbage or STAGECOACH_INSTALL_METHOD=garbage is an EXPLICIT wrong
//   value → return a wrapped error ("upgrade: unknown --install-method %q"). Silent-direct here would
//   hide a typo and could trigger a wrong-channel delegation or an unwanted self-swap.

// CRITICAL (per-query timeout — queries must not hang): osRunner wraps each Run in
//   context.WithTimeout(ctx, 3s default). A hung brew/scoop/winget must not stall `upgrade`. external_deps
//   §7: "these queries must not hang — apply a short timeout." The timeout is per-QUERY, not per-cascade.

// CRITICAL (GOOS gating via d.GOOS, not build tags): the winget/scoop probes are Windows-only, brew/
//   pacman are darwin/linux. Gate them at RUNTIME via the injected d.GOOS field (so one binary runs on
//   all platforms and tests inject GOOS freely) — NOT via //go:build tags. Tests assert the winget probe
//   is never invoked when d.GOOS="linux" (via the fakeRunner call recording).

// CRITICAL (walled off — no internal/* imports, FR-U12): detect.go imports ONLY stdlib (context, os,
//   os/exec, path/filepath, runtime, strings, fmt, time, errors). It must not import internal/ui (the
//   --verbose logger is INJECTED as a func(string) field, not imported), internal/cmd, internal/git, etc.

// GOTCHA (channel strings are a contract): delegate() (P1.M2.T2.S1) switches on these EXACT strings.
//   Use brew/scoop/winget/aur/npm/mise/asdf/nix/go-install/direct — NOT homebrew/pacman/etc. FR-U3's
//   display names are Homebrew/AUR/etc., but the Channel constants are the lowercase identifiers.

// GOTCHA (/usr/bin is ambiguous — exclude from path prefixes): external_deps §7 lists /usr/bin as an AUR
//   heuristic, but a manual `cp stagecoach /usr/bin/` would false-positive as AUR and the dispatcher
//   would print `sudo pacman…` for a non-pacman install. Decision: detect AUR via the pacman QUERY
//   (tier b) only; leave /usr/bin OUT of pathHeuristics. Ambiguous → direct (FR-U2). Document this.

// GOTCHA (path matching is GOOS-aware): Unix roots are `/opt/homebrew/Cellar/` etc.; the Scoop root is
//   `…\scoop\shims\` (backslashes). filepath.Clean normalizes per the RUNNING OS's separator, but tests
//   inject ExePath strings + d.GOOS independently. Lowercase the realpath for the Scoop match OR match
//   on the literal injected path so a Windows-style path on a GOOS=linux test still matches. Keep it simple.

// GOTCHA (nil-safe Detector): Exec nil ⇒ skip tier (b); ExePath "" ⇒ skip tier (c); Env nil ⇒ skip the
//   env override; Log nil ⇒ no-op (do NOT call a nil func). Every field is optional; a zero-value
//   Detector{} with no override returns ChannelDirect (the safe default).
```

## Implementation Blueprint

### Data models and structure

```go
// Channel — the detected install method (FR-U2). String values are stable contract IDs for delegate().
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

// Runner — injectable subprocess seam for PM DB queries (tier b).
type Runner interface {
    Run(ctx context.Context, name string, args ...string) (stdout string, exitCode int, err error)
}

// osRunner — production Runner (exec.CommandContext + per-query timeout).
type osRunner struct{ timeout time.Duration } // 0 ⇒ 3s

// Detector — the injectable resolver. Every env-touching seam is a field ⇒ fully unit-testable.
type Detector struct {
    Exec     Runner              // nil ⇒ skip tier (b)
    ExePath  string              // "" ⇒ skip tier (c)
    GOOS     string              // gates PM probes; "" ⇒ treat as runtime.GOOS? (decide: require non-empty)
    Override string              // --install-method flag value; "" ⇒ unset
    Env      func(string) string // nil ⇒ skip env override
    Log      func(string)        // nil ⇒ no-op
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/upgrade/detect.go — file comment + imports + Channel type/constants + validChannel
  - FILE COMMENT header (NOT a package doc): "// detect.go implements the FR-U2 install-method detection
    cascade… walled off (FR-U12: stdlib-only, no internal/* imports)… file comment only — releases.go
    owns the package doc."
  - IMPORTS: context, errors, fmt, os, os/exec, path/filepath, runtime, strings, time. (No internal/*.)
  - type Channel string + the 10 const declarations above (exact strings).
  - validChannel(string) bool — a switch or map membership over the 10 constants.

Task 2: CREATE internal/upgrade/detect.go — Runner interface + osRunner prod impl
  - Runner interface: Run(ctx, name, args...) (stdout string, exitCode int, err error).
  - osRunner.Run: default timeout = 3s if r.timeout==0; ctx = context.WithTimeout(ctx, timeout); defer cancel;
    exec.CommandContext(ctx, name, args...); capture stdout (&bytes.Buffer); cmd.Run().
    - On a *exec.ExitError: return (stdout.String(), ee.ExitCode(), nil)  ← exit, NOT an error.
    - On a start/LookPath error (not ExitError): return ("", 0, err)  ← PM absent ⇒ skip.
    - On ctx timeout/deadline: return (stdout.String(), 0, ctx.Err()) treated as skip (log + continue).
  - Document the err-vs-exitCode contract on the interface.

Task 3: CREATE internal/upgrade/detect.go — Detector struct + the Detect cascade
  - Detector struct with the 6 fields above (all optional/nil-safe).
  - (d *Detector) log(msg string): if d.Log != nil { d.Log(msg) }.
  - Detect(ctx) (Channel, string, error): the 4-tier cascade from findings §3:
      (a) detectOverride() — flag (d.Override) then env (d.Env("STAGECOACH_INSTALL_METHOD")); validate via
          validChannel; unknown ⇒ wrapped error; known ⇒ return (channel, evidence, nil).
      (b) if d.Exec != nil: detectPackageManager(ctx) — loop pmProbes, GOOS-gate, Run, confirm; first wins.
      (c) detectPath() — realpath(ExePath), prefix-match pathHeuristics (+ GOPATH/npm env-derived roots).
      (d) log the ambiguous hint; return (ChannelDirect, "default (ambiguous)", nil).

Task 4: CREATE internal/upgrade/detect.go — the pmProbes table + confirm predicates
  - pmProbe struct { channel Channel; goos []string; name string; args []string; confirm func(stdout string, exitCode int) bool }.
  - pmProbes = the 7 rows from findings §3 (brew/scoop/winget/pacman/npm/mise/asdf). NOTE: nix + go-install
    are NOT here (no ownership query — path-only).
  - confirm helpers: exit0Confirm (exitCode==0); grepConfirm(needle) (strings.Contains(stdout, needle)).
  - detectPackageManager loop: for each probe where goosAdmits(probe.goos, d.GOOS): Run; if err!=nil →
    log "skip (absent)"; continue; if exitCode!=0 → log "skip (exit N)"; continue; if confirm → return
    (channel, evidence, true). goosAdmits: nil slice ⇒ all GOOS; else d.GOOS ∈ slice.

Task 5: CREATE internal/upgrade/detect.go — the pathHeuristics table + detectPath
  - pathHeuristics = []struct{ prefix string; channel Channel }:
      {"/opt/homebrew/Cellar/", ChannelBrew}, {"/usr/local/Cellar/", ChannelBrew},
      {`\scoop\shims\`, ChannelScoop}, {"/nix/store/", ChannelNix}.
    (EXCLUDE /usr/bin — ambiguous; pacman query owns AUR. Document the decision in a comment.)
  - detectPath: if d.ExePath=="" → false; real = EvalSymlinks(ExePath) (tolerate err → use ExePath);
    clean; for each heuristic prefix-match → return. ALSO: go-install via d.Env("GOPATH") (default
    ~/go/bin) prefix-match → ChannelGoInstall; npm via a node_modules/stagecoach conventional prefix →
    ChannelNpm (best-effort). Keep these last + best-effort.
  - GOOS-aware matching: lower-case both sides for the Scoop backslash prefix on a case-insensitive FS;
    match the literal injected path so tests are deterministic.

Task 6: CREATE internal/upgrade/detect_test.go — canned-Runner + table tests (package upgrade)
  - fakeRunner: struct{ canned func(name,args)(string,int,error); calls []string }; Run records + delegates.
  - Tests (findings §4 matrix): override-flag-wins; override-env-wins; override-invalid-error; per-PM
    confirm (table, 7 channels); PM-not-installed-skip; GOOS-gating (winget not called on linux);
    path-brew-Cellar; path-nix-store; path-go-install-via-GOPATH; default-direct; never-mutates (grep
    the recorded calls for install/upgrade/remove verbs → none).
  - Each test builds a Detector with injected fields (no real exec, no real os.Executable).

Task 7: VERIFY — build, vet, format, focused + cross-GOOS tests, race, lint, grep guards
  - go build ./... ; go vet ./internal/upgrade/...
  - gofmt -l internal/upgrade/detect.go internal/upgrade/detect_test.go   # empty
  - go test ./internal/upgrade/ -run 'Detect|Channel' -v
  - GOOS=windows go test ./internal/upgrade/...   # Windows path prefixes compile + pass
  - go test -race ./internal/upgrade/...
  - make test ; make lint
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the cascade (override short-circuits; tiers fall through to direct)
func (d *Detector) Detect(ctx context.Context) (Channel, string, error) {
	if ch, ev, err, ok := d.detectOverride(); ok || err != nil {
		return ch, ev, err // override known ⇒ return; invalid ⇒ err; absent ⇒ fall through (ok=false, err=nil)
	}
	if d.Exec != nil {
		if ch, ev, ok := d.detectPackageManager(ctx); ok { return ch, ev, nil }
	}
	if ch, ev, ok := d.detectPath(); ok { return ch, ev, nil }
	d.log("install method ambiguous; defaulting to direct (pin --install-method if wrong)")
	return ChannelDirect, "default (ambiguous)", nil
}

// PATTERN: osRunner — exit vs start-failure (the load-bearing distinction)
func (r *osRunner) Run(ctx context.Context, name string, args ...string) (string, int, error) {
	timeout := r.timeout
	if timeout == 0 { timeout = 3 * time.Second }
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok { return out.String(), ee.ExitCode(), nil } // exit ⇒ not error
		return "", 0, err // start/LookPath/timeout ⇒ PM absent or hung ⇒ skip
	}
	return out.String(), 0, nil
}

// PATTERN: the canned test Runner (clone releases_test.go's fake style)
type fakeRunner struct {
	canned func(name string, args []string) (string, int, error)
	calls  []string
}
func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (string, int, error) {
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	return f.canned(name, args)
}
```

### Integration Points

```yaml
NO database / migration / routes / new go.mod require / new CLI flag / network. One new package file + tests.

UPGRADE PACKAGE (internal/upgrade/detect.go):
  - +type Channel + 10 const; +Runner interface + osRunner; +Detector struct; +Detect cascade;
    +pmProbes/pathHeuristics tables + confirm predicates.

DOWNSTREAM (this subtask ENABLES but does NOT build):
  - P1.M2.T2.S1 delegate() — switches on the Channel values; runs/prints the channel's updater (FR-U3).
  - P1.M4.T1.S1 upgrade command — constructs a Detector (Exec=&osRunner{}, ExePath=os.Executable(),
    GOOS=runtime.GOOS, Override=<--install-method flag>, Env=os.Getenv, Log=<ui.Verbose>) and calls Detect.
  - ZERO production callers of Detect after this subtask (only detect_test.go) — expected.

SCOPE FENCES: NO delegation logic (P1.M2.T2.S1); NO upgrade command (P1.M4); NO network/releases.go edit;
  NO releases.go/version.go/download.go edit (parallel); NO internal/* import (FR-U12); NO package doc
  (releases.go owns it); NO new go.mod require.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build + vet.
go build ./...
go vet ./internal/upgrade/...

# Format.
gofmt -l internal/upgrade/detect.go internal/upgrade/detect_test.go
# Expected: empty. If listed: gofmt -w the file(s).

# Lint.
make lint      # golangci-lint v1.61 (staticcheck/gosimple/govet/errcheck/ineffassign/unused)
# Expected: zero errors. `unused` stays clean (tests read every symbol). `gosimple`/`staticcheck` clean.

# Scope guard: only detect.go + detect_test.go added; no existing file edited.
git status --short
# Expected: ?? internal/upgrade/detect.go  ?? internal/upgrade/detect_test.go  (only).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new tests (focused).
go test ./internal/upgrade/ -run 'Detect|Channel' -v
# Expected: PASS — override-flag/env/invalid; each PM probe confirms; PM-not-installed skips; GOOS
#           gating; path brew/nix/go-install; default direct; never-mutates.

# Cross-GOOS (the Windows path prefixes must compile + the GOOS-gating logic must be GOOS-agnostic).
GOOS=windows go test ./internal/upgrade/...
GOOS=linux   go test ./internal/upgrade/...
GOOS=darwin  go test ./internal/upgrade/...
# Expected: all pass (GOOS is injected via d.GOOS, not build tags).

# Race + full upgrade package.
go test -race ./internal/upgrade/...
go test ./internal/upgrade/... -v
# Expected: green; the parallel download.go tests (P1.M1.T3.S2) also pass (no shared mutable state).

# Full repo suite.
make test
# Expected: green (race detector). No regression — detect.go is additive and imports no internal/*.
```

### Level 3: Integration Testing (System Validation)

```bash
# There is no integration/e2e surface for this task — Detect has no production caller yet (delegate() is
# P1.M2.T2.S1; the upgrade command is P1.M4). The unit tests (Level 2) ARE the contract: they inject a
# canned Runner + ExePath + GOOS + Env and assert the resolved channel for every tier. A full e2e (real
# brew install detected → delegated) is P1.M4.T3.S3.

# Sanity: the package still builds into the binary (no downstream compile break from the new symbols).
go build ./...

# Confidence check (optional, real environment only — NOT CI): if brew is actually installed locally,
# confirm the prod osRunner detects it end-to-end via a scratch program (delete after):
cat > /tmp/sc_detect_check.go <<'EOF'
package main
import ("context";"fmt";"os";"runtime";"github.com/dustin/stagecoach/internal/upgrade")
func main() {
	d := &upgrade.Detector{GOOS: runtime.GOOS, ExePath: selfExe(), Env: os.Getenv}
	d.Detect(context.Background()) // print result for manual inspection
}
EOF
# (This is a MANUAL check on a machine with a real PM; CI relies on the canned-Runner tests.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard 1: detect.go is a FILE comment, not a package doc.
head -3 internal/upgrade/detect.go | grep -q '^// detect.go' && echo "OK: file comment"
grep -c '^// Package upgrade' internal/upgrade/detect.go
# Expected: 0 (releases.go owns the package doc).

# Grep guard 2: no internal/* imports (walled off, FR-U12).
grep -n 'stagecoach/internal' internal/upgrade/detect.go
# Expected: empty.

# Grep guard 3: stdlib-only — no new go.mod require.
git diff go.mod go.sum
# Expected: empty.

# Grep guard 4: the 10 channel constants exist with the contract strings.
grep -cE 'Channel(Brew|Scoop|Winget|AUR|Npm|Mise|Asdf|Nix|GoInstall|Direct)\s+Channel\s*=' internal/upgrade/detect.go
# Expected: 10.

# Grep guard 5: 'aur' not 'pacman' (FR-U3 consistency for delegate()).
grep -c '"aur"' internal/upgrade/detect.go && ! grep -c '"pacman"' internal/upgrade/detect.go
# Expected: ≥1 for aur; 0 for pacman (as a Channel value).

# Grep guard 6: no /usr/bin in pathHeuristics (the ambiguity decision).
grep -c '/usr/bin/' internal/upgrade/detect.go
# Expected: 0 (excluded; pacman query owns AUR).

# Grep guard 7: ZERO production callers of Detect (consumer is P1.M2.T2/P1.M4).
grep -rn '\.Detect(' --include='*.go' internal/ cmd/ pkg/ | grep -v '_test.go' | grep -v 'func (d \*Detector) Detect'
# Expected: empty (no caller outside detect.go + tests).

# Grep guard 8: never-mutates — every probe verb is read-only (list/prefix/ls/-Q), never install/upgrade/remove.
grep -oE '"(brew|scoop|winget|pacman|npm|mise|asdf)"' internal/upgrade/detect.go   # the PM names
grep -E 'install|upgrade|remove|uninstall' internal/upgrade/detect.go | grep -v '//\|pmProbe\|comment'
# Expected: no install/upgrade/remove VERBS in the args of any pmProbe row (only in comments).

# Grep guard 9: per-query timeout present in osRunner.
grep -c 'context.WithTimeout' internal/upgrade/detect.go
# Expected: ≥1 (the osRunner.Run timeout).

# Regression: the parallel download.go tests still pass (no shared mutable state).
go test ./internal/upgrade/ -run 'Download|SelectAsset|Verify|Checksum' -v
# Expected: all PASS (P1.M1.T3.S2's tests, unaffected).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/upgrade/...` clean
- [ ] `gofmt -l internal/upgrade/detect.go internal/upgrade/detect_test.go` empty
- [ ] `GOOS=windows/linux/darwin go test ./internal/upgrade/...` all pass (GOOS injected, no build tags)
- [ ] `go test -race ./internal/upgrade/...` green
- [ ] `make test` + `make lint` pass; `go.mod`/`go.sum` unchanged

### Feature Validation
- [ ] override flag > env > cascade (flag short-circuits; Exec.Run not called)
- [ ] override invalid (unknown channel) → error (not silent direct)
- [ ] each of the 7 PM probes returns its channel on a canned "installed" (exit 0 / grep match)
- [ ] PM-not-installed (exit 1) / PM-absent (start error) → skip, fall through
- [ ] GOOS gating: winget/scoop probes never invoked when GOOS=linux/darwin (call recording)
- [ ] path heuristics: brew Cellar / Scoop shims / Nix store / $GOPATH/bin → correct channel
- [ ] nothing confirms → ChannelDirect, evidence "default (ambiguous)"
- [ ] every probe read-only (no install/upgrade/remove verbs)

### Scope-Boundary Validation
- [ ] `git status` == only `internal/upgrade/detect.go` + `detect_test.go` (new)
- [ ] NO edit to releases.go / version.go / download.go (parallel) / internal/cmd / go.mod
- [ ] NO `internal/*` import (FR-U12 walled-off); stdlib only
- [ ] NO package doc in detect.go (releases.go owns it); file comment only
- [ ] NO delegation logic / NO upgrade command / NO network (those are P1.M2.T2 / P1.M4 / releases.go)
- [ ] Channel constants use contract strings (brew/scoop/winget/aur/npm/mise/asdf/nix/go-install/direct)

### Code Quality & Docs
- [ ] Mirrors the Client/injectable-struct + sentinel-error conventions of releases.go
- [ ] osRunner distinguishes exit vs start-failure (the load-bearing exec gotcha)
- [ ] per-query ~3s timeout (queries must not hang)
- [ ] Doc comments cite FR-U2/FR-U12 + the override-validation rule + the /usr/bin ambiguity decision

---

## Anti-Patterns to Avoid

- ❌ Don't write a `// Package upgrade` doc in detect.go. releases.go owns the package doc; a second one
  splits the package overview and may trip doc tooling. Start with a FILE comment (`// detect.go
  implements…`) — exactly as the parallel download.go does.
- ❌ Don't conflate `*exec.ExitError` (non-zero exit = "not installed") with a start/LookPath error (PM
  absent). osRunner.Run must return `(stdout, code, nil)` for the exit and `("", 0, err)` for the start
  failure, so the probe treats both as a SKIP. Returning an error for a non-zero exit would abort the
  whole cascade on the first "not installed" PM. (Mirror git.go's exit handling.)
- ❌ Don't silently default-to-direct on an INVALID override. A user-supplied
  `--install-method=snap`/`STAGECOACH_INSTALL_METHOD=snap` is an explicit wrong value → hard error.
  Only tiers (b)/(c) AMBIGUITY → direct. Silent-direct on a typo could trigger a wrong-channel
  delegation or an unwanted self-swap.
- ❌ Don't gate PM probes with `//go:build` tags. Gate them at RUNTIME via the injected `d.GOOS` field so
  one binary runs everywhere and tests inject GOOS freely. (Build tags would split the probe table across
  files and make the GOOS-gating untestable from a single platform.)
- ❌ Don't omit the per-query timeout. A hung brew/scoop/winget would stall `upgrade`. external_deps §7 is
  explicit: "these queries must not hang." osRunner wraps every Run in `context.WithTimeout(ctx, 3s)`.
- ❌ Don't name the AUR channel `"pacman"`. FR-U3's delegation row is "AUR"; delegate() (P1.M2.T2.S1)
  switches on these strings. Use `"aur"`. (The pacman COMMAND appears in the probe args, but the Channel
  IDENTITY is aur.)
- ❌ Don't add `/usr/bin` to the path prefixes. It's ambiguous (a manual `cp` to /usr/bin would
  false-positive as AUR and the dispatcher would print `sudo pacman…` for a non-pacman install). Let the
  pacman QUERY (tier b) own AUR detection; leave /usr/bin out of pathHeuristics.
- ❌ Don't add delegation logic, the upgrade command, or any network call. detect.go only RESOLVES the
  channel. delegate() is P1.M2.T2.S1; the command is P1.M4; the GitHub client is releases.go. detect.go
  imports no net/http and no internal/*.
- ❌ Don't import `internal/ui` for the verbose logger. Inject it as a `func(string)` field (`Log`) so
  detect.go stays walled off (FR-U12: no internal/* imports). The command layer wires `ui.Verbose` in.
- ❌ Don't edit releases.go / version.go / download.go. detect.go is a standalone additive file in
  `package upgrade`; it shares no symbol with them. The parallel download.go (P1.M1.T3.S2) is in flight —
  no overlap (download = network archive; detect = local install method).
- ❌ Don't run real brew/scoop/winget in tests. CI has none of them. Inject a canned `fakeRunner` and
  assert against its recorded calls. (Mirror releases_test.go's httptest/canned style.)

---

## Confidence Score: 9/10

The API, cascade, probe table, path-roots table, exec/exit gotcha, and canned-test idiom are all spelled
out and verified against the PRD (FR-U2/U3), external_deps §7, and the existing package conventions. The
one residual uncertainty (not a full 10) is the exact per-PM stdout/exit semantics (e.g. does `scoop
prefix stagecoach` exit 0 vs 1 on absence; does `mise ls` list stagecoach by name) — but external_deps §7
explicitly defers those to implementation-time verification (FR-D5 discipline), CI has none of these PMs
(so tests use canned outputs regardless), and the `confirm` predicates are designed to be tuned per PM
without restructuring. The design absorbs that uncertainty: a wrong confirm predicate fails a test
loudly, not silently. No new dep, no internal/* import, walled off, file-comment-not-package-doc.