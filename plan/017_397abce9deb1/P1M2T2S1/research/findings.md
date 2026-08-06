# P1.M2.T2.S1 Research Findings — delegate() channel→updater table + run-vs-print (FR-U3/FR-U4)

Source: `plan/017_…/prd_snapshot.md` §9.29 FR-U3/FR-U4, `architecture/external_deps.md` §7, the parallel PRP
P1.M2.T1.S1 (detect.go — the Channel contract), and `internal/upgrade/releases.go` (package conventions).

## 0. The Channel contract (INPUT — produced by the parallel P1.M2.T1.S1, treat as landed)

detect.go (package upgrade) exports:
```go
type Channel string
const (
    ChannelBrew Channel = "brew"; ChannelScoop = "scoop"; ChannelWinget = "winget"; ChannelAUR = "aur"
    ChannelNpm = "npm"; ChannelMise = "mise"; ChannelAsdf = "asdf"; ChannelNix = "nix"
    ChannelGoInstall = "go-install"; ChannelDirect = "direct"
)
```
- detect.go ALSO defines `Runner` (interface: `Run(ctx, name, args...) (stdout string, exitCode int, err error)`) and
  `osRunner` (its prod impl) — used for read-only PM DB QUERIES that CAPTURE stdout to a string.
- delegate.go is in the SAME package, so it MUST NOT redefine `Runner`/`osRunner` — it needs a SEPARATE streaming seam
  (see §3). The Channel constant STRINGS are a contract; switch on them verbatim.

## 1. FR-U3 — the exact delegation table (PRD lines 596-607, verbatim)

| Channel | Command | Run/Print |
|---|---|---|
| brew (Homebrew) | `brew upgrade stagecoach` | RUN |
| scoop | `scoop update stagecoach` | RUN |
| winget | `winget upgrade stagecoach` | RUN (judgment — §2) |
| npm | `npm install -g @dabstractor/stagecoach@latest` (detect pnpm/yarn/bun globals; emit matching syntax) | RUN |
| mise | `mise upgrade stagecoach` | RUN |
| asdf | `asdf install stagecoach latest` (+ `asdf global stagecoach latest` if it was global) | RUN (judgment — §2) |
| aur | print `sudo pacman -Syu stagecoach-bin` (or `yay -Syu stagecoach-bin`) | PRINT (needs root) |
| nix | print (`nix profile upgrade` / `nix flake update` on user config) | PRINT (declarative/immutable) |
| go-install | `go install github.com/dabstractor/stagecoach/cmd/stagecoach@latest` | RUN |
| direct | — (FR-U5 self-swap) | NOT DELEGATED → ErrDirectSwap |

Module path confirmed: `go.mod` = `module github.com/dabstractor/stagecoach` (so the go-install command matches the
real module). npm package = `@dabstractor/stagecoach` (PRD-confirmed).

## 2. The RUN-vs-PRINT judgment call (winget + asdf — the one ambiguity)

FR-U4 NAMING: RUN list = "Homebrew, Scoop, mise, go install, npm"; PRINT list = AUR(root) / Nix(declarative).
Winget and asdf are NOT in either named list, BUT FR-U3 gives them commands. Resolve via FR-U4's PRINCIPLE
("RUNS … for channels whose updater is per-user and non-destructive"):
- **Winget** = RUN (`winget upgrade stagecoach` is per-user, non-destructive — identical性质 to Scoop). Windows default PM.
- **asdf** = RUN (`asdf install` is per-user, non-destructive — identical性质 to mise).
Both qualify under the principle. So RUN = brew/scoop/winget/npm/mise/asdf/go-install (7); PRINT = aur/nix (2);
direct = ErrDirectSwap (1). This is the faithful reading. Document it explicitly (it's defensible: neither needs sudo).

## 3. The streaming exec seam — DISTINCT from detect.go's Runner (CRITICAL)

detect.go's `Runner.Run` CAPTURES stdout to a string (for parsing PM DB query output). Delegation needs the OPPOSITE:
STREAM the updater's stdout+stderr live to the terminal (FR-U4: "streaming its output"). So delegate.go defines a NEW
interface (different name — `Runner` is taken in the same package):
```go
type ExecRunner interface {
    Run(ctx context.Context, stdout, stderr io.Writer, name string, args ...string) (exitCode int, err error)
}
type osExecRunner struct{}                       // prod impl (exec.CommandContext + stream; ExitError→code)
```
osExecRunner.Run mirrors detect.go's osRunner exit-vs-start-failure distinction: `*exec.ExitError` → `(code, nil)`
(an exit is NOT an error — the updater ran and returned non-zero); a start/LookPath failure → `(0, err)` (the PM binary
is absent). Tests inject a fakeExecRunner that records calls + returns canned (code, err).

No Setpgid for the delegated updater (it's the USER's package manager, runs in the foreground group; Ctrl-C reaches it
naturally). The provider executor's Setpgid is for stagecoach's OWN child agents — different concern.

## 4. The Delegate API (the deliverable)

```go
var ErrDirectSwap = errors.New("upgrade: direct channel requires self-swap, not delegation")

type DelegateOptions struct {
    Exec      ExecRunner            // nil ⇒ osExecRunner (prod)
    Out       io.Writer             // stream target (run) + print destination (print); REQUIRED (nil ⇒ panic-guard or io.Discard)
    Env       func(string) string   // nil ⇒ npmVariant returns "npm"; used for PNPM_HOME/BUN_INSTALL detection
    Verbose   func(string)          // nil ⇒ no-op; logs the resolved command (keeps walled off — no internal/ui import)
    Confirmed bool                  // the command layer (P1.M4) owns the y/N prompt + --yes; Delegate never prompts
}

type DelegateResult struct {
    Ran      bool    // true = a real updater was executed; false = printed-only
    Command  string  // the argv joined (" && "-joined for asdf's 2 steps); the primary cmd for print channels
    ExitCode int     // run: the updater's exit code (0 success); print: 0
}

func Delegate(ctx context.Context, ch Channel, opts DelegateOptions) (DelegateResult, error)
```
- `direct` → `(DelegateResult{}, ErrDirectSwap)`. The command layer (P1.M4.T2) routes ErrDirectSwap to P1.M3 (the swap).
- run-channel → build argv(s), Verbose-log, Exec each in sequence (stop on first non-zero / err), return Ran=true + code.
- print-channel → write the exact command to opts.Out, return Ran=false + Command=primary + ExitCode=0.
- Delegate NEVER self-swaps (FR-U1) and NEVER gates on --force (the command layer decides force→swap, bypassing Delegate).

## 5. Per-channel argv (runArgv returns [][]string — a slice so asdf is a clean 2-step)

- brew:  `{{"brew","upgrade","stagecoach"}}`
- scoop: `{{"scoop","update","stagecoach"}}`
- winget:`{{"winget","upgrade","stagecoach"}}`
- mise:  `{{"mise","upgrade","stagecoach"}}`
- go-install: `{{"go","install","github.com/dabstractor/stagecoach/cmd/stagecoach@latest"}}`
- npm: `npmVariant(opts.Env)` ⇒
  - `"pnpm"` (PNPM_HOME set) → `{{"pnpm","add","-g","@dabstractor/stagecoach@latest"}}`
  - `"bun"` (BUN_INSTALL set) → `{{"bun","add","-g","@dabstractor/stagecoach@latest"}}`
  - `"npm"` (default) → `{{"npm","install","-g","@dabstractor/stagecoach@latest"}}`
  - (yarn deferred — berry removed `global`; the variant→argv map is trivially extensible. Document.)
- asdf: `{{"asdf","install","stagecoach","latest"},{"asdf","global","stagecoach","latest"}}` (run install then global;
  the "if it was global" condition simplified to "always set global" — `asdf global` is idempotent + safe for an upgrade;
  the user asked to update, so promoting to the global latest is the expected outcome. Document as a noted simplification.)

joinArgv(argvs) = strings.Join of each argv's space-joined form with " && " (so asdf → "asdf install stagecoach latest && asdf global stagecoach latest").

## 6. Print strings (printCommand)

- aur: print to opts.Out:
  ```
  sudo pacman -Syu stagecoach-bin
  # (or, with an AUR helper: yay -Syu stagecoach-bin)
  ```
  Command = "sudo pacman -Syu stagecoach-bin" (primary).
- nix: print to opts.Out:
  ```
  nix profile upgrade stagecoach
  # (declarative/flake users: run `nix flake update` in your config)
  ```
  Command = "nix profile upgrade stagecoach" (primary). Nix is immutable — no in-place swap (refuse = print only).

Print exits 0 (FR-U4: "A printed command exits 0 — here is how to update"). Ran=false.

## 7. Package conventions to mirror (releases.go / detect.go)

- `package upgrade`; FILE comment header (`// delegate.go implements…`), NOT a package doc — releases.go owns it.
- Injectable seam: ExecRunner field on DelegateOptions (nil ⇒ osExecRunner prod default), like Client's HTTP/BaseURL.
- Sentinel errors: `var ErrDirectSwap = errors.New("upgrade: …")` (mirrors ErrHTTP/ErrRateLimited).
- Stdlib-only: context, errors, fmt, io, os, os/exec, strings. NO internal/* imports (FR-U12 walled off). Verbose is an
  injected `func(string)`, NOT an internal/ui import.
- Tests in `package upgrade` (internal), table-driven, a fakeExecRunner recording calls + returning canned (code, err),
  bytes.Buffer for Out. Clone releases_test.go / detect_test.go's canned-fake idiom.

## 8. Scope fences (what Delegate does NOT do)

- NO detect logic (P1.M2.T1.S1 owns detect.go; Delegate CONSUMES the Channel).
- NO command layer / cobra / y/N prompt / --yes (P1.M4.T1/T2). Delegate takes a pre-confirmed bool and never prompts.
- NO --force gate (FR-U1's "force a self-swap of a manager-owned binary" is the command layer's call — it bypasses Delegate
  and invokes P1.M3 directly; Delegate never self-swaps and never sees --force).
- NO direct-binary swap / network / download (P1.M3 + releases.go + download.go). ErrDirectSwap hands off.
- NO exit-code mapping to 0/1/6 (the command layer P1.M4 maps DelegateResult + error to §15.4 codes; Delegate returns the
  raw updater exit code + Ran + ErrDirectSwap).
- NO edit to detect.go / releases.go / version.go / download.go / config / cmd / go.mod.

## 9. Test plan (clone the detect_test.go / releases_test.go canned-fake idiom)

- fakeExecRunner: records each Run call (name+args, stdout/stderr writers) + returns canned (code, err) from a queue/func.
- Tests (delegate_test.go, package upgrade):
  1. **per-channel argv** (table): for each RUN channel, inject fakeExecRunner returning (0,nil); assert the recorded argv
     equals the FR-U3 command (brew→["brew","upgrade","stagecoach"], …, go-install path, etc.).
  2. **run returns exit code**: fake returns (1,nil) → DelegateResult{Ran:true, ExitCode:1}, err==nil. Fake returns (0,nil)
     → Ran:true, ExitCode:0.
  3. **start failure**: fake returns (0, errSome) → DelegateResult{Ran:true}, err==errSome (wrapped or direct — document).
  4. **print channels**: aur/nix → opts.Out (bytes.Buffer) contains the printed command; Ran:false; ExitCode:0; Command=primary.
  5. **npm variant**: Env=PNPM_HOME-set → recorded argv is pnpm; BUN_INSTALL-set → bun; nil/empty Env → npm.
  6. **asdf 2-step**: 2 recorded Run calls (install then global), in order; stop on first non-zero (assert only install ran
     if install returns non-zero).
  7. **direct → ErrDirectSwap**: errors.Is(Delegate(ctx,ChannelDirect,opts)) == ErrDirectSwap; Exec.Run never called.
  8. **streaming**: fakeExecRunner writes to the passed stdout writer; assert opts.Out received the bytes (proves streaming,
     not capture).
  9. **never auto-sudo**: grep the source — no "sudo" in any RUN argv (only in the AUR PRINT string). Assert no run-channel
     argv starts with "sudo".