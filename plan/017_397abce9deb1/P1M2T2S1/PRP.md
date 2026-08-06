name: "P1.M2.T2.S1 — delegate() channel→updater table + run-vs-print policy (FR-U3/FR-U4)"
description: >
  New file internal/upgrade/delegate.go (package upgrade): the delegation dispatcher that maps a detected install Channel
  (from the parallel P1.M2.T1.S1 detect.go) to its native updater and RUNs it (brew/scoop/winget/npm/mise/asdf/go-install)
  or PRINTs it (aur=needs root, nix=declarative), per PRD §9.29 FR-U3 table + FR-U4 run-vs-print. Public API:
  `Delegate(ctx, ch Channel, opts DelegateOptions) (DelegateResult, error)` + `DelegateResult{Ran, Command, ExitCode}` +
  `ErrDirectSwap` sentinel (the `direct` channel is NOT delegated — Delegate returns ErrDirectSwap and the command layer
  P1.M4 routes it to the P1.M3 self-swap). Defines a NEW streaming exec seam `ExecRunner` (Run(ctx, stdout, stderr
  io.Writer, name, args...) (exitCode, err) — STREAMS the updater's output, distinct from detect.go's capturing `Runner`
  which is already taken in this package) + `osExecRunner` prod impl (exec.CommandContext; *exec.ExitError→code-not-error).
  npm variant detection (PNPM_HOME→pnpm, BUN_INSTALL→bun, else npm) via the injected Env field. asdf is a 2-step
  (install+global). Delegate NEVER self-swaps (FR-U1), NEVER auto-sudo (FR-U4), NEVER prompts (the command layer P1.M4 owns
  the y/N + --yes; Delegate takes a pre-confirmed bool). Stdlib-only (no internal/* imports — FR-U12 walled off); file
  comment not package doc (releases.go owns it). Plus delegate_test.go (per-channel argv table; run-returns-exit-code;
  print writes command; npm-variant; asdf 2-step; ErrDirectSwap; streaming; never-sudo). Consumed by P1.M4.T2 (the upgrade
  command's runUpgrade dispatcher).

---

## Goal

**Feature Goal**: Implement the FR-U3 delegation table + FR-U4 run-vs-print policy as a pure, injectable, walled-off
function: given a detected Channel, build that channel's native updater command and either RUN it (streaming its output) or
PRINT it (for channels needing root or that are declarative). The `direct` channel is refused with `ErrDirectSwap` (the
self-swap is P1.M3's job). This is the "delegate-first" core of `stagecoach upgrade` — it never fights a package manager
because it never overwrites a manager-owned file; it asks the manager to do it.

**Deliverable** (2 new files in package upgrade):
1. **internal/upgrade/delegate.go** — `DelegateOptions` struct; `DelegateResult` struct; `ExecRunner` interface +
   `osExecRunner` prod impl; `ErrDirectSwap` sentinel; `Delegate(ctx, ch, opts)` dispatcher; `runArgv`/`printCommand`/
   `npmVariant`/`joinArgv` helpers. Switch on the 10 Channel constants from detect.go.
2. **internal/upgrade/delegate_test.go** — `fakeExecRunner` (records calls + canned code/err) + table tests covering every
   channel + npm variants + asdf 2-step + ErrDirectSwap + streaming + never-sudo.

**Success Definition**:
- `Delegate(ctx, ChannelBrew, opts)` execs `["brew","upgrade","stagecoach"]` via the injected runner, streams to opts.Out,
  returns `DelegateResult{Ran:true, Command:"brew upgrade stagecoach", ExitCode:<runner's code>}`.
- Each RUN channel (brew/scoop/winget/npm/mise/asdf/go-install) execs its exact FR-U3 argv; AUR/Nix WRITE the command to
  opts.Out and return `Ran:false, ExitCode:0`; `ChannelDirect` returns `(zero, ErrDirectSwap)`.
- npm variant: `opts.Env("PNPM_HOME")!=""`
  ⇒ `pnpm add -g @dabstractor/stagecoach@latest`; `BUN_INSTALL` ⇒ `bun add -g …`; else `npm install -g …`.
- asdf execs `asdf install stagecoach latest` THEN `asdf global stagecoach latest` (2 sequential runs; stops on first non-zero).
- A run-channel returning exit 1 ⇒ `DelegateResult{Ran:true, ExitCode:1}, nil` (an exit is NOT a Delegate error); a
  start-failure (PM absent) ⇒ `(DelegateResult{Ran:true}, err)`.
- `go build ./...`, `go vet ./internal/upgrade/...`, `go test -race ./internal/upgrade/...`, `make test`/`make lint` green;
  `gofmt -l` empty; `go.mod`/`go.sum` unchanged; NO `internal/*` import; ZERO production callers (consumer is P1.M4.T2).

## User Persona (if applicable)

**Target User**: The `stagecoach upgrade` command layer (P1.M4.T2 `runUpgrade`), which calls `Detect` then `Delegate`. End
users never call Delegate directly.

**Use Case**: `stagecoach upgrade` detects `ChannelBrew` → calls `Delegate(ctx, ChannelBrew, opts)` → brew runs
`brew upgrade stagecoach` (streamed) → returns exit 0 → upgrade reports success. Detect `ChannelAUR` → Delegate prints
`sudo pacman -Syu stagecoach-bin` → returns Ran=false → upgrade exits 0 ("here is how to update"). Detect `ChannelDirect` →
Delegate returns ErrDirectSwap → upgrade routes to the P1.M3 self-swap.

**User Journey**: (brew) user runs `stagecoach upgrade` → Detect = brew → Delegate runs `brew upgrade stagecoach` (the
user sees brew's own output) → the Homebrew-managed binary is updated BY HOMEBREW → next `brew upgrade` won't revert it.
That's the whole point: the manager owns the update, so nothing fights it.

**Pain Points Addressed**: FR-U1/FR-U3 — without delegation, a self-overwrite of a brew/scoop/winget/npm binary is silently
reverted on the manager's next upgrade and corrupts its bookkeeping. Delegate routes through the manager instead.

## Why

- **FR-U3 / FR-U4 / §9.29**: the delegation table + run-vs-print policy ARE the delegate-first architecture. This task makes
  the table executable. The direct-binary swap (P1.M3) is the exception; delegation is the default.
- **Consistency**: mirrors the existing upgrade package conventions (injectable seam + nil⇒prod-default, sentinel errors,
  file-comment-not-package-doc, stdlib-only, canned-fake tests) — no new pattern. The streaming `ExecRunner` is the natural
  counterpart to detect.go's capturing `Runner` (same package, distinct purpose, distinct name).
- **Bounded scope**: one new production file + tests. No detect logic (parallel sibling), no command/cobra (P1.M4), no
  self-swap/network (P1.M3/releases.go), no --force gate (command layer). Lands independently — Delegate needs only the
  Channel type (which detect.go ships) and stdlib exec.

## What

**User-visible behavior**: None directly (no caller yet — the command is P1.M4). Internally, `Delegate` becomes the
authoritative channel→updater dispatcher.

**Technical change** (one new file + tests; verbatim API in the Blueprint): a streaming exec seam + a switch-over-Channel
dispatcher + the npm-variant/asdf helpers + the ErrDirectSwap handoff.

### Success Criteria
- [ ] `DelegateOptions{Exec, Out, Env, Verbose, Confirmed}` exists; `Exec` nil ⇒ `osExecRunner`; `Out` is the stream/print target.
- [ ] `DelegateResult{Ran bool; Command string; ExitCode int}` exists.
- [ ] `ExecRunner` interface (`Run(ctx, stdout, stderr io.Writer, name string, args ...string) (exitCode int, err error)`) +
      `osExecRunner` (exec.CommandContext; `*exec.ExitError`⇒`(code,nil)`; start-failure⇒`(0,err)`).
- [ ] `ErrDirectSwap = errors.New("upgrade: direct channel requires self-swap, not delegation")`; `Delegate(direct)` returns it.
- [ ] RUN channels (brew/scoop/winget/npm/mise/asdf/go-install) exec their FR-U3 argv via the runner, stream to Out, Ran=true.
- [ ] PRINT channels (aur/nix) write the exact command to Out, Ran=false, ExitCode=0.
- [ ] npm variant: PNPM_HOME⇒pnpm, BUN_INSTALL⇒bun, else npm (Env nil ⇒ npm).
- [ ] asdf = 2 sequential runs (install then global); stops on first non-zero exit.
- [ ] A run-channel non-zero exit ⇒ Ran=true + that code, err==nil (an exit is not a Delegate error).
- [ ] NEVER auto-sudo: no RUN argv begins with `sudo` (sudo appears ONLY in the AUR print string).
- [ ] delegate.go is a FILE comment (not package doc); imports stdlib only; imports no `internal/*`; go.mod unchanged.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim Delegate API (DelegateOptions/DelegateResult/ExecRunner/ErrDirectSwap), the exact per-channel FR-U3
argv table (incl. the winget/asdf RUN judgment + asdf's 2-step + npm variants), the AUR/Nix print strings, the streaming-vs-
capturing distinction (why a NEW `ExecRunner` seam — `Runner` is taken by detect.go), the osExecRunner exit-vs-start-failure
gotcha, the file-comment-not-package-doc convention, the canned fakeExecRunner test idiom, and the scope fences (no detect /
no command / no swap / no --force / never-prompt / never-sudo).

### Documentation & References

```yaml
# MUST READ — the authoritative research (verbatim API + the argv table + the run/print judgment + tests)
- docfile: plan/017_397abce9deb1/P1M2T2S1/research/findings.md
  why: "§0 the Channel contract (input from detect.go); §1 the exact FR-U3 table; §2 the winget/asdf RUN judgment call;
        §3 the ExecRunner-vs-detect.Runner distinction (CRITICAL — Runner is taken); §4 the Delegate API; §5 per-channel argv
        (asdf 2-step, npm variants); §6 print strings; §7 package conventions; §8 scope fences; §9 the test plan."
  critical: "§3: do NOT reuse detect.go's `Runner` — it CAPTURES stdout; delegation STREAMS. Define `ExecRunner` (different
             name, same package). §2: winget+asdf are RUN by FR-U4's principle (per-user, non-destructive) even though FR-U4's
             example list names only 5. §5 asdf = 2 sequential argvs (install + global)."

# MUST READ — the Channel contract (the input type — produced by the parallel sibling, treat as landed)
- docfile: plan/017_397abce9deb1/P1M2T1S1/PRP.md
  why: "Defines `Channel` + the 10 constants (brew/scoop/winget/aur/npm/mise/asdf/nix/go-install/direct) + `Runner`/`osRunner`
        (detect.go's CAPTURING query seam — do NOT reuse the names) + `Detector.Detect`. Delegate switches on these EXACT
        Channel constant strings."
  critical: "The Channel constant strings are a contract — switch on ChannelBrew etc., not on display names like 'Homebrew'.
             detect.go already owns the names `Runner`, `osRunner`, `Detector`, `Detect`, `Channel` — delegate.go MUST NOT
             redefine any of them."

# MUST READ — the FR-U3 table + FR-U4 run/print policy (the spec)
- docfile: plan/017_397abce9deb1/prd_snapshot.md
  why: "§9.29 lines 596-607 = the verbatim FR-U3 delegation table (the source of truth for every argv); line 607 = FR-U4
        (RUN per-user/non-destructive, PRINT root/declarative, NEVER auto-sudo, printed⇒exit 0)."
  section: "§9.29 FR-U3 (delegation table), FR-U4 (run vs print)"
  critical: "FR-U3 npm row: 'detect pnpm/yarn/bun globals and emit the matching syntax'. FR-U4: 'NEVER auto-sudo/auto-elevate'.
             FR-U4: 'A printed command exits 0'. direct → FR-U5 (self-swap), i.e. NOT delegated."

# MUST READ — the file being created (conventions to match) + the package-doc owner
- file: internal/upgrade/releases.go
  why: "THE pattern: package upgrade, stdlib-only, sentinel errors (`var ErrHTTP = errors.New('upgrade: …')`), the injectable-
        struct pattern (Client with HTTP/BaseURL/Repo/Token), the FILE-COMMENT header. releases.go OWNS the package doc —
        delegate.go must NOT add a competing `// Package upgrade` doc."
  pattern: "Client (injectable fields) ⇒ DelegateOptions (injectable fields); ErrHTTP sentinel ⇒ ErrDirectSwap sentinel;
            httptest/canned tests ⇒ fakeExecRunner canned tests."
  gotcha: "Do NOT write a `// Package upgrade` doc in delegate.go. Start with `// delegate.go implements…` (a file comment).
           detect.go and download.go already follow this rule."

# MUST READ — the exec exit-vs-start-failure pattern (osExecRunner wraps this)
- file: internal/provider/executor.go
  why: "Lines ~51-80: exec.CommandContext(ctx, name, args…), cmd.Stdout/cmd.Stderr assignment, cmd.Run(), and the
        *exec.ExitError handling. osExecRunner.Run mirrors this but STREAMS to the injected writers (not captures to buffers)
        and returns (exitCode, err) (not (stdout, code, err))."
  critical: "A non-zero process exit = *exec.ExitError ⇒ return (ee.ExitCode(), nil) — an exit is NOT an error for delegation
             (the updater ran and reported failure; the command layer maps the code). A Start/LookPath failure (the PM binary
             is absent) ⇒ return (0, err) — that IS a Delegate error. Conflating them makes every failed updater look like
             'binary missing'. Mirror detect.go's osRunner distinction (capture vs stream is the only difference)."
  gotcha: "Do NOT use Setpgid/process-group setup for the delegated updater (that's for stagecoach's OWN child agents in the
           commit path). A delegated `brew`/`scoop` runs in the foreground group; Ctrl-C reaches it naturally."

# MUST READ — the test idiom to clone (canned fake + table)
- file: internal/upgrade/releases_test.go   # (and detect_test.go once landed)
  why: "package upgrade (internal test), table-driven, a canned fake (newFakeClient) + t.Cleanup + bytes.Buffer. Clone this
        for fakeExecRunner (records Run calls + returns canned (code, err)) + the per-channel argv table."
  pattern: "type fakeExecRunner struct{ canned func(...) (int, error); calls [][]string }; Run records (name+args) + delegates."

# CONTEXT — the future consumer (LANDS LATER)
- file: internal/upgrade/  (the P1.M4.T2 runUpgrade dispatcher — not yet created)
  why: "P1.M4.T2 will: Detect(ctx)→ch; if ch==direct-or-force → P1.M3 swap; else Delegate(ctx, ch, opts); map DelegateResult
        (Ran/Command/ExitCode) + ErrDirectSwap to the §15.4 exit codes (0 printed/up-to-date, non-zero run-failure)."
  critical: "Do NOT add the command layer here. After this subtask, grep must show Delegate called ONLY in delegate_test.go."

# CONTEXT — the npm wrapper's STAGECOACH_INSTALL_METHOD=npm (why npm detection is reliable)
- docfile: plan/017_397abce9deb1/architecture/external_deps.md
  why: "§3: the npm wrapper sets STAGECOACH_INSTALL_METHOD=npm, so Detect's tier (a) returns ChannelNpm authoritatively (no
        path-heuristic guessing). §7 lists the path roots for the other channels. Confirms the npm package is
        @dabstractor/stagecoach."
```

### Current Codebase tree (relevant slice)

```bash
internal/upgrade/
  releases.go         # READ-ONLY — Client + GitHub Releases metadata; OWNS the package doc
  releases_test.go    # READ-ONLY — the canned-fake test idiom to clone
  version.go          # READ-ONLY — CurrentSemver/Compare (P1.M1.T1.S2)
  download.go         # READ-ONLY (parallel P1.M1.T3.S2) — archive fetch; no overlap
  detect.go           # (parallel P1.M2.T1.S1, in-flight) — Channel/Runner/Detector/Detect; READ-ONLY / no symbol overlap
  detect_test.go      # (parallel) — READ-ONLY
  delegate.go         # CREATE — Delegate/DelegateOptions/DelegateResult/ExecRunner/osExecRunner/ErrDirectSwap (THIS TASK)
  delegate_test.go    # CREATE — fakeExecRunner + per-channel table tests (THIS TASK)
internal/cmd/
  upgrade.go          # READ-ONLY / not-yet-created — the command is P1.M4 (consumes Delegate later)
go.mod                # READ-ONLY — module github.com/dabstractor/stagecoach; stdlib only; NO new require
```

### Desired Codebase tree with files to be added

```bash
internal/upgrade/delegate.go        # NEW — Delegate dispatcher + ExecRunner seam + ErrDirectSwap + runArgv/printCommand/npmVariant
internal/upgrade/delegate_test.go   # NEW — fakeExecRunner + per-channel argv table + run/print/npm-variant/asdf/direct/streaming tests
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (ExecRunner ≠ detect.go's Runner — name + semantics): detect.go defines `Runner` (CAPTURING:
//   Run(ctx, name, args...) (stdout string, exitCode int, err error)) for read-only PM DB queries. Delegation needs the
//   OPPOSITE — STREAM the updater's stdout/stderr live to the terminal. Define a NEW interface `ExecRunner`
//   (Run(ctx, stdout, stderr io.Writer, name, args...) (exitCode int, err error)) + `osExecRunner`. Do NOT reuse or
//   rename `Runner`/`osRunner` — both are already in package upgrade.

// CRITICAL (an exit is NOT a Delegate error; a start-failure IS): osExecRunner.Run must return (ee.ExitCode(), nil) for a
//   *exec.ExitError (the updater ran, returned non-zero — e.g. `brew upgrade` exit 1 on a network blip) and (0, err) for a
//   Start/LookPath failure (the PM binary itself is absent). Delegate then: exit⇒DelegateResult{Ran:true,ExitCode:code},nil;
//   start-failure⇒DelegateResult{Ran:true},err. Conflating them makes every failed updater look like "brew not installed".
//   (Mirror detect.go's osRunner; the only difference is stream-vs-capture.)

// CRITICAL (winget + asdf are RUN, by FR-U4's principle): FR-U4's named RUN list is "Homebrew, Scoop, mise, go install, npm"
//   and PRINT is AUR/Nix — winget and asdf appear in FR-U3 but not in FR-U4's lists. Resolve via FR-U4's PRINCIPLE ("per-user
//   and non-destructive"): winget (`winget upgrade`) and asdf (`asdf install`) are both per-user, non-destructive, need no
//   sudo ⇒ RUN. RUN = brew/scoop/winget/npm/mise/asdf/go-install (7); PRINT = aur/nix (2); direct = ErrDirectSwap (1).

// CRITICAL (Delegate NEVER self-swaps / NEVER sees --force): the `direct` channel returns ErrDirectSwap (the command layer
//   P1.M4 routes it to the P1.M3 swap). FR-U1's "force a self-swap of a manager-owned binary via --force" is ALSO the command
//   layer's decision — it bypasses Delegate entirely and calls P1.M3 directly. Delegate has no --force parameter and never
//   overwrites a binary. (Adding a --force gate here = scope creep into the command layer.)

// CRITICAL (never auto-sudo — FR-U4): NO run-channel argv may begin with `sudo`. sudo appears ONLY in the AUR PRINT string
//   (which the USER runs). Auto-privilege-escalation by a tool that also writes binaries is a footgun; Delegate prints the
//   exact command for the user instead. (Grep guard: no RUN argv's [0] == "sudo".)

// CRITICAL (Delegate never prompts — the command layer owns y/N + --yes): DelegateOptions.Confirmed is set by the command
//   layer AFTER its y/N prompt (or --yes). Delegate itself does NOT prompt; it trusts Confirmed. FR-U9's prompt is P1.M4.T1.S2.
//   (Confirmed is an exported field ⇒ staticcheck U1000 never fires on it even though Delegate doesn't read it for control flow.)

// GOTCHA (file comment, NOT package doc): releases.go owns the package doc. Start delegate.go with `// delegate.go implements…`
//   (a file comment), exactly as detect.go and download.go do. A second `// Package upgrade` doc splits the package overview.

// GOTCHA (asdf is a 2-STEP run, not one): runArgv returns [][]string so asdf = {install, global}. Run them in sequence; stop
//   on the first non-zero. The "if it was global" condition is simplified to "always set global" (asdf global is idempotent;
//   an upgrade implies promoting to the active latest). Document this simplification. joinArgv joins with " && ".

// GOTCHA (npm variant via injected Env, NOT a real subprocess): npmVariant(opts.Env) reads PNPM_HOME/BUN_INSTALL from the
//   injected Env func (nil ⇒ "npm"). Do NOT shell out to `npm config`/`which pnpm` in Delegate — that's detection, and Detect
//   already resolved ChannelNpm. The variant only picks the COMMAND syntax. yarn is deferred (berry removed `global`); the
//   variant→argv map is trivially extensible. Defaulting to npm is safe (the package is the same).

// GOTCHA (go.mod is github.com/dabstractor/stagecoach — matches the go-install command): the go-install argv
//   `go install github.com/dabstractor/stagecoach/cmd/stagecoach@latest` uses the REAL module path (go.mod line 1). Do not
//   "correct" it to a dustin/ path.

// GOTCHA (walled off — no internal/* imports, FR-U12): delegate.go imports ONLY stdlib (context, errors, fmt, io, os, os/exec,
//   strings). Verbose is an injected func(string) field, NOT an internal/ui import. Exit-code mapping (0/1/6) is the command
//   layer's job — Delegate returns the raw updater exit code.
```

## Implementation Blueprint

### Data models and structure

```go
// ErrDirectSwap — the `direct` channel is NOT delegated (FR-U5 self-swap is P1.M3's job).
var ErrDirectSwap = errors.New("upgrade: direct channel requires self-swap, not delegation")

// ExecRunner — the streaming subprocess seam (DISTINCT from detect.go's capturing Runner).
type ExecRunner interface {
	Run(ctx context.Context, stdout, stderr io.Writer, name string, args ...string) (exitCode int, err error)
}

// osExecRunner — production ExecRunner (exec.CommandContext + stream; exit≠error).
type osExecRunner struct{}

// DelegateOptions — every env/stream/confirm seam is injectable ⇒ fully unit-testable, walled off.
type DelegateOptions struct {
	Exec      ExecRunner          // nil ⇒ osExecRunner
	Out       io.Writer           // stream target (run) + print destination (print); REQUIRED
	Env       func(string) string // nil ⇒ npmVariant returns "npm" (PNPM_HOME/BUN_INSTALL detection)
	Verbose   func(string)        // nil ⇒ no-op (logs the resolved command; no internal/ui import)
	Confirmed bool                // the command layer (P1.M4) owns the y/N + --yes; Delegate never prompts
}

// DelegateResult — the dispatcher's return (the command layer maps it to §15.4 exit codes).
type DelegateResult struct {
	Ran      bool   // true = a real updater was executed; false = printed-only
	Command  string // the argv joined (" && "-joined for asdf's 2 steps); the primary cmd for print
	ExitCode int    // run: the updater's exit code (0 success); print: 0
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/upgrade/delegate.go — file comment + imports + ErrDirectSwap + ExecRunner/osExecRunner
  - FILE COMMENT header (NOT a package doc): "// delegate.go implements the FR-U3 delegation table + FR-U4 run-vs-print
    policy… walled off (FR-U12: stdlib-only, no internal/* imports)… file comment only — releases.go owns the package doc."
  - IMPORTS: context, errors, fmt, io, os, os/exec, strings. (No internal/*.)
  - var ErrDirectSwap = errors.New("upgrade: direct channel requires self-swap, not delegation").
  - ExecRunner interface (Run(ctx, stdout, stderr io.Writer, name string, args ...string) (exitCode int, err error)).
  - osExecRunner.Run: exec.CommandContext(ctx, name, args...); cmd.Stdout=stdout; cmd.Stderr=stderr; cmd.Run().
      *exec.ExitError ⇒ (ee.ExitCode(), nil); other error (Start/LookPath/ctx) ⇒ (0, err); nil ⇒ (0, nil).

Task 2: CREATE internal/upgrade/delegate.go — DelegateOptions + DelegateResult + Delegate dispatcher
  - DelegateOptions{Exec, Out, Env, Verbose, Confirmed} (all as above; Out REQUIRED — guard nil ⇒ io.Discard w/ a note, OR document panic).
  - DelegateResult{Ran bool; Command string; ExitCode int}.
  - Delegate(ctx, ch, opts) (DelegateResult, error):
      switch ch {
      case ChannelDirect: return DelegateResult{}, ErrDirectSwap
      case ChannelAUR, ChannelNix:
          primary, full := printCommand(ch)              // (primary cmd, multi-line text incl. alternatives)
          fmt.Fprintln(opts.Out, full)
          verbose(opts, "printed update command for "+string(ch))
          return DelegateResult{Ran:false, Command:primary, ExitCode:0}, nil
      default:                                            // brew/scoop/winget/npm/mise/asdf/go-install = RUN
          argvs := runArgv(ch, opts)                      // [][]string (asdf = 2 steps)
          cmd := joinArgv(argvs)
          verbose(opts, "running: "+cmd)
          runner := opts.Exec; if runner == nil { runner = osExecRunner{} }
          code := 0
          for _, a := range argvs {
              c, err := runner.Run(ctx, opts.Out, opts.Out, a[0], a[1:]...)
              if err != nil { return DelegateResult{Ran:true, Command:cmd, ExitCode:c}, err }   // start failure
              if c != 0    { return DelegateResult{Ran:true, Command:cmd, ExitCode:c}, nil }    // updater failed
              code = c
          }
          return DelegateResult{Ran:true, Command:cmd, ExitCode:code}, nil
      }
  - verbose(opts, msg): if opts.Verbose != nil { opts.Verbose(msg) }.

Task 3: CREATE internal/upgrade/delegate.go — runArgv (the FR-U3 argv table) + npmVariant + joinArgv
  - runArgv(ch, opts) [][]string — switch on the RUN channels:
      brew:      {{"brew","upgrade","stagecoach"}}
      scoop:     {{"scoop","update","stagecoach"}}
      winget:    {{"winget","upgrade","stagecoach"}}
      mise:      {{"mise","upgrade","stagecoach"}}
      goinstall: {{"go","install","github.com/dabstractor/stagecoach/cmd/stagecoach@latest"}}
      npm:       npmVariant returns "pnpm"/"bun"/"npm" ⇒ {{"<v>","add"|"install","-g","@dabstractor/stagecoach@latest"}}
                   (pnpm/bun use "add"; npm uses "install")
      asdf:      {{"asdf","install","stagecoach","latest"},{"asdf","global","stagecoach","latest"}}
  - npmVariant(env func(string)string) string:
      if env == nil { return "npm" }
      if env("PNPM_HOME") != "" { return "pnpm" }
      if env("BUN_INSTALL") != "" { return "bun" }
      return "npm"   // yarn deferred (berry removed `global`); map is trivially extensible
  - joinArgv(argvs): strings.Join(each argv space-joined, " && ").

Task 4: CREATE internal/upgrade/delegate.go — printCommand (AUR/Nix print strings)
  - printCommand(ch) (primary, full string):
      aur: primary = "sudo pacman -Syu stagecoach-bin"
           full    = "sudo pacman -Syu stagecoach-bin\n# (or, with an AUR helper: yay -Syu stagecoach-bin)"
      nix: primary = "nix profile upgrade stagecoach"
           full    = "nix profile upgrade stagecoach\n# (declarative/flake users: run `nix flake update` in your config)"
  - Both are PRINT-only (Ran=false, ExitCode=0). Document: AUR needs root (FR-U4); Nix is immutable/declarative (no in-place swap).

Task 5: CREATE internal/upgrade/delegate_test.go — fakeExecRunner + table tests (package upgrade)
  - fakeExecRunner: struct{ canned func(name string, args []string) (int, error); calls [][]string }; Run records a[0]+args,
    delegates to canned (writes to the passed stdout so the streaming test can assert).
  - Tests:
    (a) TestDelegate_RunArgvPerChannel — table over the 7 RUN channels; fake returns (0,nil); assert recorded calls ==
        the FR-U3 argv (brew→["brew","upgrade","stagecoach"], …, go-install path, scoop, winget, mise).
    (b) TestDelegate_RunReturnsExitCode — fake returns (1,nil) ⇒ {Ran:true, ExitCode:1}, err==nil; (0,nil) ⇒ {Ran:true,ExitCode:0}.
    (c) TestDelegate_RunStartFailure — fake returns (0, errors.New("exec: not found")) ⇒ {Ran:true}, err!=nil.
    (d) TestDelegate_PrintChannels — aur/nix: opts.Out (bytes.Buffer) Contains the printed command; Ran==false; ExitCode==0;
        Command==primary ("sudo pacman -Syu stagecoach-bin" / "nix profile upgrade stagecoach").
    (e) TestDelegate_NpmVariant — Env(PNPM_HOME)!=""
       ⇒ recorded argv ["pnpm","add","-g",…]; Env(BUN_INSTALL)!=""
       ⇒ ["bun","add","-g",…]; Env nil/empty ⇒ ["npm","install","-g",…].
    (f) TestDelegate_AsdfTwoStep — recorded calls == [["asdf","install",…],["asdf","global",…]] in order; if install returns
        non-zero, global is NOT called (assert len(calls)==1).
    (g) TestDelegate_DirectErrDirectSwap — errors.Is(Delegate(ctx,ChannelDirect,opts), ErrDirectSwap); fake.calls empty.
    (h) TestDelegate_StreamsToWriter — fake writes "BREW-OUT" to the passed stdout; assert opts.Out received "BREW-OUT"
        (proves streaming, not capture).
    (i) TestDelegate_NeverSudo — for every RUN channel, assert the recorded argv[0] != "sudo" (FR-U4).
  - Each test builds DelegateOptions{Exec: fake, Out: &bytes.Buffer{}, Env: …, Verbose: nil}. No real exec.

Task 6: VERIFY — build, vet, format, focused + full tests, lint, grep guards
  - go build ./... ; go vet ./internal/upgrade/...
  - gofmt -l internal/upgrade/delegate.go internal/upgrade/delegate_test.go   # empty
  - go test ./internal/upgrade/ -run 'Delegate' -v
  - go test -race ./internal/upgrade/...
  - make test ; make lint
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the dispatcher (switch on Channel; direct=ErrDirectSwap; aur/nix=print; else=run)
func Delegate(ctx context.Context, ch Channel, opts DelegateOptions) (DelegateResult, error) {
	switch ch {
	case ChannelDirect:
		return DelegateResult{}, ErrDirectSwap
	case ChannelAUR, ChannelNix:
		primary, full := printCommand(ch)
		fmt.Fprintln(opts.Out, full)
		verbose(opts, "printed update command for "+string(ch))
		return DelegateResult{Ran: false, Command: primary, ExitCode: 0}, nil
	default:
		argvs := runArgv(ch, opts)
		cmd := joinArgv(argvs)
		verbose(opts, "running: "+cmd)
		runner := opts.Exec
		if runner == nil {
			runner = osExecRunner{}
		}
		code := 0
		for _, a := range argvs {
			c, err := runner.Run(ctx, opts.Out, opts.Out, a[0], a[1:]...)
			if err != nil {
				return DelegateResult{Ran: true, Command: cmd, ExitCode: c}, err // start failure (PM absent)
			}
			if c != 0 {
				return DelegateResult{Ran: true, Command: cmd, ExitCode: c}, nil // updater ran + failed
			}
			code = c
		}
		return DelegateResult{Ran: true, Command: cmd, ExitCode: code}, nil
	}
}

// PATTERN: osExecRunner — exit vs start-failure (the load-bearing distinction; streams, does not capture)
func (osExecRunner) Run(ctx context.Context, stdout, stderr io.Writer, name string, args ...string) (int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode(), nil // exit ⇒ not an error (the updater ran)
		}
		return 0, err // start/LookPath/ctx failure ⇒ PM absent or hung
	}
	return 0, nil
}

// PATTERN: the canned test runner (clone releases_test.go's fake style)
type fakeExecRunner struct {
	canned func(name string, args []string, stdout io.Writer) (int, error)
	calls  [][]string
}
func (f *fakeExecRunner) Run(ctx context.Context, stdout, stderr io.Writer, name string, args ...string) (int, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	return f.canned(name, args, stdout)
}
```

### Integration Points

```yaml
UPGRADE PACKAGE (internal/upgrade/delegate.go):
  - +ErrDirectSwap sentinel; +ExecRunner interface + osExecRunner; +DelegateOptions; +DelegateResult; +Delegate dispatcher;
    +runArgv/printCommand/npmVariant/joinArgv helpers.

DOWNSTREAM (this subtask ENABLES but does NOT build):
  - P1.M4.T2 runUpgrade dispatcher — Detect(ctx)→ch; if ch==direct (or --force on a manager channel) → P1.M3 swap; else
    Delegate(ctx, ch, opts); map DelegateResult{Ran,Command,ExitCode}+ErrDirectSwap to §15.4 exit codes (0 printed/up-to-date,
    non-zero run-failure). P1.M4.T1.S2 owns the y/N prompt + --yes (sets opts.Confirmed).
  - ZERO production callers of Delegate after this subtask (only delegate_test.go) — expected.

SCOPE FENCES: NO detect logic (P1.M2.T1.S1); NO cobra/command/y-N/--yes (P1.M4.T1); NO --force gate / self-swap (command layer +
  P1.M3); NO network/download (releases.go/download.go); NO exit-code→0/1/6 mapping (command layer); NO edit to detect.go /
  releases.go / version.go / download.go / config / cmd / go.mod; NO internal/* import (FR-U12); NO package doc (releases.go owns it).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build + vet (delegate.go must compile alongside the parallel detect.go — no symbol clash).
go build ./...
go vet ./internal/upgrade/...
# Expected: clean. A failure likely means a name clash with detect.go (Runner/osRunner/Channel) — use ExecRunner/osExecRunner.

# Format.
gofmt -l internal/upgrade/delegate.go internal/upgrade/delegate_test.go
# Expected: empty. If listed: gofmt -w the file(s).

# Lint.
make lint      # golangci-lint (staticcheck/gosimple/govet/errcheck/ineffassign/unused)
# Expected: zero errors. `unused` stays clean (tests read every symbol). errcheck satisfied (Run's error is checked).

# Scope guard: only delegate.go + delegate_test.go added; no existing file edited.
git status --short
# Expected: ?? internal/upgrade/delegate.go  ?? internal/upgrade/delegate_test.go  (only).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new tests (focused).
go test ./internal/upgrade/ -run 'Delegate' -v
# Expected: PASS — per-channel argv (7 RUN); run-returns-exit-code; start-failure; print (aur/nix); npm variants (3);
#           asdf 2-step; ErrDirectSwap; streaming; never-sudo.

# The full upgrade package incl. the parallel detect/download tests (no shared mutable state).
go test ./internal/upgrade/... -v
go test -race ./internal/upgrade/...
# Expected: green (race detector). detect_test.go (parallel) + download_test.go + releases_test.go all pass unaffected.

# Full repo suite.
make test
# Expected: green. delegate.go is additive and imports no internal/*.

# NOTE: there is no coverage-gate on internal/upgrade (the gate is internal/{git,provider,generate,config} only), but the
# per-channel table + variants give strong coverage of the switch + helpers regardless.
```

### Level 3: Integration Testing (System Validation)

```bash
# There is no integration/e2e surface for this task — Delegate has no production caller yet (the command is P1.M4; the runUpgrade
# dispatcher is P1.M4.T2). The unit tests (Level 2) ARE the contract: they inject a fakeExecRunner + bytes.Buffer and assert the
# exact argv / exit-code / print-streaming for every channel. A full e2e (real brew detected → delegated) is P1.M4.T3.S3.

# Sanity: the package still builds into the binary (no downstream compile break from the new symbols).
go build ./...

# Confidence check (optional, real environment only — NOT CI): if brew is actually installed locally, a scratch program can
# confirm Delegate streams `brew upgrade stagecoach` end-to-end (delete after; do NOT commit a real-brew test):
cat > /tmp/sc_delegate_check.go <<'EOF'
package main
import ("bytes";"context";"fmt";"os";"github.com/dabstractor/stagecoach/internal/upgrade")
func main() {
	var out bytes.Buffer
	res, err := upgrade.Delegate(context.Background(), upgrade.ChannelBrew, upgrade.DelegateOptions{Out: &out, Env: os.Getenv})
	fmt.Printf("res=%+v err=%v out=%q\n", res, err, out.String())
}
EOF
# (MANUAL on a brew machine; CI relies on the fakeExecRunner tests. Do not run `brew upgrade` in CI.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard 1: delegate.go is a FILE comment, not a package doc.
head -3 internal/upgrade/delegate.go | grep -q '^// delegate.go' && echo "OK: file comment"
grep -c '^// Package upgrade' internal/upgrade/delegate.go
# Expected: 0 (releases.go owns the package doc).

# Grep guard 2: no internal/* imports (walled off, FR-U12).
grep -n 'stagecoach/internal' internal/upgrade/delegate.go
# Expected: empty.

# Grep guard 3: stdlib-only — no new go.mod require.
git diff go.mod go.sum
# Expected: empty.

# Grep guard 4: NO name clash with detect.go — ExecRunner (not Runner), osExecRunner (not osRunner).
grep -cE 'type Runner interface|type osRunner ' internal/upgrade/delegate.go
# Expected: 0 (those live in detect.go). And:
grep -cE 'type ExecRunner interface|type osExecRunner ' internal/upgrade/delegate.go
# Expected: ≥1 each.

# Grep guard 5: ErrDirectSwap exists + Delegate returns it for direct.
grep -n 'ErrDirectSwap' internal/upgrade/delegate.go
# Expected: the var decl + the ChannelDirect case returning it.

# Grep guard 6: never-sudo on RUN channels — sudo appears ONLY in the AUR print string.
grep -n '"sudo"' internal/upgrade/delegate.go
# Expected: ONLY inside printCommand's aur literal (NOT in any runArgv argv).

# Grep guard 7: the FR-U3 commands are present (the argv table).
grep -cE '"brew", "upgrade", "stagecoach"|"scoop", "update", "stagecoach"|"winget", "upgrade", "stagecoach"|"mise", "upgrade", "stagecoach"|"go", "install", "github.com/dabstractor/stagecoach/cmd/stagecoach@latest"|"asdf", "install", "stagecoach", "latest"' internal/upgrade/delegate.go
# Expected: ≥1 each (the 6 non-npm RUN channels).

# Grep guard 8: npm package is @dabstractor/stagecoach (not @dustin / stagecoach alone).
grep -c '@dabstractor/stagecoach@latest' internal/upgrade/delegate.go
# Expected: ≥1 (the npm/pnpm/bun argv all use it).

# Grep guard 9: ZERO production callers of Delegate (consumer is P1.M4.T2).
grep -rn 'upgrade.Delegate(\|\.Delegate(' --include='*.go' internal/ cmd/ pkg/ | grep -v '_test.go' | grep -v 'func Delegate'
# Expected: empty (no caller outside delegate.go + tests).

# Regression: the parallel detect/download/releases tests still pass (no shared mutable state).
go test ./internal/upgrade/ -run 'Detect|Download|SelectAsset|Verify|Release|Latest' -v
# Expected: all PASS (the sibling tests, unaffected).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean (no symbol clash with detect.go)
- [ ] `go vet ./internal/upgrade/...` clean
- [ ] `gofmt -l internal/upgrade/delegate.go internal/upgrade/delegate_test.go` empty
- [ ] `go test -race ./internal/upgrade/...` green (incl. the parallel detect/download tests)
- [ ] `make test` + `make lint` pass; `go.mod`/`go.sum` unchanged

### Feature Validation
- [ ] each RUN channel (brew/scoop/winget/npm/mise/asdf/go-install) execs its exact FR-U3 argv (table test)
- [ ] run returns the runner's exit code (Ran:true); a non-zero exit is NOT a Delegate error; a start-failure IS
- [ ] AUR/Nix print the exact command to Out (Ran:false, ExitCode:0, Command=primary)
- [ ] npm variant: PNPM_HOME⇒pnpm, BUN_INSTALL⇒bun, else npm
- [ ] asdf = 2 sequential runs (install then global); stops on first non-zero
- [ ] ChannelDirect → ErrDirectSwap; Exec.Run never called
- [ ] the updater's output is STREAMED to opts.Out (not captured)
- [ ] no RUN argv begins with "sudo" (FR-U4)

### Scope-Boundary Validation
- [ ] `git status` == only `internal/upgrade/delegate.go` + `delegate_test.go` (new)
- [ ] NO edit to detect.go / releases.go / version.go / download.go / internal/cmd / go.mod
- [ ] NO `internal/*` import (FR-U12 walled-off); stdlib only
- [ ] NO package doc in delegate.go (releases.go owns it); file comment only
- [ ] NO detect logic / NO command (cobra/y-N/--yes) / NO --force gate / NO self-swap / NO network (those are P1.M2.T1 / P1.M4 / P1.M3 / releases.go)
- [ ] NO exit-code→0/1/6 mapping (the command layer P1.M4 owns §15.4)

### Code Quality & Docs
- [ ] Mirrors the Client/injectable-struct + sentinel-error conventions of releases.go
- [ ] ExecRunner is DISTINCT from detect.go's Runner (stream vs capture; different name)
- [ ] osExecRunner distinguishes exit vs start-failure (the load-bearing exec gotcha)
- [ ] Doc comments cite FR-U3/FR-U4/FR-U12 + the winget/asdf RUN judgment + the asdf "always global" simplification + npm-variant deferral of yarn

---

## Anti-Patterns to Avoid

- ❌ Don't reuse or rename detect.go's `Runner`/`osRunner`. They CAPTURE stdout (for parsing PM DB query output); delegation
  needs to STREAM the updater's output live. Define a NEW `ExecRunner` (Run with stdout/stderr io.Writer params) +
  `osExecRunner`. Same package ⇒ the names `Runner`/`osRunner` are taken.
- ❌ Don't conflate `*exec.ExitError` (the updater ran and returned non-zero) with a Start/LookPath failure (the PM binary is
  absent). osExecRunner.Run returns `(code, nil)` for the exit and `(0, err)` for the start failure. Delegate then treats a
  non-zero exit as `Ran:true, ExitCode:code, err==nil` (the command layer maps the code) and a start-failure as an error.
  Returning an error for a non-zero exit would make every failed `brew upgrade` look like "brew not installed".
- ❌ Don't make winget/asdf PRINT. FR-U4's named run list omits them, but FR-U4's PRINCIPLE ("per-user and non-destructive")
  puts them squarely in RUN (`winget upgrade` and `asdf install` are per-user, need no sudo). PRINT is only AUR (root) +
  Nix (declarative). Mis-classifying winget/asdf as print would silently skip the update for those users.
- ❌ Don't add a `--force` gate or self-swap to Delegate. FR-U1's "force a self-swap of a manager-owned binary" is the COMMAND
  LAYER's decision (P1.M4.T2) — it bypasses Delegate and calls P1.M3 directly. Delegate has no --force param and never
  overwrites a binary; `direct` is the only channel that doesn't delegate, and it returns ErrDirectSwap (P1.M3 does the swap).
- ❌ Don't auto-sudo. NO run-channel argv may start with `sudo` (FR-U4: "stagecoach NEVER auto-sudo/auto-elevates"). `sudo`
  appears ONLY in the AUR PRINT string (which the user runs). Grep guard: no `a[0]=="sudo"` in runArgv.
- ❌ Don't prompt in Delegate. FR-U9's y/N + `--yes` is the command layer (P1.M4.T1.S2). Delegate takes a pre-confirmed bool
  and trusts it. Adding a prompt here couples a leaf dispatcher to IO/TTY and breaks library use.
- ❌ Don't write a `// Package upgrade` doc in delegate.go. releases.go owns it; a second one splits the package overview.
  Start with a FILE comment (`// delegate.go implements…`) — exactly as detect.go and download.go do.
- ❌ Don't shell out to `npm config` / `which pnpm` in Delegate for the npm variant. That's DETECTION (Detect already
  resolved ChannelNpm). The variant only picks command SYNTAX; read PNPM_HOME/BUN_INSTALL from the injected Env func. (yarn
  is deferred — berry removed `global`; defaulting to npm is safe because the package is the same.)
- ❌ Don't make asdf a single command. It's 2 sequential runs (install + global). runArgv returns `[][]string` so asdf is a
  clean 2-element slice; the run loop executes them in order and stops on the first non-zero. joinArgv joins with " && ".
- ❌ Don't run real brew/scoop/winget/npm/mise/asdf/go in tests. CI has none of them reliably. Inject a `fakeExecRunner`
  (records calls + returns canned code/err) and a `bytes.Buffer` for Out. (Mirror releases_test.go's canned-fake style.)
- ❌ Don't map exit codes to 0/1/6 in Delegate. Delegate returns the RAW updater exit code + Ran + ErrDirectSwap; the command
  layer (P1.M4.T2) maps to §15.4 (0 printed/up-to-date, non-zero run-failure, 6 update-available via --check). Coupling the
  mapping here conflates the dispatcher with the CLI surface.
- ❌ Don't edit detect.go / releases.go / version.go / download.go. delegate.go is a standalone additive file in package
  upgrade; it switches on detect.go's Channel constants but shares no symbol with those files. The parallel detect.go
  (P1.M2.T1.S1) is in flight — no overlap (detect = resolve channel; delegate = act on it).

---

## Confidence Score: 9/10

The Delegate API (DelegateOptions/DelegateResult/ExecRunner/ErrDirectSwap), the exact per-channel FR-U3 argv table (incl. the
winget/asdf RUN judgment, asdf's 2-step, and the npm variants), the AUR/Nix print strings, the streaming-vs-capturing ExecRunner
distinction (why it can't reuse detect.go's Runner), the osExecRunner exit-vs-start-failure gotcha, the file-comment convention,
the canned fakeExecRunner test idiom, and the scope fences (no detect / no command / no swap / no --force / never-prompt /
never-sudo / no exit-code mapping) are all spelled out and verified against the PRD (FR-U3/U4/U1/U12), external_deps §3/§7, and
the existing package conventions. The one residual uncertainty (not a full 10) is the npm-variant detection fidelity (PNPM_HOME
and BUN_INSTALL are clear signals, but yarn-global is genuinely unreliable post-berry and is intentionally deferred) and the
asdf "always set global" simplification of the PRD's "if it was global" condition — both are documented judgment calls with
testable, extensible designs (a variant map; a 2-step argv slice), not guesses. No new dep, no internal/* import, walled off,
file-comment-not-package-doc, no symbol clash with the parallel detect.go.