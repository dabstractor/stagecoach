# P1.M4.T1.S2 Research Findings — confirmUpgrade + --yes skip + run/print output helpers (FR-U9/U4/U7)

## §0 Headline / deliverable shape

This item provides the **user-facing I/O half** of `stagecoach upgrade`: the FR-U9 y/N confirmation
prompt (+ `--yes` skip + non-TTY refusal) and the FR-U4/FR-U7 run-vs-print OUTPUT FORMATTING helpers.
They are **shared helpers the orchestrator (P1.M4.T2 `runUpgrade`) calls**; S1's placeholder `runUpgrade`
does NOT call them yet (P1.M4.T2 wires the dispatch).

**Files (NEW, package `cmd` — separate from S1's `internal/cmd/upgrade.go` to avoid parallel-write collision):**
- `internal/cmd/upgrade_prompt.go` — `confirmUpgrade` + `confirmUpgradeIsTTY` seam + `printDelegatedUpdate` + `printPrivilegeCommand`.
- `internal/cmd/upgrade_prompt_test.go` — the contract's 4 confirmUpgrade cases + the 2 print-helper tests.

## §1 The three codebase precedents (the templates to clone)

1. **`internal/cmd/config_init_interactive.go:20`** — the EXACT injectable-isTTY-seam pattern:
   ```go
   var interactiveStdinIsTTY = func() bool { return ui.IsTerminal(os.Stdin) }
   ```
   And its pure-wizard idiom (`runInteractiveWizard(r io.Reader, w io.Writer, ...)` line 98, comment
   "No os.Stdin / IsTerminal inside — fully testable with bytes.Buffer and strings.NewReader"). **Clone this
   seam as `confirmUpgradeIsTTY`.** The TTY GATE lives at the caller layer in config_init (runConfigInitInteractive
   checks `interactiveStdinIsTTY()` BEFORE the wizard) — but for confirmUpgrade the gate is INSIDE the helper
   (the contract signature is `confirmUpgrade(current,target,action,assumeYes,in,out)` with no isTTY param;
   the seam is the injectable, read internally).

2. **`internal/integrate/protocol.go:240` `DefaultConfirm(out, path, diff string) bool`** — the y/N prompt
   precedent: `fmt.Fscanln(os.Stdin, &line)`; accept only first byte `y`/`Y`; non-TTY ⇒ auto-decline.
   **confirmUpgrade is STRICTER than DefaultConfirm**: non-TTY + no `--yes` ⇒ REFUSE with an error
   (caller → exit 1), NOT silent auto-decline. Rationale (contract decision): a declined file edit is
   reversible; a botched binary swap is not, so upgrade demands explicit opt-in (`--yes`). The y/Y-only
   accept rule is reused verbatim.

3. **`internal/cmd/root_test.go`** — the test state-restore idiom is NOT needed here (confirmUpgrade is a
   pure function taking io.Reader/io.Writer — no rootCmd state). Tests just call `confirmUpgrade(...,
   strings.NewReader("y\n"), &buf)` after flipping the `confirmUpgradeIsTTY` package var.

## §2 FR-U9 exact spec (prd_snapshot.md line 622) + the 4 contract outcomes

> **FR-U9.** The direct-swap path (FR-U5) and any delegated updater that stagecoach RUNS (FR-U4) prompt
> `y/N` before changing anything, printing current→target version and the action; `--yes`/`-y` skips the
> prompt for scripting. `--check` and a pure PRINT (FR-U4) never prompt.

The contract pins 4 observable outcomes from `confirmUpgrade(current, target, action, assumeYes, in, out) (bool, error)`:

| Condition | Return | Caller action | Exit |
|-----------|--------|---------------|------|
| `assumeYes` (--yes) | `(true, nil)` | proceed with the swap/delegation | 0 (on success) |
| non-TTY stdin, no --yes | `(false, err)` | `return exitcode.New(exitcode.Error, err)` | **1 (refuse)** |
| TTY, user typed `y`/`Y` | `(true, nil)` | proceed | 0 |
| TTY, user typed `n`/`N`/empty | `(false, nil)` | `return nil` (intentional abort) | **0** |

The non-TTY refusal error message (contract): `'re-run with --yes to confirm non-interactively'`.
The prompt format (contract): `stagecoach <current> → <target>\n<action>\nProceed? [y/N] `.

**CRITICAL — no "stagecoach:" prefix in confirmUpgrade's error**: `cmd/stagecoach/main.go:70` ALWAYS
prepends `stagecoach: ` to RunE errors (`fmt.Fprintf(os.Stderr, "stagecoach: %v\n", err)`). The codebase
convention (config.go:162, config_init_interactive.go:47) is RunEs return `exitcode.New(exitcode.Error,
fmt.Errorf("<msg WITHOUT 'stagecoach:' prefix>"))`. So confirmUpgrade returns a PLAIN `fmt.Errorf(...)`
(no prefix, no exitcode), and the caller wraps `exitcode.New(exitcode.Error, err)` WITHOUT re-adding
"stagecoach:" — else the doubled-prefix bug (cf. the OTHER plan's Issue 5). Documented as a gotcha.

## §3 The run-vs-print output helpers (FR-U4/FR-U7 framing)

**Key fact**: `internal/upgrade` ALREADY PRODUCES the raw command strings — my helpers are the
**user-facing FRAMING wrappers** P1.M4.T2 calls to render them consistently (centralize the wording so
runUpgrade doesn't inline it):
- `delegate.go` `printCommand(ch)` → `(primary, full)` (AUR `sudo pacman -Syu stagecoach-bin` / Nix
  `nix profile upgrade stagecoach`); `Delegate()` itself writes `full` to `opts.Out` (PRINT branch,
  `fmt.Fprintln(opts.out(), full)`) and returns `DelegateResult{Ran:false, Command:primary, ExitCode:0}`.
- `swap.go` `NeedsPrivilegesError{Command: ...}` (FR-U7 re-run form; `privilegeCommand(exe)` in
  swap_unix.go → `sudo "<exe>" upgrade`); `errors.As(err, &npe)` reads `.Command`.

So P1.M4.T2's runUpgrade, on a PRINT result / NeedsPrivilegesError, calls my helper to frame it + exits 0:

- **`printDelegatedUpdate(out io.Writer, channel, command string)`** — FR-U4 PRINT framing: "stagecoach
  was installed via `<channel>` — run its updater yourself (stagecoach does not run it for you):\n
  `<command>`". The caller returns nil (exit 0; FR-U4 "a printed command exits 0"). `command` is the
  verbatim `DelegateResult.Command`.
- **`printPrivilegeCommand(out io.Writer, command string)`** — FR-U7 needs-privileges framing: "install
  path not writable by the current user; re-run with privileges (stagecoach never auto-elevates):\n
  `<command>`". Caller returns nil (exit 0; FR-U4 never auto-sudo). `command` is the verbatim
  `NeedsPrivilegesError.Command`.

Both echo the command VERBATIM (never mutate the sudo/argv string the user will paste).

## §4 Placement & the parallel-S1 collision (why a SEPARATE file)

S1 is writing `internal/cmd/upgrade.go` + `internal/cmd/upgrade_test.go` + `docs/cli.md` RIGHT NOW. To
avoid a same-file parallel-write collision, this item's helpers go in a **SEPARATE** new file:
`internal/cmd/upgrade_prompt.go` + `internal/cmd/upgrade_prompt_test.go`. Same package (`cmd`), so
P1.M4.T2's `runUpgrade` (in upgrade.go) calls them directly; no new exports needed. **No name collisions**
with S1 (S1 owns `upgradeCmd`/`runUpgrade`/`validateUpgradeFlags`/`flagX`; this item owns
`confirmUpgrade`/`confirmUpgradeIsTTY`/`printDelegatedUpdate`/`printPrivilegeCommand`).

## §5 The U1000/unused-linter consideration (important)

`make lint` = `golangci-lint run` with `staticcheck` + `unused` enabled (`.golangci.yml`). Pre-P1.M4.T2,
these unexported helpers have **NO production caller** (S1's `runUpgrade` is a placeholder). golangci-lint's
`unused` linter treats **same-package `_test.go` references as uses** (standard Go tooling), so the helpers
are NOT flagged AS LONG AS `upgrade_prompt_test.go` references all three. **The tests MUST call
confirmUpgrade + printDelegatedUpdate + printPrivilegeCommand** (they do — §6). If a future config change
ever made unused ignore tests, export the names — but that is not needed with the current config.

## §6 Test design (contract: inject io.Reader strings + isTTY seam)

- `TestConfirmUpgrade_AssumeYes`: assumeYes=true → (true,nil); `out` has NO "Proceed?" (no prompt).
- `TestConfirmUpgrade_NonTTYRefuses`: flip `confirmUpgradeIsTTY`=false, assumeYes=false → (false,err);
  `err != nil`, `strings.Contains(err.Error(), "re-run with --yes")`; `out` has NO "Proceed?" (refused
  before prompting).
- `TestConfirmUpgrade_TTYYes`: `confirmUpgradeIsTTY`=true, `in=strings.NewReader("y\n")` → (true,nil);
  `out` contains "current → target", the action, "Proceed? [y/N]". (Also cover uppercase "Y\n".)
- `TestConfirmUpgrade_TTYDeclines`: `in=strings.NewReader("n\n")` → (false,nil); `out` contains "aborted".
  (Also `in=strings.NewReader("\n")` empty → (false,nil).)
- `TestPrintDelegatedUpdate`: `out` contains the framing + the command verbatim + "does not run".
- `TestPrintPrivilegeCommand`: `out` contains the framing + the command verbatim + "never auto-elevates".

isTTY seam override idiom (save/restore to avoid leaking into sibling tests):
```go
orig := confirmUpgradeIsTTY
confirmUpgradeIsTTY = func() bool { return true }
defer func() { confirmUpgradeIsTTY = orig }()
```

## §7 APIs consumed (LANDED — read-only)
- `ui.IsTerminal(os.Stdin)` — internal/ui/output.go:32 (custom isatty, NO golang.org/x/term dep; go.mod
  has only cobra/pflag/toml/yaml). The TTY probe to clone into the seam.
- `exitcode.New(exitcode.Error, err)` / `exitcode.Error=1` — internal/exitcode/exitcode.go:27,58. The
  CALLER wraps confirmUpgrade's plain error; confirmUpgrade itself does NOT import exitcode (pure helper).
- `upgrade.Channel` (`type Channel string`; ChannelBrew="brew"…ChannelDirect) — internal/upgrade/detect.go:36.
  printDelegatedUpdate takes a STRING channel label (P1.M4.T2 passes `string(ch)`), keeping the helper
  free of an internal/upgrade import.

## §8 Scope fences
- Touches ONLY: `internal/cmd/upgrade_prompt.go` (NEW) + `internal/cmd/upgrade_prompt_test.go` (NEW).
- Does NOT edit: S1's `internal/cmd/upgrade.go` / `upgrade_test.go`, root.go, internal/upgrade/* (the
  dispatch + primitives are P1.M4.T2/M3), config.Load, exitcode.go, docs/cli.md (S1 owns the Mode-A doc
  edit), the commit path (FR-U12), go.mod, any PRD/task file.
- confirmUpgrade NEVER os.Exit (returns bool/error; the caller routes exit codes). NEVER prompts for
  --check / PRINT (the caller simply doesn't call it on those paths).