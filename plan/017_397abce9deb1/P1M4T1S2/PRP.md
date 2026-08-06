name: "P1.M4.T1.S2 — confirmUpgrade (current→target + action y/N) + --yes skip + non-TTY refuse + FR-U4/U7 run-vs-print output helpers (FR-U9)"
description: >
  The USER-FACING I/O HALF of `stagecoach upgrade` (PRD §9.29). A NEW `internal/cmd/upgrade_prompt.go`
  (package `cmd`, a SEPARATE file from S1's parallel `internal/cmd/upgrade.go` to avoid a same-file
  collision) provides THREE shared helpers the orchestrator (P1.M4.T2 `runUpgrade`) calls:
  (1) confirmUpgrade(current,target,action,assumeYes,in,out)(bool,error) — the FR-U9 y/N prompt that
  prints "stagecoach <current> → <target>\n<action>\nProceed? [y/N]", reads one line, accepts ONLY an
  explicit y/Y. --yes (assumeYes) skips it. A NON-TTY stdin WITHOUT --yes is REFUSED with an error
  (caller→exit 1) — stricter than integrate.DefaultConfirm's silent auto-decline (a declined file edit is
  reversible; a botched binary swap is not), matching the contract's "non-interactive-safety posture". A
  TTY 'n'/empty/anything-else → (false,nil) → caller exit 0 (intentional abort). (2) printDelegatedUpdate
  — the FR-U4 PRINT framing (AUR/Nix: stagecoach prints the exact updater command for the user to run,
  does not run it, exits 0). (3) printPrivilegeCommand — the FR-U7/FR-U4 needs-privileges framing (install
  path not writable → print the exact `sudo` re-run command; stagecoach NEVER auto-elevates; exits 0). Both
  print helpers echo the command VERBATIM (the raw string internal/upgrade already produced — Delegate's
  DelegateResult.Command / NeedsPrivilegesError.Command) and centralize the wording so runUpgrade doesn't
  inline it. Every environment seam is INJECTABLE: confirmUpgrade takes the stdin reader + stdout writer
  (testable with bytes.Buffer + strings.NewReader), and the TTY-ness is a package-level overridable
  predicate `confirmUpgradeIsTTY` cloned from config_init_interactive.go:20's interactiveStdinIsTTY. The
  error carries NO "stagecoach:" prefix (main.go:70 adds it — avoids the doubled-prefix bug). NO exitcode
  import in the helper (returns a plain error; the caller wraps via exitcode.New(exitcode.Error, err)).
  Tests (internal/cmd/upgrade_prompt_test.go, NEW): the 4 contract confirmUpgrade cases (assumeYes→true;
  non-TTY+no-yes→refuse; TTY 'y'/'Y'→true; TTY 'n'/empty→false) + the 2 print-helper framing tests; the
  tests reference ALL three helpers (they have no production caller until P1.M4.T2, so the tests are their
  only use — golangci-lint treats same-package test refs as uses). SCOPE: 2 NEW files only. ZERO edits to
  S1's upgrade.go/upgrade_test.go, internal/upgrade/* (P1.M4.T2/M3 own dispatch + primitives), config.Load,
  exitcode.go, docs/cli.md (S1 owns the Mode-A doc), the commit path (FR-U12), go.mod, any PRD/task file.
  Parallel-safe: S1 owns upgrade.go; P1.M4.T2 BUILDS ON this item (calls these helpers from its dispatch).

---

## Goal

**Feature Goal**: Provide the confirmation + run-vs-print output helpers for `stagecoach upgrade`
(FR-U9 confirmation; FR-U4/FR-U7 print framing) as testable, injectable shared functions in package
`cmd`, ready for the P1.M4.T2 dispatch (`runUpgrade`) to call before any on-disk change.

**Deliverable**:
1. `internal/cmd/upgrade_prompt.go` (NEW) — `confirmUpgrade(current, target, action string, assumeYes bool,
   in io.Reader, out io.Writer) (bool, error)` + the `confirmUpgradeIsTTY` package-var seam + the two
   print helpers `printDelegatedUpdate(out, channel, command)` and `printPrivilegeCommand(out, command)`.
2. `internal/cmd/upgrade_prompt_test.go` (NEW) — the 4 contract confirmUpgrade cases + the 2 print-helper
   tests.

**Success Definition**:
- `confirmUpgrade` returns the 4 contract outcomes exactly: assumeYes→(true,nil); non-TTY+no-yes→
  (false,err) whose message contains "re-run with --yes"; TTY 'y'/'Y'→(true,nil); TTY 'n'/empty/other→
  (false,nil). The TTY prompt prints "stagecoach <current> → <target>\n<action>\nProceed? [y/N] ".
- The non-TTY refusal prints NO "Proceed?" prompt (it refuses before prompting) and returns a PLAIN error
  with NO "stagecoach:" prefix (main.go adds it).
- `printDelegatedUpdate` and `printPrivilegeCommand` echo the command string verbatim with consistent
  FR-U4/FR-U7 framing; the caller returns nil (exit 0) after calling either.
- All seams are injectable: stdin via `in io.Reader`, stdout via `out io.Writer`, TTY-ness via the
  `confirmUpgradeIsTTY` package var (overridden + restored in tests).
- `go build ./...` clean; `go vet ./internal/cmd/...` clean; `gofmt -l` empty on the 2 new files;
  `go test ./internal/cmd/ -run TestConfirmUpgrade -v` + the print-helper tests green; `make test` +
  `make lint` clean (no U1000 — the tests reference all three helpers).
- `git status --porcelain` == the 2 new files ONLY. ZERO edits to S1's upgrade.go/upgrade_test.go,
  internal/upgrade/*, root.go, config.Load, exitcode.go, docs/cli.md, the commit path, go.mod, any PRD/task file.

## User Persona (if applicable)

**Target User**: A Stagecoach user running `stagecoach upgrade` interactively (sees the current→target +
action prompt, types y to proceed) OR in a script (passes `--yes`, or pipes stdin and gets a clean refusal
pointing at `--yes`). Also the CI/cron user who never hits the prompt (`--check` / a pure PRINT).
**Use Case**: confirm a binary self-swap or a delegated updater run before stagecoach changes anything
(FR-U9); when stagecoach cannot safely act (non-writable path, or a channel it won't run), get the exact
command to run yourself (FR-U4/FR-U7).
**User Journey**: `stagecoach upgrade` → (detect+resolve, P1.M4.T2) → `confirmUpgrade` prints
"stagecoach 1.2.3 → 1.3.0\nSelf-swap the direct-binary install.\nProceed? [y/N] " → user types `y` → swap
runs. OR non-TTY pipe → `stagecoach: non-interactive stdin — ... re-run with --yes ...` (exit 1). OR
`stagecoach upgrade` against `/usr/local/bin` → `printPrivilegeCommand`: "install path not writable ...
re-run with privileges: sudo ...". OR AUR channel → `printDelegatedUpdate`: "installed via aur — run its
updater yourself: sudo pacman -Syu stagecoach-bin".
**Pain Points Addressed**: FR-U9 (no silent binary change — explicit confirm; safe scripting via --yes;
safe non-interactive refusal); FR-U4 (never auto-sudo; always tell the user the exact command); FR-U7
(detect a non-writable path and print the re-run, never brick the install).

## Why

- **PRD §9.29 FR-U9**: the direct-swap path and any RUN-delegated-updater MUST prompt `y/N` before changing
  anything, printing current→target + the action; `--yes` skips it; `--check`/PRINT never prompt. This
  item IS that prompt.
- **Non-interactive safety (contract decision)**: a non-TTY stdin without `--yes` is REFUSED (exit 1), not
  silently auto-declined — because a botched binary swap is irreversible (unlike integrate's reversible file
  edits). This matches the codebase precedent (config init --interactive non-TTY → exit 1, config_init_interactive.go:47).
- **FR-U4/FR-U7 framing centralization**: the raw command strings already exist (internal/upgrade Delegate
  / NeedsPrivilegesError); this item provides the consistent user-facing wording so the P1.M4.T2 dispatch
  doesn't inline (and drift on) it.
- **Bounded + parallel-safe**: 2 new files in package `cmd`, separate from S1's `internal/cmd/upgrade.go`
  (S1 writes that now). P1.M4.T2 builds on these helpers (calls them from its dispatch). No collision.

## What

A pure, injectable `confirmUpgrade` prompt helper + two FR-U4/FR-U7 print-framing helpers in a new
`internal/cmd/upgrade_prompt.go`, with tests in `internal/cmd/upgrade_prompt_test.go`.

### Success Criteria
- [ ] `internal/cmd/upgrade_prompt.go` defines `confirmUpgrade(current, target, action string, assumeYes bool, in io.Reader, out io.Writer) (bool, error)` with the 4 contract outcomes.
- [ ] It defines `var confirmUpgradeIsTTY = func() bool { return ui.IsTerminal(os.Stdin) }` (the injectable seam, mirroring `interactiveStdinIsTTY`).
- [ ] Non-TTY + no `--yes` → `(false, fmt.Errorf("...re-run with --yes..."))` — a PLAIN error, NO "stagecoach:" prefix; NO "Proceed?" prompt is printed.
- [ ] TTY path prints `stagecoach <current> → <target>\n<action>\nProceed? [y/N] ` to `out`, reads one line from `in`, accepts only first byte `y`/`Y` → `(true, nil)`; everything else (n/N/empty/garbage) → `(false, nil)` + prints "stagecoach: upgrade aborted".
- [ ] `assumeYes` → `(true, nil)` with NO prompt printed.
- [ ] `printDelegatedUpdate(out io.Writer, channel, command string)` prints the FR-U4 PRINT framing + echoes `command` verbatim.
- [ ] `printPrivilegeCommand(out io.Writer, command string)` prints the FR-U7 needs-privileges framing + echoes `command` verbatim.
- [ ] `confirmUpgrade` does NOT import `internal/exitcode` (returns a plain error; caller wraps). It does NOT `os.Exit`.
- [ ] `internal/cmd/upgrade_prompt_test.go` covers: assumeYes→true; non-TTY+no-yes→refuse (err+message); TTY 'y'/'Y'→true; TTY 'n'/empty→false; + printDelegatedUpdate + printPrivilegeCommand framing. The isTTY seam is flipped + restored via defer.
- [ ] `go build ./...` clean; `go vet ./internal/cmd/...` clean; `gofmt -l` empty on the 2 new files.
- [ ] `go test ./internal/cmd/ -run 'TestConfirmUpgrade|TestPrint' -v` green; `make test` + `make lint` clean (no U1000).
- [ ] `git status --porcelain` == `internal/cmd/upgrade_prompt.go` + `internal/cmd/upgrade_prompt_test.go`.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the 3 codebase precedents (the isTTY seam to clone, the y/N prompt to reuse, the test idiom), the
exact 4-outcome contract table, the verbatim prompt/error formats, the no-"stagecoach:"-prefix rule (main.go:70
adds it), the verbatim print-helper framing, the consumed APIs (ui.IsTerminal, exitcode.New/Error for the
CALLER), the U1000-dodge (tests reference all 3 helpers), the parallel-S1 separate-file rationale, and the
grep guards.

### Documentation & References

```yaml
# MUST READ — the codebase-specific findings (the design + the 3 precedents + the verbatim strings).
- docfile: plan/017_397abce9deb1/P1M4T1S2/research/findings.md
  why: "§1 the 3 precedents (interactiveStdinIsTTY seam @config_init_interactive.go:20; DefaultConfirm y/N
        @integrate/protocol.go:240); §2 FR-U9 spec + the 4-outcome table + the NO-'stagecoach:'-prefix rule
        (main.go:70); §3 the print helpers + that internal/upgrade ALREADY produces the raw command strings
        (Delegate.printCommand / NeedsPrivilegesError.Command); §4 the separate-file rationale (S1 collision);
        §5 the U1000 dodge; §6 the test design; §7 consumed APIs; §8 scope fences."

# MUST READ — the isTTY seam to CLONE (the injectable package-var predicate).
- file: internal/cmd/config_init_interactive.go
  why: "Line 20: `var interactiveStdinIsTTY = func() bool { return ui.IsTerminal(os.Stdin) }` — clone this
        EXACTLY as `confirmUpgradeIsTTY`. Line 98 `runInteractiveWizard(r io.Reader, w io.Writer, ...)` — the
        pure-io.Reader/io.Writer idiom (no os.Stdin/IsTerminal inside) that confirmUpgrade follows. Line 47 —
        the non-TTY refusal returns exitcode.New(exitcode.Error, fmt.Errorf(...)) with NO 'stagecoach:' prefix
        (the precedent for the refusal's error shape, though that one is in a RunE; confirmUpgrade returns a
        PLAIN error and the caller wraps)."
  pattern: "package-var `var xIsTTY = func() bool { return ui.IsTerminal(os.Stdin) }`; pure helper takes
            io.Reader/io.Writer; TTY gate consults the seam."
  gotcha: "The seam is read INSIDE confirmUpgrade (the contract signature has no isTTY param). Tests flip the
           package var + defer-restore. Do NOT add an isTTY parameter (deviates from the contract signature)."

# MUST READ — the y/N prompt to REUSE (accept-only-y/Y rule + read-one-line idiom).
- file: internal/integrate/protocol.go
  why: "DefaultConfirm (line 240): the y/N precedent — accept ONLY a line whose first non-space byte is
        'y'/'Y'; everything else ⇒ decline. confirmUpgrade reuses this accept rule but is STRICTER on non-TTY
        (refuse+error vs DefaultConfirm's silent auto-decline). Shows the `fmt.Fscanln` read idiom (confirmUpgrade
        uses bufio.NewReader(r).ReadString('\n') for the io.Reader seam — more robust than Fscanln on a reader)."
  pattern: "trim the line; `len(line) > 0 && (line[0]=='y' || line[0]=='Y')` ⇒ accept."
  gotcha: "DefaultConfirm auto-declines on non-TTY (returns false). confirmUpgrade REFUSES on non-TTY (returns
           (false, error)) — do NOT copy DefaultConfirm's non-TTY behavior; the contract explicitly chose refusal."

# MUST READ — main.go's error-printing (the NO-'stagecoach:'-prefix rule).
- file: cmd/stagecoach/main.go
  why: "Line 70: `fmt.Fprintf(os.Stderr, \"stagecoach: %v\n\", err)` — main ALWAYS prepends 'stagecoach: '.
        Therefore confirmUpgrade's refusal error MUST NOT include 'stagecoach:' (else double prefix, cf. the
        doubled-prefix bug pattern). The codebase convention (config.go:162 etc.) is RunE errors carry no
        'stagecoach:' prefix."
  critical: "Return a PLAIN fmt.Errorf(\"...\") from confirmUpgrade (no prefix). The caller wraps
             exitcode.New(exitcode.Error, err) WITHOUT re-adding 'stagecoach:'."

# MUST READ — ui.IsTerminal (the TTY probe the seam calls; NO golang.org/x/term dep).
- file: internal/ui/output.go
  why: "Line 32: `func IsTerminal(f *os.File) bool` — the platform-delegated isatty (linux ioctl/darwin/
        windows GetConsoleMode/other char-device fallback). go.mod has NO golang.org/x/term — use ui.IsTerminal,
        do NOT add a new dependency."
  pattern: "`var confirmUpgradeIsTTY = func() bool { return ui.IsTerminal(os.Stdin) }`."

# CONTEXT — the print helpers' INPUTS (the raw command strings internal/upgrade produces; for framing accuracy).
- file: internal/upgrade/delegate.go
  why: "printCommand(ch) (AUR/Nix) + DelegateResult.Command (the PRIMARY printed command). printDelegatedUpdate
        echoes this verbatim. Shows FR-U4 'a printed command exits 0' + 'never auto-sudo' (sudo appears ONLY in
        the AUR print string the USER runs)."
- file: internal/upgrade/swap.go
  why: "NeedsPrivilegesError{Command: ...} (FR-U7 re-run form; swap_unix.go privilegeCommand → `sudo \"<exe>\" upgrade`).
        printPrivilegeCommand echoes .Command verbatim. FR-U4 'never auto-elevate'."

# CONTEXT — exitcode (the CALLER's wrap; confirmUpgrade itself does NOT import it).
- file: internal/exitcode/exitcode.go
  why: "exitcode.Error=1 (line 27), exitcode.New(code, err) (line 58). The P1.M4.T2 caller does
        `exitcode.New(exitcode.Error, err)` on confirmUpgrade's refusal error. confirmUpgrade returns a PLAIN
        error so it stays a pure, dependency-light helper (no exitcode import)."

# MUST READ — the FR-U9/U4/U7 spec (the prompt + run-vs-print contract).
- docfile: plan/017_397abce9deb1/prd_snapshot.md
  section: "§9.29 FR-U9 (line 622), FR-U4 (line 607), FR-U7 (line 618-620); §15.4 exit codes"
  why: "FR-U9 pins the prompt (current→target + action, y/N, --yes skip, --check/PRINT never prompt). FR-U4
        pins the run-vs-print policy (PRINT exits 0; never auto-sudo). FR-U7 pins the non-writable → print
        sudo re-run. Exit 0 (up-to-date/upgraded/printed); 1 (failure); 6 (--check behind)."

# CONTEXT — the sibling S1 PRP (what runUpgrade looks like when P1.M4.T2 wires these helpers in).
- docfile: plan/017_397abce9deb1/P1M4T1S1/PRP.md
  why: "S1 owns upgrade.go's `runUpgrade` (a PLACEHOLDER pre-P1.M4.T2) + the `flagYes` var. P1.M4.T2's dispatch
        will call `confirmUpgrade(current, target, action, flagYes, cmd.InOrStdin(), cmd.OutOrStdout())` and
        `printDelegatedUpdate` / `printPrivilegeCommand`. This item PROVIDES those helpers; it does NOT edit
        S1's upgrade.go. `flagYes` is the `--yes/-y` source (FR-U9)."
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/
  config_init_interactive.go  # READ-ONLY — the interactiveStdinIsTTY seam + pure-wizard idiom (TEMPLATE)
  upgrade.go                  # S1's file (PARALLEL) — upgradeCmd + runUpgrade (placeholder) + flagYes; DO NOT EDIT
  upgrade_prompt.go           # NEW — confirmUpgrade + confirmUpgradeIsTTY + printDelegatedUpdate + printPrivilegeCommand
  upgrade_prompt_test.go      # NEW — the 4 confirmUpgrade cases + 2 print-helper tests
  root.go                     # READ-ONLY — rootCmd; ZERO edits
internal/integrate/protocol.go  # READ-ONLY — DefaultConfirm y/N precedent
internal/ui/output.go        # READ-ONLY — IsTerminal (the TTY probe the seam calls)
internal/upgrade/
  delegate.go                 # READ-ONLY — Delegate/printCommand/DelegateResult.Command (print-helper input)
  swap.go, swap_unix.go       # READ-ONLY — NeedsPrivilegesError.Command / privilegeCommand (print-helper input)
internal/exitcode/exitcode.go # READ-ONLY — Error=1/New (the CALLER's wrap; helper does not import)
cmd/stagecoach/main.go        # READ-ONLY — line 70 prepends "stagecoach: " (the no-double-prefix rule)
docs/cli.md                   # READ-ONLY — S1 owns the Mode-A `### upgrade` edit; DO NOT EDIT
go.mod                        # READ-ONLY — UNCHANGED (no new dep; ui.IsTerminal is in-repo)
```

### Desired Codebase tree with files to be added/modified

```bash
internal/cmd/upgrade_prompt.go       # NEW — confirmUpgrade + confirmUpgradeIsTTY + printDelegatedUpdate + printPrivilegeCommand
internal/cmd/upgrade_prompt_test.go  # NEW — TestConfirmUpgrade_{AssumeYes,NonTTYRefuses,TTYYes,TTYDeclines} + TestPrint{DelegatedUpdate,PrivilegeCommand}
# NOTHING ELSE. No edit to upgrade.go/upgrade_test.go (S1), internal/upgrade/* (P1.M4.T2/M3), root.go,
# config.Load, exitcode.go, docs/cli.md (S1), the commit path, go.mod, any PRD/task file.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (NO "stagecoach:" prefix in confirmUpgrade's error): main.go:70 ALWAYS prepends "stagecoach: ".
// The codebase convention (config.go:162, config_init_interactive.go:47) is RunE/returned errors carry NO
// "stagecoach:" prefix. Return a PLAIN fmt.Errorf("...re-run with --yes...") — the caller wraps
// exitcode.New(exitcode.Error, err) WITHOUT re-adding "stagecoach:". Else the doubled-prefix bug.

// CRITICAL (non-TTY REFUSES, does NOT auto-decline): unlike integrate/protocol.go DefaultConfirm (which
// returns false silently on non-TTY), confirmUpgrade returns (false, error) on non-TTY+no-yes. A declined
// file edit is reversible; a botched binary swap is not. Do NOT copy DefaultConfirm's non-TTY behavior.

// CRITICAL (accept ONLY explicit y/Y; everything else ⇒ abort exit 0): trim the line; the FIRST byte must
// be 'y' or 'Y'. Empty line, 'n', 'N', garbage, EOF ⇒ (false, nil) — intentional abort, exit 0 (NOT an
// error). Only the non-TTY refusal is an error (exit 1). Keep the two outcomes straight.

// CRITICAL (the isTTY seam is a package VAR, not a parameter): the contract signature is
// confirmUpgrade(current,target,action,assumeYes,in,out) — NO isTTY param. Clone interactiveStdinIsTTY as
// confirmUpgradeIsTTY and read it INSIDE confirmUpgrade. Tests flip the var + defer-restore. Do NOT add an
// isTTY func parameter (deviates from the contract).

// GOTCHA (confirmUpgrade must NOT import internal/exitcode): it returns a PLAIN error so it stays a pure,
// dependency-light helper. The CALLER (P1.M4.T2) wraps exitcode.New(exitcode.Error, err). Importing exitcode
// here would couple a pure prompt helper to the exit-code layer unnecessarily.

// GOTCHA (U1000/unused linter — tests are the helpers' only use pre-P1.M4.T2): golangci-lint enables
// staticcheck+unused (`.golangci.yml`). Pre-P1.M4.T2 these unexported helpers have NO production caller.
// golangci-lint treats SAME-PACKAGE _test.go references as uses, so the tests MUST call all three
// (confirmUpgrade + printDelegatedUpdate + printPrivilegeCommand). If you forget to test one, `make lint`
// fails with U1000. (Do NOT export the names just to dodge this — the contract pins lowercase confirmUpgrade.)

// GOTCHA (read the line with bufio, not fmt.Fscanln): confirmUpgrade takes an io.Reader (the seam). Use
// bufio.NewReader(in).ReadString('\n') (the config_init_interactive.go wizard idiom), not fmt.Fscanln(in,...)
// (DefaultConfirm uses Fscanln on os.Stdin directly; the io.Reader seam prefers bufio). EOF ⇒ empty ⇒ decline.

// GOTCHA (print helpers echo the command VERBATIM): printDelegatedUpdate/printPrivilegeCommand receive the
// exact command string internal/upgrade produced (DelegateResult.Command / NeedsPrivilegesError.Command) and
// must NOT mutate/quote/re-shell it (the user pastes it). They only ADD the framing line(s).

// GOTCHA (do NOT prompt on --check or a pure PRINT — but that is the CALLER's job): confirmUpgrade is a pure
// prompt; it does not know about --check/PRINT. P1.M4.T2's runUpgrade decides NOT to call confirmUpgrade on
// those paths. Do not add a --check/PRINT branch inside confirmUpgrade.

// GOTCHA (separate file from S1's upgrade.go): S1 is writing internal/cmd/upgrade.go in parallel. Put these
// helpers in internal/cmd/upgrade_prompt.go (same package cmd, no name collision with S1's
// upgradeCmd/runUpgrade/validateUpgradeFlags/flagX). Do NOT edit S1's upgrade.go.
```

## Implementation Blueprint

### Data models and structure

None beyond the helper function signatures + the one package-var seam. No new types/structs, no new config
fields, no new exit codes, no new deps (bufio/fmt/io/os/strings + internal/ui are all already used by the
package or stdlib). The `(bool, error)` return is the only "model".

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/cmd/upgrade_prompt.go — confirmUpgrade + confirmUpgradeIsTTY + 2 print helpers
  - PACKAGE DOC: explain this is the FR-U9 confirmation + FR-U4/FR-U7 run-vs-print OUTPUT FORMATTING for
    `stagecoach upgrade` — the user-facing I/O half; the dispatch (P1.M4.T2 runUpgrade) calls these helpers
    before any on-disk change. Note: confirmUpgrade is STRICTER than integrate.DefaultConfirm (non-TTY ⇒
    refuse, not auto-decline — a binary swap is irreversible); it never prompts for --check/PRINT (the caller
    simply doesn't call it); every seam is injectable (io.Reader/io.Writer + the confirmUpgradeIsTTY seam);
    walled off (FR-U12). File comment only.
  - IMPORTS: bufio, fmt, io, os, strings, github.com/dabstractor/stagecoach/internal/ui.
    (NO internal/exitcode — confirmUpgrade returns a plain error; the caller wraps. NO internal/upgrade —
    the print helpers take a STRING command, staying decoupled. NO cobra/pflag — pure helpers.)
  - THE SEAM (clone interactiveStdinIsTTY @config_init_interactive.go:20):
      // confirmUpgradeIsTTY is the TTY gate for confirmUpgrade. Overridable in tests so the non-TTY refusal
      // and the TTY prompt paths are exercisable without a real terminal (mirrors interactiveStdinIsTTY in
      // config_init_interactive.go). Production defaults to ui.IsTerminal(os.Stdin).
      var confirmUpgradeIsTTY = func() bool { return ui.IsTerminal(os.Stdin) }
  - confirmUpgrade(current, target, action string, assumeYes bool, in io.Reader, out io.Writer) (bool, error):
      func confirmUpgrade(current, target, action string, assumeYes bool, in io.Reader, out io.Writer) (bool, error) {
          if assumeYes {
              return true, nil
          }
          if !confirmUpgradeIsTTY() {
              // Non-interactive + no --yes ⇒ refuse (never silently swap a binary in a piped/scripted run).
              // --yes is the explicit script bypass. Stricter than integrate.DefaultConfirm's auto-decline:
              // a declined file edit is reversible; a botched binary swap is not. NO "stagecoach:" prefix
              // (main.go adds it); the caller wraps via exitcode.New(exitcode.Error, err) ⇒ exit 1.
              return false, fmt.Errorf("non-interactive stdin — refusing to upgrade without confirmation; re-run with --yes to confirm non-interactively")
          }
          fmt.Fprintf(out, "stagecoach %s → %s\n%s\nProceed? [y/N] ", current, target, action)
          br := bufio.NewReader(in)
          line, _ := br.ReadString('\n') // EOF/empty ⇒ decline (best-effort; mirrors DefaultConfirm)
          line = strings.TrimSpace(line)
          if len(line) > 0 && (line[0] == 'y' || line[0] == 'Y') {
              return true, nil
          }
          fmt.Fprintln(out, "stagecoach: upgrade aborted")
          return false, nil
      }
  - printDelegatedUpdate(out io.Writer, channel, command string) — FR-U4 PRINT framing:
      // printDelegatedUpdate frames the FR-U4 PRINT-channel output: stagecoach detected the install channel
      // but will NOT run the updater itself (AUR needs root; Nix is declarative/immutable), so it prints the
      // exact command for the user to run and exits 0 (the caller returns nil). command is the verbatim
      // command internal/upgrade.Delegate produced (DelegateResult.Command); channel is the friendly channel
      // label (e.g. "aur", "nix").
      func printDelegatedUpdate(out io.Writer, channel, command string) {
          fmt.Fprintf(out, "stagecoach: installed via %s — run its updater yourself (stagecoach does not run it for you):\n  %s\n", channel, command)
      }
  - printPrivilegeCommand(out io.Writer, command string) — FR-U7/FR-U4 needs-privileges framing:
      // printPrivilegeCommand frames the FR-U7 needs-privileges output: the install path is not writable by
      // the current user, so stagecoach left everything untouched and prints the exact elevated re-run command
      // (it NEVER auto-sudo/auto-elevates — FR-U4). command is the verbatim command from
      // NeedsPrivilegesError.Command. The caller returns nil (exit 0).
      func printPrivilegeCommand(out io.Writer, command string) {
          fmt.Fprintf(out, "stagecoach: install path not writable by the current user; re-run with privileges (stagecoach never auto-elevates):\n  %s\n", command)
      }
  - NAMING: confirmUpgrade, confirmUpgradeIsTTY, printDelegatedUpdate, printPrivilegeCommand (all unexported,
    same-package helpers for P1.M4.T2; the contract pins lowercase confirmUpgrade).
  - GOTCHA: the refusal error has NO "stagecoach:" prefix; the "stagecoach: upgrade aborted" NOTICE on decline
    DOES (it is a user-facing stdout line, not a returned error — main does not process it). Keep them straight.

Task 2: CREATE internal/cmd/upgrade_prompt_test.go — the 4 confirmUpgrade cases + 2 print-helper tests
  - PACKAGE: `package cmd` (white-box — references confirmUpgrade/confirmUpgradeIsTTY/print* directly).
    IMPORTS: bytes, strings, testing.
  - Helper: a save/restore for the seam (avoid leaking into sibling tests):
      func setTTY(t *testing.T, isTTY bool) {
          t.Helper()
          orig := confirmUpgradeIsTTY
          confirmUpgradeIsTTY = func() bool { return isTTY }
          t.Cleanup(func() { confirmUpgradeIsTTY = orig })
      }
  - TestConfirmUpgrade_AssumeYes:
      var out bytes.Buffer
      ok, err := confirmUpgrade("1.2.3", "1.3.0", "Self-swap the direct-binary install.", true, strings.NewReader(""), &out)
      // expect (true, nil); out has NO "Proceed?" (no prompt printed).
      if !ok || err != nil { t.Errorf("assumeYes: want (true,nil), got (%v,%v)", ok, err) }
      if strings.Contains(out.String(), "Proceed?") { t.Errorf("assumeYes must not prompt; got %q", out.String()) }
  - TestConfirmUpgrade_NonTTYRefuses:
      setTTY(t, false)
      var out bytes.Buffer
      ok, err := confirmUpgrade("1.2.3", "1.3.0", "Self-swap.", false, strings.NewReader("y\n"), &out)
      // expect (false, err); err message has "re-run with --yes"; NO "stagecoach:" prefix; NO "Proceed?".
      if ok { t.Errorf("non-TTY: want ok=false, got true") }
      if err == nil { t.Errorf("non-TTY: want an error, got nil") }
      if err != nil && !strings.Contains(err.Error(), "re-run with --yes") { t.Errorf("non-TTY err missing --yes hint: %v", err) }
      if err != nil && strings.Contains(err.Error(), "stagecoach:") { t.Errorf("non-TTY err must NOT have 'stagecoach:' prefix (main adds it): %v", err) }
      if strings.Contains(out.String(), "Proceed?") { t.Errorf("non-TTY must not print the prompt; got %q", out.String()) }
  - TestConfirmUpgrade_TTYYes (covers lowercase + uppercase):
      setTTY(t, true)
      for _, input := range []string{"y\n", "Y\n", "yes\n", "yeah\n"} {
          var out bytes.Buffer
          ok, err := confirmUpgrade("1.2.3", "1.3.0", "Self-swap.", false, strings.NewReader(input), &out)
          if !ok || err != nil { t.Errorf("input %q: want (true,nil), got (%v,%v)", input, ok, err) }
          if !strings.Contains(out.String(), "1.2.3 → 1.3.0") || !strings.Contains(out.String(), "Proceed? [y/N]") {
              t.Errorf("input %q: prompt missing current→target/Proceed; got %q", input, out.String())
          }
      }
  - TestConfirmUpgrade_TTYDeclines (covers n/empty/garbage):
      setTTY(t, true)
      for _, input := range []string{"n\n", "N\n", "\n", "no\n", "asdf\n"} {
          var out bytes.Buffer
          ok, err := confirmUpgrade("1.2.3", "1.3.0", "Self-swap.", false, strings.NewReader(input), &out)
          if ok || err != nil { t.Errorf("input %q: want (false,nil), got (%v,%v)", input, ok, err) }
          if !strings.Contains(out.String(), "aborted") { t.Errorf("input %q: want 'aborted' notice; got %q", input, out.String()) }
      }
  - TestPrintDelegatedUpdate:
      var out bytes.Buffer
      printDelegatedUpdate(&out, "aur", "sudo pacman -Syu stagecoach-bin")
      s := out.String()
      // framing + verbatim command + "does not run":
      if !strings.Contains(s, "aur") { t.Errorf("missing channel") }
      if !strings.Contains(s, "sudo pacman -Syu stagecoach-bin") { t.Errorf("command not echoed verbatim: %q", s) }
      if !strings.Contains(s, "does not run") { t.Errorf("missing FR-U4 'does not run' framing: %q", s) }
  - TestPrintPrivilegeCommand:
      var out bytes.Buffer
      printPrivilegeCommand(&out, `sudo "/usr/local/bin/stagecoach" upgrade`)
      s := out.String()
      if !strings.Contains(s, `sudo "/usr/local/bin/stagecoach" upgrade`) { t.Errorf("command not echoed verbatim: %q", s) }
      if !strings.Contains(s, "not writable") { t.Errorf("missing 'not writable' framing: %q", s) }
      if !strings.Contains(s, "never auto-elevates") { t.Errorf("missing FR-U4 'never auto-elevates' framing: %q", s) }
  - FOLLOW pattern: pure-function tests (no rootCmd state). The seam flip via setTTY(t, bool) + t.Cleanup
    restore (mirrors the package-var override idiom). All 6 tests reference the 3 helpers (the U1000 dodge).
  - GOTCHA: the loop in TestConfirmUpgrade_TTYYes/Declines uses a FRESH bytes.Buffer per input (so out doesn't
    accumulate across cases). EOF from a short reader (e.g. strings.NewReader("y") with no newline) ⇒
    ReadString returns ("y", io.EOF) ⇒ trimmed "y" ⇒ accept — that's fine (best-effort read).

Task 3: VERIFY — build, vet, format, tests, lint, grep guards
  - go build ./... ; go vet ./internal/cmd/... ; gofmt -l internal/cmd/upgrade_prompt.go internal/cmd/upgrade_prompt_test.go  # empty
  - go test ./internal/cmd/ -run 'TestConfirmUpgrade|TestPrint' -v   # the 6 new tests
  - go test ./internal/cmd/ -race                                  # full cmd regression (no sibling test broke)
  - make test && make lint                                         # golangci-lint: errcheck/gosimple/govet/ineffassign/staticcheck/unused
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN (the injectable isTTY seam — clone of config_init_interactive.go:20):
var confirmUpgradeIsTTY = func() bool { return ui.IsTerminal(os.Stdin) }

// PATTERN (confirmUpgrade — pure, 4 contract outcomes, no exitcode import, no "stagecoach:" prefix):
func confirmUpgrade(current, target, action string, assumeYes bool, in io.Reader, out io.Writer) (bool, error) {
	if assumeYes {
		return true, nil
	}
	if !confirmUpgradeIsTTY() {
		return false, fmt.Errorf("non-interactive stdin — refusing to upgrade without confirmation; re-run with --yes to confirm non-interactively")
	}
	fmt.Fprintf(out, "stagecoach %s → %s\n%s\nProceed? [y/N] ", current, target, action)
	br := bufio.NewReader(in)
	line, _ := br.ReadString('\n')
	line = strings.TrimSpace(line)
	if len(line) > 0 && (line[0] == 'y' || line[0] == 'Y') {
		return true, nil
	}
	fmt.Fprintln(out, "stagecoach: upgrade aborted")
	return false, nil
}

// PATTERN (print helpers — framing + verbatim command; caller returns nil ⇒ exit 0):
func printDelegatedUpdate(out io.Writer, channel, command string) {
	fmt.Fprintf(out, "stagecoach: installed via %s — run its updater yourself (stagecoach does not run it for you):\n  %s\n", channel, command)
}
func printPrivilegeCommand(out io.Writer, command string) {
	fmt.Fprintf(out, "stagecoach: install path not writable by the current user; re-run with privileges (stagecoach never auto-elevates):\n  %s\n", command)
}

// PATTERN (test seam flip + restore):
func setTTY(t *testing.T, isTTY bool) {
	t.Helper()
	orig := confirmUpgradeIsTTY
	confirmUpgradeIsTTY = func() bool { return isTTY }
	t.Cleanup(func() { confirmUpgradeIsTTY = orig })
}
```

### Integration Points

```yaml
CLI SURFACE (consumed by P1.M4.T2 runUpgrade — NOT this item):
  - P1.M4.T2 calls: ok, err := confirmUpgrade(current, target, action, flagYes, cmd.InOrStdin(), cmd.OutOrStdout())
    if err != nil { return exitcode.New(exitcode.Error, err) }   // non-TTY refusal ⇒ exit 1
    if !ok { return nil }                                        // TTY decline ⇒ exit 0 (intentional)
    // …proceed with the swap/delegation…
  - On a Delegate PRINT result (Ran:false): printDelegatedUpgrade(cmd.OutOrStdout(), channel, res.Command); return nil  // exit 0
  - On NeedsPrivilegesError: printPrivilegeCommand(cmd.OutOrStdout(), npe.Command); return nil  // exit 0
  - confirmUpgrade is NOT called on --check or a pure PRINT (FR-U9).
CONSUMES (LANDED — read-only):
  - ui.IsTerminal(os.Stdin) (internal/ui/output.go:32) — the TTY probe in the seam. NO new dep (go.mod unchanged).
  - exitcode.New/exitcode.Error (the CALLER; confirmUpgrade does not import exitcode).
  - DelegateResult.Command / NeedsPrivilegesError.Command (internal/upgrade — the print helpers' `command` arg source).
NO database / migration / routes / config-struct change / exitcode-const change / go.mod change / docs change.
SCOPE FENCES:
  - Touches ONLY: internal/cmd/upgrade_prompt.go (NEW), internal/cmd/upgrade_prompt_test.go (NEW).
  - Does NOT edit: internal/cmd/upgrade.go or upgrade_test.go (S1 — parallel), root.go, internal/upgrade/*
    (P1.M4.T2/M3 own dispatch + primitives), config.Load, exitcode.go, docs/cli.md (S1 owns the Mode-A edit),
    the commit path (FR-U12), go.mod, any PRD/task file.
  - confirmUpgrade NEVER os.Exit (returns bool/error; the caller routes exit codes). NEVER prompts on
    --check/PRINT (the caller decides not to call it).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build (bufio/fmt/io/os/strings + internal/ui all already used/available; no new dep).
go build ./...
# Expected: clean. (A failure on "imported and not used: os" ⇒ remove the unused import; "confirmUpgradeIsTTY
#           declared and not used" ⇒ the seam IS used inside confirmUpgrade — confirm the reference is present.)

# Vet the changed package.
go vet ./internal/cmd/...
# Expected: clean.

# Format the 2 new files.
gofmt -l internal/cmd/upgrade_prompt.go internal/cmd/upgrade_prompt_test.go
# Expected: empty. If listed: gofmt -w internal/cmd/upgrade_prompt.go internal/cmd/upgrade_prompt_test.go

# Lint (errcheck/gosimple/govet/ineffassign/staticcheck/unused). The helpers are unexported + have no
# production caller pre-P1.M4.T2 — the tests are their ONLY use. golangci-lint treats same-package _test.go
# references as uses, so the 6 tests referencing all 3 helpers keeps `unused` (U1000) green.
make lint
# Expected: zero errors. If U1000 fires on a helper ⇒ its test is missing a call — add it (do NOT export).

# Scope guard: ONLY the 2 new files.
git status --porcelain
# Expected: internal/cmd/upgrade_prompt.go + internal/cmd/upgrade_prompt_test.go ONLY.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The 6 new tests (4 confirmUpgrade + 2 print helpers).
go test ./internal/cmd/ -run 'TestConfirmUpgrade|TestPrint' -v
# Expected: ALL PASS —
#   TestConfirmUpgrade_AssumeYes: (true,nil); no prompt.
#   TestConfirmUpgrade_NonTTYRefuses: (false,err) w/ "re-run with --yes", no "stagecoach:" prefix, no prompt.
#   TestConfirmUpgrade_TTYYes: y/Y/yes/yeah ⇒ (true,nil); prompt has "current → target" + "Proceed? [y/N]".
#   TestConfirmUpgrade_TTYDeclines: n/N/empty/no/asdf ⇒ (false,nil); "aborted" notice.
#   TestPrintDelegatedUpdate: framing + verbatim command + "does not run".
#   TestPrintPrivilegeCommand: framing + verbatim command + "not writable" + "never auto-elevates".

# Full cmd-package regression (the new helpers + seam don't disturb root's flag set / sibling commands; the
# seam flip is restored via t.Cleanup so it never leaks into a sibling test).
go test ./internal/cmd/ -race
# Expected: green.

# Full race suite + lint + build.
make test && make lint && make build
# Expected: all green.
```

### Level 3: Integration Testing (System Validation)

```bash
# N/A for a pure helper with no caller yet — confirmUpgrade/print* are exercised by their unit tests. There
# is no command path to run end-to-end until P1.M4.T2 wires the dispatch. (P1.M4.T3.S1/S2/S3 are the e2e
# self-update harness that exercises confirmUpgrade via the real `stagecoach upgrade` flow.)
# Sanity: the helpers COMPILE into the binary (no link error).
make build
# Expected: clean.
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: confirmUpgrade signature matches the contract EXACTLY.
grep -n 'func confirmUpgrade(current, target, action string, assumeYes bool, in io.Reader, out io.Writer) (bool, error)' internal/cmd/upgrade_prompt.go
# Expected: 1 hit.

# Guard 2: the isTTY seam is the interactiveStdinIsTTY clone (package var, not a param).
grep -n 'var confirmUpgradeIsTTY = func() bool { return ui.IsTerminal(os.Stdin) }' internal/cmd/upgrade_prompt.go  # 1 hit

# Guard 3: confirmUpgrade does NOT import internal/exitcode (pure helper; caller wraps).
! grep -q 'internal/exitcode' internal/cmd/upgrade_prompt.go && echo "OK: no exitcode import" || echo "FAIL: exitcode imported"

# Guard 4: the refusal error has NO "stagecoach:" prefix (main.go:70 adds it — avoid double prefix).
grep -n 're-run with --yes' internal/cmd/upgrade_prompt.go | grep -v 'stagecoach:'   # the error line must NOT contain "stagecoach:"

# Guard 5: confirmUpgrade NEVER os.Exit.
! grep -q 'os.Exit' internal/cmd/upgrade_prompt.go && echo "OK: no os.Exit" || echo "FAIL: os.Exit present"

# Guard 6: both print helpers echo the command verbatim (use the %s arg, no mutation/quoting of `command`).
grep -n 'func printDelegatedUpdate' internal/cmd/upgrade_prompt.go   # 1 hit
grep -n 'func printPrivilegeCommand' internal/cmd/upgrade_prompt.go  # 1 hit

# Guard 7: the prompt format matches the contract (current → target + action + Proceed? [y/N]).
grep -n 'stagecoach %s → %s' internal/cmd/upgrade_prompt.go          # 1 hit (the prompt)
grep -n 'Proceed? \[y/N\]' internal/cmd/upgrade_prompt.go            # 1 hit

# Guard 8: the tests reference ALL 3 helpers (the U1000 dodge pre-P1.M4.T2).
grep -c 'confirmUpgrade(' internal/cmd/upgrade_prompt_test.go        # ≥4 (the 4 cases)
grep -c 'printDelegatedUpdate(' internal/cmd/upgrade_prompt_test.go  # ≥1
grep -c 'printPrivilegeCommand(' internal/cmd/upgrade_prompt_test.go # ≥1

# Guard 9: scope — ONLY the 2 new files; S1's upgrade.go UNTOUCHED.
git status --porcelain
# Expected: internal/cmd/upgrade_prompt.go + internal/cmd/upgrade_prompt_test.go ONLY.
git diff --name-only | grep -vE '^internal/cmd/upgrade_prompt(\.go|_test\.go)$' && echo "FAIL: out-of-scope file" || echo "OK: scope clean"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./internal/cmd/...` clean; `gofmt -l` empty on the 2 new files
- [ ] `make lint` zero errors (no U1000 — all 3 helpers referenced by the 6 tests)
- [ ] `go test ./internal/cmd/ -run 'TestConfirmUpgrade|TestPrint' -v` green (6 tests)
- [ ] `go test ./internal/cmd/ -race` green (seam flip restored via t.Cleanup; no sibling test broke)
- [ ] `make test` + `make build` clean

### Feature Validation (the contract's 4 outcomes + 2 helpers)
- [ ] confirmUpgrade assumeYes ⇒ (true,nil), no prompt (grep guard 7: no "Proceed?" when assumeYes)
- [ ] confirmUpgrade non-TTY+no-yes ⇒ (false,err) w/ "re-run with --yes", no "stagecoach:" prefix, no prompt
- [ ] confirmUpgrade TTY 'y'/'Y' ⇒ (true,nil); prompt prints "current → target" + action + "Proceed? [y/N]"
- [ ] confirmUpgrade TTY 'n'/empty/other ⇒ (false,nil) + "aborted" notice
- [ ] printDelegatedUpdate ⇒ FR-U4 framing + verbatim command + "does not run"
- [ ] printPrivilegeCommand ⇒ FR-U7 framing + verbatim command + "not writable" + "never auto-elevates"

### Scope-Boundary Validation
- [ ] `git status` shows ONLY `internal/cmd/upgrade_prompt.go` + `internal/cmd/upgrade_prompt_test.go`
- [ ] NO edit to S1's `internal/cmd/upgrade.go` / `upgrade_test.go` (parallel)
- [ ] NO edit to `internal/upgrade/*` (P1.M4.T2/M3 own dispatch + primitives)
- [ ] NO edit to `root.go`, `config.Load`, `exitcode.go`, `docs/cli.md` (S1 owns Mode-A), the commit path (FR-U12), `go.mod`
- [ ] NO new dependency (ui.IsTerminal is in-repo; go.mod UNCHANGED)
- [ ] NO PRD/task file edit

### Code Quality & Docs
- [ ] confirmUpgrade is a pure helper (no exitcode import; no os.Exit; no "stagecoach:" prefix in the error)
- [ ] The isTTY seam is a package var (not a param) — matches the contract signature + the interactiveStdinIsTTY precedent
- [ ] The print helpers echo the command verbatim (never mutate the user's paste-able command)
- [ ] File comment explains FR-U9, the stricter-than-DefaultConfirm refusal, and that the dispatch (P1.M4.T2) is the caller

---

## Anti-Patterns to Avoid

- ❌ Don't add an `isTTY func() bool` PARAMETER to confirmUpgrade. The contract signature is fixed
  (`confirmUpgrade(current,target,action,assumeYes,in,out)`). The TTY-ness is the `confirmUpgradeIsTTY`
  package-var seam (clone of `interactiveStdinIsTTY`), read INSIDE the helper. Adding a parameter deviates
  from the contract and breaks the P1.M4.T2 call site.
- ❌ Don't copy integrate/protocol.go's `DefaultConfirm` non-TTY behavior. DefaultConfirm AUTO-DECLINES
  (returns false, no error) on non-TTY. confirmUpgrade REFUSES (returns (false, error) → exit 1). A declined
  file edit is reversible; a botched binary swap is not. The accept-only-y/Y rule is reused; the non-TTY
  policy is NOT.
- ❌ Don't prefix confirmUpgrade's refusal error with "stagecoach:". main.go:70 ALWAYS prepends
  "stagecoach: " to RunE errors. The codebase convention (config.go:162) is returned errors carry NO
  "stagecoach:" prefix. Return a plain `fmt.Errorf("...re-run with --yes...")`; the caller wraps
  `exitcode.New(exitcode.Error, err)` WITHOUT re-adding "stagecoach:". (Cf. the doubled-prefix bug pattern.)
  NOTE: the "stagecoach: upgrade aborted" DECLINE notice is a stdout line (NOT a returned error) — it DOES
  carry "stagecoach:" intentionally. Keep the two straight.
- ❌ Don't import internal/exitcode into confirmUpgrade. It returns a PLAIN error so it stays a pure,
  dependency-light helper. The CALLER (P1.M4.T2) wraps `exitcode.New(exitcode.Error, err)`. Coupling a pure
  prompt helper to the exit-code layer is unnecessary.
- ❌ Don't conflate the decline and the refusal. TTY 'n'/empty ⇒ (false, nil) → exit 0 (intentional user
  abort). Non-TTY+no-yes ⇒ (false, error) → exit 1 (safety refusal). Only the refusal is an error. A unit
  test that asserts err != nil on a TTY 'n' (or err == nil on non-TTY) is wrong.
- ❌ Don't edit S1's `internal/cmd/upgrade.go`. S1 is writing it in parallel. Put these helpers in the
  SEPARATE `internal/cmd/upgrade_prompt.go` (same package `cmd`; no name collision with S1's
  upgradeCmd/runUpgrade/validateUpgradeFlags/flagX). P1.M4.T2 will call them from runUpgrade.
- ❌ Don't wire confirmUpgrade into runUpgrade. S1's runUpgrade is a PLACEHOLDER; P1.M4.T2 owns the dispatch
  that calls confirmUpgrade/print*. This item PROVIDES the helpers + tests; it does NOT edit runUpgrade or
  touch upgrade.go. Confirming "the helper compiles and is tested" is the success bar, not "upgrade prompts".
- ❌ Don't prompt on --check or a pure PRINT inside confirmUpgrade. confirmUpgrade is a pure prompt — it does
  not know about --check/PRINT. P1.M4.T2's runUpgrade decides NOT to call confirmUpgrade on those paths
  (FR-U9). Adding a --check/PRINT branch here is out of scope and couples the helper to flag state.
- ❌ Don't mutate/quote/re-shell the command in the print helpers. printDelegatedUpdate/printPrivilegeCommand
  receive the EXACT command internal/upgrade produced (DelegateResult.Command / NeedsPrivilegesError.Command)
  and the user will PASTE it. Echo it verbatim (a single `%s`); only ADD the framing line(s).
- ❌ Don't export the helper names just to dodge the unused linter. The contract pins lowercase
  `confirmUpgrade`. The 6 tests reference all 3 helpers, which is how golangci-lint's `unused` is satisfied
  pre-P1.M4.T2 (same-package test refs count as uses). If U1000 ever fires, add the missing test reference —
  don't rename to exported.
- ❌ Don't add a new dependency (golang.org/x/term etc.) for TTY detection. The repo has its OWN
  `ui.IsTerminal` (internal/ui/output.go:32, platform-delegated isatty). go.mod is UNCHANGED.
- ❌ Don't touch docs/cli.md. S1 owns the Mode-A `### upgrade` subsection + exit-code-6 row. This item is
  code + tests only (the helpers have no doc surface of their own — they are internal implementation helpers
  P1.M4.T2 calls).