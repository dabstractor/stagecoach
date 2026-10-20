# Provider Execution Subsystem — Grounding Report (spec v3.5: activity-based generation deadlines + event-stream output)

Repo: /home/dustin/projects/stagecoach @ HEAD `df98a9a6d971345081141060b8ccd1f0f7dd6714` ("spec: add idle + hard-cap generation deadlines (v3.5)"). Working tree clean except this untracked plan dir. All line numbers below refer to that commit. Package: `github.com/dabstractor/stagecoach/internal/provider` (all files `package provider`).

**Verdict up front: every expectation in the brief matched reality, with two phrasing-level nuances flagged inline (§6: `StderrHint` lives in executor.go, not parse.go; `DEBUG: cause` is emitted in internal/cmd, not internal/provider). No activity*.go or event-stream code exists (§10).**

---

## 1. internal/provider/executor.go

### 1.1 StderrHint (lines 15–38) — the cause-hint mechanism (exported, package-level)

```go
// executor.go:23
func StderrHint(stdout, stderr string) string {
	if strings.TrimSpace(stdout) != "" || strings.TrimSpace(stderr) == "" {
		return ""
	}
	hint := strings.TrimSpace(stderr)
	// collapse to a single line and cap so a noisy provider can't flood the error
	hint = strings.ReplaceAll(hint, "\n", " ")
	if len(hint) > 1000 {
		hint = hint[:1000] + "…"
	}
	return "agent produced no output; provider stderr: " + hint
}
```

Returns `""` when stdout has usable content OR stderr is empty; otherwise a single-line, 1000-char-capped hint. Callers today (all pattern `if h := provider.StderrHint(out, stderr); h != ""`): `internal/decompose/message.go:201`, `internal/decompose/planner.go:140`, `internal/generate/generate.go:402`. **This is the existing "empty stdout ⇒ surface stderr cause" seam** — an event-stream mode that also dies with empty stdout should reuse it.

### 1.2 Execute — full signature (line 65)

```go
// executor.go:65
func Execute(ctx context.Context, spec CmdSpec, timeout time.Duration, vb *ui.Verbose) (stdout string, stderr string, err error) {
```

- Input types: `spec CmdSpec` (defined in render.go:32, see §5.1), `timeout time.Duration`, `vb *ui.Verbose` (from `internal/ui`, see §1.6).
- Output: stdout/stderr as strings **returned even on error** (partial output is useful for rescue/verbose), plus `err`.

Surrounding behavior, verbatim (lines 66–103):

```go
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout) // SHADOW — see doc; do not rename
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, spec.Command, spec.Args...)
	if spec.Stdin != "" {
		cmd.Stdin = strings.NewReader(spec.Stdin) // "" ⇒ nil ⇒ /dev/null (CmdSpec contract)
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out // separate capture
	cmd.Stderr = &errb
	if len(spec.Env) > 0 {
		cmd.Env = spec.Env // Render populates; nil ⇒ inherit parent env
	}
	setupProcessGroup(cmd) // platform seam (procgroup_*.go): Setpgid + Cancel + WaitDelay

	vb.VerboseCommand(strings.Join(append([]string{spec.Command}, spec.Args...), " "))
	vb.VerbosePayload(spec.PayloadBytes) // size only (never contents) — exposes whether the token-limit gate ran
	if err := cmd.Start(); err != nil {
		return "", "", fmt.Errorf("provider %q: start: %w", spec.Command, err)
	}
	signal.RegisterChild(cmd.Process.Pid) // arm signal forwarding (Setpgid ⇒ PGID==PID)
	defer signal.ClearChild()             // clear before return so a later signal can't kill a recycled PID

	if werr := cmd.Wait(); werr != nil {
		vb.VerboseRawOutput(out.String())
		vb.VerboseStderr(errb.String()) // surface the provider's real failure reason (upstream errors live on stderr)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return out.String(), errb.String(), ctxErr // timeout → DeadlineExceeded; cancel → Canceled
		}
		return out.String(), errb.String(), fmt.Errorf("provider %q: %w", spec.Command, werr) // exit failure
	}
	vb.VerboseRawOutput(out.String())
	vb.VerboseStderr(errb.String()) // surface provider warnings/diagnostics even on success
	return out.String(), errb.String(), nil
}
```

### 1.3 Timeout application (lines 66–70)

`context.WithTimeout` derived from the parent ctx **when `timeout > 0`**; the derived ctx **shadows** the parameter (`ctx, cancel = ...`) — load-bearing: the later `ctx.Err()` at line 95 reads the *timeout* ctx. `timeout ≤ 0` ⇒ no timeout (parent ctx still applies). The command itself is created by `exec.CommandContext` (line 72), so ctx cancellation triggers `cmd.Cancel` (set by `setupProcessGroup`). **Note for v3.5: this is a single fixed wall-clock deadline applied once, before Start. An idle/activity-based deadline cannot be expressed by `time.Duration` alone — it needs either a custom `context.Context` implementation (a deadline that extends on observed output) or an executor-level watchdog; both hook into the same ctx seam.**

### 1.4 stdout/stderr capture (lines 76–78)

Two **separate `bytes.Buffer`s**, assigned to `cmd.Stdout = &out` and `cmd.Stderr = &errb`. Capture is fully buffered — `cmd.Wait()` returns only after process exit, and the buffers are stringified once at return. **There is no incremental/streaming read of stdout anywhere in this file — an event-stream parser that must consume stdout *as it arrives* (to detect activity / emit events) needs `cmd.Stdout` to be an `io.Writer` wrapper that both feeds the parser and accumulates for the existing return contract.** The doc comment (lines 43–46) states the contract: "stdout and stderr are captured to SEPARATE buffers and returned even on error".

### 1.5 Kill path (lines 72, 82, 86–90, 92–98)

- `exec.CommandContext(ctx, ...)` (line 72) — ties ctx cancel to `cmd.Cancel`.
- `setupProcessGroup(cmd)` (line 82) — the single platform seam (§2): on Unix sets `SysProcAttr.Setpgid`, `cmd.Cancel` (SIGTERM to `-pid`, the whole group), `cmd.WaitDelay = 3s`.
- `cmd.Start()` (line 86); on failure returns wrapped start error (line 87).
- `signal.RegisterChild(cmd.Process.Pid)` (line 89; `internal/signal/signal.go:188`) arms stagecoach's own signal forwarding to the child group; `defer signal.ClearChild()` (line 90; `signal.go:196`) clears it before return so a recycled PID can't be killed by a later signal. Valid because Setpgid guarantees PGID==PID.
- `cmd.Wait()` (line 92): on error, verbose dumps fire, then **`ctx.Err()` is checked FIRST** (line 95) and returned bare if non-nil; otherwise the wrapped `*exec.ExitError`.
- **Signal escalation** (SIGKILL after grace) is performed by os/exec itself via `cmd.WaitDelay` — not by procgroup code. Same on Windows (os/exec TerminateProcess's the direct child).

### 1.6 Exact error value on timeout

On timeout, Execute returns the **bare sentinel `context.DeadlineExceeded`** (line 96 returns `ctxErr` from `ctx.Err()` — no wrapping, no `%w`). Parent/signal cancel ⇒ bare `context.Canceled`. Non-zero exit ⇒ `fmt.Errorf("provider %q: %w", spec.Command, werr)` (wrapped `*exec.ExitError`). Start failure ⇒ `fmt.Errorf("provider %q: start: %w", spec.Command, err)`. Documented contract (lines 58–64):

```go
// Error contract (check ctx.Err() FIRST):
//
//   - timeout        ⇒ err IS context.DeadlineExceeded  (orchestrator: exit 124 + rescue)
//   - signal/parent  ⇒ err IS context.Canceled          (orchestrator: exit 3 + rescue)
//   - non-zero exit  ⇒ wrapped *exec.ExitError           (orchestrator: retry, then rescue)
//   - start failure  ⇒ wrapped LookPath/start error      (orchestrator: "command not found", exit 1)
//   - success        ⇒ err == nil
```

Downstream: `pkg/stagecoach/stagecoach.go:569-573` maps `errors.Is(execErr, context.DeadlineExceeded)` → `&generate.RescueError{Kind: generate.ErrTimeout, ...}` → exit 124 (internal/cmd/default_action.go:373-388).

### 1.7 Verbose parameter and ui functions

`vb *ui.Verbose` (`internal/ui/verbose.go:34 type Verbose struct`; `verbose.go:44 func NewVerbose(w io.Writer, on bool) *Verbose`). **Every method is nil-safe (no-op when `v == nil`)** — all Execute callers pass `nil` in production unless `--verbose`. Methods emitted by Execute:

| call site (executor.go) | ui function | exact line format (verbose.go) |
|---|---|---|
| :84 (pre-Start) | `VerboseCommand(cmd string)` | `"DEBUG: command: " + cmd + "\n"` (verbose.go:57) |
| :85 (pre-Start) | `VerbosePayload(bytes int)` | `"DEBUG: payload: %d bytes (~%d tokens est)\n"`, tokens = `(bytes+3)/4` (verbose.go:118); no-op when bytes==0 |
| :93, :100 | `VerboseRawOutput(output string)` | `"DEBUG: raw output:\n"` + verbatim (verbose.go:74) |
| :94, :101 | `VerboseStderr(stderr string)` | `"DEBUG: stderr:\n"` + verbatim (verbose.go:96) |

Other available emitters (not used by Execute): `VerboseWarn` → `"DEBUG: <msg>\n"` (verbose.go:129), `VerboseRetry` → `"DEBUG: attempt %d: %s\n"` (verbose.go:141), `VerboseRoles` (verbose.go:164+). Tests assert **no env vars leak** into verbose output (§19 security guard) — any new verbose line must respect that.

---

## 2. procgroup_unix.go + procgroup_windows.go

### 2.1 procgroup_unix.go (`//go:build !windows`, line 1)

`setupProcessGroup` at line 28 — the complete implementation:

```go
// procgroup_unix.go:28
func setupProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) // -pid ⇒ whole process group
	}
	cmd.WaitDelay = 3 * time.Second
}
```

- **Setpgid**: child becomes a new process-group leader ⇒ **PGID == PID by kernel guarantee** (a new group created by Setpgid gets the child's own PID as its PGID), so `-pid` addresses child + all descendants. This invariant is also what makes `signal.RegisterChild(cmd.Process.Pid)` (executor.go:89) sufficient for whole-group signal forwarding.
- **cmd.Cancel**: on ctx cancel (timeout OR parent/signal), SIGTERM to the group — gentler than default SIGKILL, reaches grandchildren.
- **cmd.WaitDelay = 3s**: grace before Go's os/exec escalates to SIGKILL.
- The file header (lines 22–26) freezes the contract: *"the Windows build (procgroup_windows.go, //go:build windows) MUST implement the IDENTICAL signature `func setupProcessGroup(cmd *exec.Cmd)`"*. CI matrix targets Linux + macOS.

### 2.2 procgroup_windows.go (`//go:build windows`, line 1)

`setupProcessGroup` at line 46:

```go
// procgroup_windows.go:46
func setupProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP, // child = console process-group leader; PID==PGID
	}
	cmd.Cancel = func() error {
		// CTRL_BREAK_EVENT (1) targets the specific group == child PID (NOT a negative pid; Windows has no -pid).
		r1, _, err := procGenerateConsoleCtrlEvent.Call(
			uintptr(syscall.CTRL_BREAK_EVENT),
			uintptr(cmd.Process.Pid),
		)
		if r1 == 0 {
			return err
		}
		return nil
	}
	cmd.WaitDelay = 3 * time.Second
}
```

Mechanism: `CREATE_NEW_PROCESS_GROUP` (0x00000200) makes the child a **console** process-group leader (the Windows parallel of PGID==PID); `cmd.Cancel` calls `kernel32!GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT, childPID)` via `syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent")` (line 15, resolved lazily — keeps go.mod stdlib-only). CTRL_BREAK (not CTRL_C) because it honors the process-group id; CTRL_C would broadcast to the caller's whole console. `WaitDelay = 3s` — if the child ignores CTRL_BREAK, os/exec TerminateProcess'es the direct child. Documented v1 limitation (header): grandchildren that detach from the console or swallow CTRL_BREAK may survive; Job Objects deferred. Error contract identical to Unix (header): ctx.Err() sentinels flow through unchanged. `procgroup_windows_test.go` exists alongside.

---

## 3. internal/provider/manifest.go

### 3.1 Defaults constants (lines 10–15)

```go
// PRD §12.1 default values (applied by Resolve to nil optional fields).
const (
	DefaultPromptDelivery   = "stdin"
	DefaultOutput           = "raw"
	DefaultStripCodeFence   = true
	DefaultRetryInstruction = "Output ONLY the commit message. No preamble, no markdown, no quotes."
)
```

Enum member sets (lines 18–20): `validPromptDeliveries = {stdin, positional, flag}`, `validOutputs = {raw, json}`.

### 3.2 The complete Manifest struct (lines 37–106), all fields + toml tags

```go
// manifest.go:37
type Manifest struct {
	// --- discovery (§12.1) ---
	Name              string   `toml:"name"`                // REQUIRED; registry sets from table key
	Detect            *string  `toml:"detect"`              // nil/"" => DetectCommand falls back to Command
	Command           *string  `toml:"command"`             // REQUIRED (post-merge)
	ListModelsCommand []string `toml:"list_models_command"` // nil/empty => no listing command
	Subcommand        []string `toml:"subcommand"`          // nil => none; between command and flags

	// --- prompt delivery (§12.1) ---
	PromptDelivery *string `toml:"prompt_delivery"` // stdin|positional|flag; nil => Resolve→"stdin"
	PromptFlag     *string `toml:"prompt_flag"`     // used only when PromptDelivery=="flag"

	// --- non-interactive / print mode (§12.1) ---
	PrintFlag *string `toml:"print_flag"` // nil/"" => no print flag appended

	// --- model (§12.1) ---
	ModelFlag    *string `toml:"model_flag"`    // e.g. "--model"
	DefaultModel *string `toml:"default_model"` // nil/"" => user must set a model

	// --- system prompt (§12.1) ---
	SystemPromptFlag *string `toml:"system_prompt_flag"` // nil/"" => prepend sys to payload (§12.2)

	// --- sub-provider (§12.1) ---
	ProviderFlag *string `toml:"provider_flag"`

	// --- session continuation (multi-turn fallback, §9.24) ---
	SessionMode *string `toml:"session_mode"`        // "" | "append"; nil => Resolve→""

	// --- bare mode (§12.1) ---
	BareFlags []string `toml:"bare_flags"` // appended verbatim; nil => none

	// --- tooled mode (v2; §11.5, §12.1) ---
	TooledFlags       []string `toml:"tooled_flags"`        // nil/empty => cannot serve as stager
	TooledRepoDirFlag *string  `toml:"tooled_repo_dir_flag"` // non-nil non-empty => append [flag, repoDir]

	// --- experimental (§12.7.2, §12.5.1) ---
	Experimental *bool `toml:"experimental"` // §12.7.2 default false

	// --- output (§12.1) ---
	Output         *string `toml:"output"`           // raw|json; nil => Resolve→"raw"
	JsonField      *string `toml:"json_field"`       // used only when Output=="json"
	StripCodeFence *bool   `toml:"strip_code_fence"` // nil => Resolve→true

	// --- retry (§12.1) ---
	RetryInstruction *string `toml:"retry_instruction"` // nil => Resolve→default

	// --- environment (§12.1) ---
	Env map[string]string `toml:"env"` // nil => none

	// --- reasoning levels (v3; FR-R6) ---
	ReasoningLevels map[string][]string `toml:"reasoning_levels"` // [reasoning_levels] subtable; nil => no-op
}
```

**Design call (header comment, lines 28–36)**: go-toml/v2 has no omitempty, so optional SCALARS are pointers — absent in TOML decodes to **nil** (inherit on merge), present-even-`""`/`false` decodes to a **non-nil pointer** (override). Slices/maps stay plain (nil is their natural absent sentinel). Name is plain (identity, set by the registry from the table key). `strPtr`/`boolPtr` helpers at lines 223–224.

### 3.3 SessionMode precedent — THE template for a new optional capability field

This is exactly the pattern a v3.5 "event/stream mode" capability field should follow:

1. **Field** (line 67): `SessionMode *string \`toml:"session_mode"\`` with a block comment stating the semantics of `""` (default = capability off), the capability value (`"append"`), and that it "REQUIRES a verified append rendering (FR-T9); never set speculatively."
2. **Validation** (lines 129–132) — closed whitelist, nil-tolerant:

```go
	if m.SessionMode != nil {
		if *m.SessionMode != "" && *m.SessionMode != "append" {
			return fmt.Errorf("provider manifest %q: session_mode %q must be \"\" or \"append\"", m.Name, *m.SessionMode)
		}
	}
```

3. **Resolve default** (lines 211–212): `if out.SessionMode == nil { out.SessionMode = strPtr("") }` — every optional pointer non-nil post-Resolve, so consumers deref safely.
4. **Merge** (merge.go:59–60): regime-1 pointer override — "explicit `\"\"` disables multi-turn (overrides the built-in `\"append\"`)".
5. **Runtime gate** (render.go:225): `if *r.SessionMode != "append" { return error }` — a capability is *acted on* only at render/execute time, erroring when a caller requests an unsupported capability.
6. **Built-in declaration** (builtin.go:61): `SessionMode: strPtr("append"), // VERIFIED 2026-07-05 via \`pi --session-id X ...\`` — the dated-verification comment precedent (see §7.3).

### 3.4 Validate / ValidateModel / DetectCommand / Resolve

- `Validate()` (line 112): strict on Name non-empty + Command non-nil-non-empty; rejects non-nil invalid enums (`prompt_delivery`, `output`, `session_mode`); nil optionals pass (safe on partial overrides; primary call site post-merge).
- `ValidateModel(model string)` (line 155): pre-flight of Render's FR-R5b slash check (provider_flag provider needs "inference/model") so callers can reject before printing "↳ Generating…".
- `DetectCommand()` (line 173): Detect if set/non-empty, else Command, else "" (registry treats "" as not-installed via LookPath).
- `Resolve()` (line 190): returns a copy with every nil optional pointer defaulted (PromptDelivery→"stdin", PrintFlag/PromptFlag/ModelFlag/DefaultModel/SystemPromptFlag/ProviderFlag/SessionMode→`""`, Output→"raw", JsonField→`""`, StripCodeFence→true, Experimental→false, RetryInstruction→default, Detect→`""`); **Command left nil if absent** (Validate enforces); slices/maps left as-is (nil stays nil — `append(nil...)` no-op). Explicit values — including non-nil `false`/`""` — are PRESERVED ("Resolve never overwrites a present value; this is the correctness keystone of the pointer design").

---

## 4. internal/provider/merge.go — MergeManifest (line 28)

`func MergeManifest(base, override Manifest) Manifest` — overlays non-nil/non-empty override fields onto a struct copy of base, per PRD §16.1 field-by-field merge. Three regimes (header lines 6–26):

1. **Scalar pointer fields** (`*string`/`*bool`): `override.Field != nil` ⇒ result takes the override pointer. An **explicit `""` or `false` OVERRIDES** (non-nil is the signal). Fields covered (lines 32–81): Detect, Command, PromptDelivery, PromptFlag, PrintFlag, ModelFlag, DefaultModel, SystemPromptFlag, ProviderFlag, SessionMode (line 59–60, with FR-37a comment), Output, JsonField, StripCodeFence, Experimental, RetryInstruction.
2. **Slices** (lines 84–99): `len(override.Slice) > 0` ⇒ wholesale REPLACE, **no element-level merge**; empty/nil override = "not overridden" (base preserved). Covers Subcommand, ListModelsCommand, BareFlags, TooledFlags. (So an empty builtin slice like opencode's `BareFlags: []string{}` can never be re-added by a user override with `[]` — only replaced with non-empty.)
3. **Maps** (Env, lines 102–117; ReasoningLevels, lines 121–129): key-by-key merge into a **FRESH map** (override keys win, base keys survive) — fresh allocation is CRITICAL because `out := base` aliases map headers; in-place mutation would corrupt the caller's base. ReasoningLevels values replace wholesale per key.

**Name is NOT field-merged** — `out.Name == base.Name`; the registry overwrites it from the `[provider.<name>]` table key. MergeManifest performs NO Validate/Resolve (a partial override legitimately lacks Command). Nil vs zero summary: strings — nil=inherit, `""`=override; bools — nil=inherit, `false`=override; slices — nil or empty=inherit, non-empty=replace; maps — nil/empty=inherit, else key-merge.

---

## 5. internal/provider/render.go — CmdSpec + the Render algorithm

### 5.1 CmdSpec (lines 32–39) — where it is defined

```go
// render.go:32
type CmdSpec struct {
	Command      string   // the executable (resolved manifest.Command), e.g. "pi", "agent"
	Args         []string // the flag portion AFTER command, in §12.2 token order (NOT including Command)
	Stdin        string   // payload to pipe (stdin delivery); "" → executor uses os.DevNull
	PayloadBytes int      // delivered payload size in bytes (ALL delivery modes); set by Render/RenderMultiTurn; 0 ⇒ none
	Env          []string // os.Environ() + manifest Env entries as "KEY=VAL" (manifest wins on collision)
}
```

Pure data; Render performs NO spawning (os.Environ() is the sole side effect). `PayloadBytes` is delivery-agnostic (FR50 verbose) — do not conflate with `len(Stdin)`. CmdSpec carries **no mode/event metadata** today — an event-stream mode would either bake its flags into Args at Render time or need a new field here.

`RenderMode` (line 48): `type RenderMode string`, consts `RenderBare = "bare"` (default) / `RenderTooled = "tooled"` (lines 51–62). Render's `mode ...RenderMode` is **variadic, defaulting to bare** so every v1 caller is unchanged.

### 5.2 Render (line 100) — exact argument composition order

`func (m Manifest) Render(model, sysPrompt, userPayload, reasoning string, mode ...RenderMode) (*CmdSpec, error)` — calls `m.Validate()` then `m.Resolve()`; model-default fallback (`model == ""` → `*r.DefaultModel`). Composition, in exact order with line numbers:

| # | tokens appended | line | guard |
|---|---|---|---|
| 1 | `r.Subcommand...` | 114 | none (append(nil…) no-op) |
| 2 | `*r.ProviderFlag, modelToUse[:i]` | 123 | ProviderFlag≠"" && model has "/" (FR-R5b fold; bare model on provider_flag provider = HARD ERROR at 126–129) |
| 3 | `*r.ModelFlag, modelToUse` | 132 | ModelFlag≠"" && model≠"" |
| 4 | `r.ReasoningLevels[reasoning]...` | 138 | reasoning≠"" && non-empty token list (FR-R6; else silent no-op) |
| 5 | `*r.SystemPromptFlag, sysPrompt` | 142 | SystemPromptFlag≠"" && sys≠"" |
| 6 | **mode flag-set**: `r.TooledFlags...` (157) if `selectedMode == RenderTooled` (errors at 155 if TooledFlags empty), else `r.BareFlags...` (159) | 148–159 | `selectedMode := RenderBare; if len(mode) > 0 && mode[0] != "" { selectedMode = mode[0] }` (148–151); default case comment: "RenderBare — also the fallback for \"\" / any unrecognized mode" |
| 7 | `*r.PrintFlag` | 160–162 | `if *r.PrintFlag != "" { args = append(args, *r.PrintFlag) // LAST per §12.2 }` |

Then (post-flags): payload unification (sys prepended with `"\n\n"` delimiter when SystemPromptFlag=="" and sys≠"", lines 166–169), the delivery switch into `spec := &CmdSpec{Command: *r.Command, Args: args}` (174–183): `stdin` → `spec.Stdin = payload`; `positional` → `spec.Args = append(spec.Args, payload)`; `flag` → `spec.Args = append(spec.Args, *r.PromptFlag, payload)`; default → error. `spec.PayloadBytes = len(payload)` (187). Env = `os.Environ()` + manifest Env as `k+"="+v` appended last (191–196).

### 5.3 THE INSERTION POINT for new tokens "after the mode flag-set, before print_flag"

Between **render.go:159/160 (end of the mode switch — after `args = append(args, r.BareFlags...)` / the switch's closing brace) and render.go:160–162 (`if *r.PrintFlag != "" { args = append(args, *r.PrintFlag) }`)**. Concretely:

```go
	default: // RenderBare — also the fallback for "" / any unrecognized mode (PRD §12.2 ternary)
		args = append(args, r.BareFlags...)
	}                        // ← render.go:160 — switch closes; NEW TOKENS INSERT HERE
	if *r.PrintFlag != "" {  // ← render.go:161
		args = append(args, *r.PrintFlag) // LAST per §12.2
	}
```

i.e. after the mode ternary completes (TooledFlags or BareFlags appended) and **strictly before the print_flag guard**, preserving print_flag's "ALWAYS LAST" §12.2 invariant (any new token must not displace `-p` from the tail — some CLIs like pi treat a trailing `-p` as the print-mode toggle). `RenderMultiTurn` (line 217) has the **same seam** between `args = append(args, sessionArgs...)` (line 275) and its print_flag block (lines 277–279) — a v3.5 flag insertion must be mirrored in BOTH functions (they are siblings by design; see the RenderMultiTurn header, lines 199–216, which explicitly rejected widening Render).

RenderMultiTurn deltas vs Render: capability gate `*r.SessionMode != "append"` → error (225); turn-1-only system prompt (`turnSys` local, 234–236); session-flags block = BareFlags minus exact token `"--no-session"` plus `"--session-id", sessionID` into a FRESH slice (268–275); everything else (model fold, reasoning, print_flag, delivery switch, PayloadBytes, Env) identical.

---

## 6. internal/provider/parse.go — parsing pipeline

### 6.1 ParseOutput (line 41) — the 5-step pipeline

`func ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool)` — pure function over captured stdout + the manifest's three output fields; calls `m.Resolve()` first (nil-pointer-safe deref, mirrors render.go). Stages (verbatim order from the authoritative header, lines 12–25, implemented at 51–77):

1. `s = strings.TrimSpace(raw)` (line 52)
2. If `*r.StripCodeFence` and s starts with ``` or ~~~: `stripCodeFence(s)` removes the first line (opener + language tag) and everything from the LAST fence closer onward; re-trim (lines 56–58; impl 168–196)
3. Output-mode switch (lines 60–70): `raw` → `msg = s`; `json` → `parseJSON(s, *r.JsonField)`; default → treated as raw (robustness over strictness)
4. `normalizeNewlines(msg)` — `\r\n`→`\n`, collapse 3+ `\n` to 2 (line 74; impl 214–236; no lone-`\r` handling, no regexp)
5. `msg = strings.TrimSpace(msg); ok = msg != ""` (lines 77–78)

`parseJSON` (line 81): whole-string `json.Unmarshal` → on failure `extractJSONObject` (first brace-balanced `{...}`, quote/escape-aware state machine, lines 103–148) → extract `obj[field]` as **string**; ANY failure ⇒ `(s, true)` fallback.

### 6.2 Empty-output failure path

There is no dedicated error — emptiness surfaces as **`ok == false`** (step 5: `ok = msg != ""`, line 78, comment: "ok==false ⇒ orchestrator retries with retry_instruction (FR29)"). The orchestrator (e.g. pkg/stagecoach/stagecoach.go retry loop, generate.go) retries once with the manifest's RetryInstruction prepended, then rescues. `fellback` is a pure logging signal (json-mode parse failure), never changes retry behavior.

### 6.3 StderrHint / cause mechanism — REALITY FLAG

**`StderrHint` is NOT in parse.go — it is in executor.go:23–38** (§1.1 above). The brief's grouping implied it might be a parse concern; it lives with the executor because it operates on captured stdout+stderr pairs. The other cause mechanism, `DEBUG: cause:`, is emitted **outside this package entirely**: `rg -n 'DEBUG: cause'` across the repo finds exactly one emitter — **internal/cmd/default_action.go:381**:

```go
		if verboseOn && re.Cause != nil {
			fmt.Fprintln(stderr, "DEBUG: cause: "+re.Cause.Error())
		}
```

(appended AFTER the spec-frozen §18.3 rescue recipe, verbose-only, mirroring ui.Verbose's DEBUG style; `re` is `*generate.RescueError` carrying a `Cause error` field — generate.go:113 area). Test references: internal/cmd/default_action_test.go:1141, 1153, 1169. **No `DEBUG: cause` line exists anywhere in internal/provider.**

---

## 7. internal/provider/builtin.go + registry.go

### 7.1 How built-ins are declared

**Go structs, not embedded TOML.** `BuiltinManifests()` (builtin.go:18) returns a fresh map each call (no package-level var — fresh-per-call prevents any caller corrupting a built-in by mutating returned slices/maps; tested by `TestBuiltinManifests_FreshEachCall`):

```go
// builtin.go:18
func BuiltinManifests() map[string]Manifest {
	return map[string]Manifest{
		"pi":       builtinPi(),
		"claude":   builtinClaude(),
		"opencode": builtinOpenCode(),
		"codex":    builtinCodex(),
		"cursor":   builtinCursor(),
		"agy":      builtinAgy(),
	}
}
```

### 7.2 Full built-in list (6 providers) with constructor lines

| name | ctor (builtin.go) | Detect/Command | delivery | print | SessionMode | Experimental | notable |
|---|---|---|---|---|---|---|---|
| pi | builtinPi :49 | pi/pi | stdin | `-p` | `"append"` (:61) | no | ReasoningLevels `--thinking`; TooledFlags = bare minus `--no-tools`; ListModelsCommand `pi --list-models` |
| claude | builtinClaude :123 | claude/claude | stdin | `-p` | nil | no | `--tools ""`, `--setting-sources ""`; tooled = `--allowed-tools` git-allowlist |
| opencode | builtinOpenCode :280 | opencode/opencode | stdin | `""` (non-nil) | nil | no | Subcommand `["run"]`; BareFlags `[]string{}` (non-nil empty); tooled `--agent build` |
| codex | builtinCodex :342 | codex/codex | stdin | `""` | nil | no | Subcommand `["exec"]`; `--sandbox read-only --ephemeral`; tooled `danger-full-access` |
| cursor | builtinCursor :397 | **agent**/agent (only Detect≠Name) | positional | `-p` | nil | no | `--mode ask --trust`; Subcommand `[]string{}`; tooled `--trust --yolo` |
| agy | builtinAgy :222 | agy/agy | stdin | `""` (non-nil; bare `-p` is value-taking) | nil | **true** | `--mode plan`; DefaultModel = display label `"Gemini 3.5 Flash (Low)"`; TooledRepoDirFlag `--add-dir` |

All six set `Output: strPtr("raw")`, `StripCodeFence: boolPtr(true)`; nil fields are listed explicitly in trailing comments (e.g. "Subcommand, PromptFlag, JsonField, RetryInstruction, Env: nil").

### 7.3 Dated-verification comment precedent (the FR-T9 style to copy for new per-provider claims)

The pattern is an inline `// VERIFIED <YYYY-MM-DD> via <exact probe> ; <FR ref>` on the field or block, plus a dated re-verification paragraph in the constructor doc comment. Examples:

- `SessionMode: strPtr("append"), // VERIFIED 2026-07-05 via \`pi --session-id X <isolation-flags-minus-no-session> -p "remember BANANA"\` then recall returns BANANA; FR-T9.` (builtin.go:61)
- `ListModelsCommand: []string{"pi", "--list-models"}, // VERIFIED 2026-07-03 via \`pi --list-models\` (exit 0); FLAG form, not a subcommand. FR-L2/FR-D5.` (builtin.go:55)
- CHROME-DISABLE (FR-C5) blocks: "CHROME-DISABLE (FR-C5, §9.28): verified vs `pi --help` (external_deps.md §pi, 2026-06-29)." and per-provider "verified … 2026-07-08" paragraphs; `TestBuiltinManifests_ChromeDisableContract` (builtin_test.go:708) enforces them.
- Tooled flags: "VERIFIED 2026-07-09 (agy v1.1.11) end-to-end: stdin task + this combo stages exactly the requested paths."

A new per-provider capability claim (e.g. "emits JSONL events on stdout with flag X") must carry this same dated, probe-named verification.

### 7.4 registry.go — structure and where an event-extractor registry belongs

`package provider`; key items: `preferredBuiltins` slice (line 16: pi, opencode, cursor, agy, codex, claude — FR-D1 cascade; a test enforces sync with BuiltinManifests keys); `type Registry struct { manifests map[string]Manifest }` (~line 31); `NewRegistry(userOverrides)` (line 36 — seeds fresh built-ins, MergeManifest onto matching names, brand-new §12.8 names verbatim, Name always from table key; NO Validate/Resolve — consumers do Validate→Resolve on Get); `Get(name)`; `List()` sorted by Name; `IsInstalled(m)` via `exec.LookPath(m.DetectCommand())`; `MarshalTOML(name)` (go-toml omits nil pointers — free omitempty); `DefaultProvider(installed)`; `FirstTooledProvider(installed)` (two-phase: built-ins in preference order, then user-defined); `PreferredBuiltins()`; `DecodeUserOverrides(raw)` (raw `map[string]map[string]any` → re-encode TOML → Unmarshal into Manifest; the ONLY toucher of the raw config shape).

**Natural home for a per-provider event-extractor registry**: package `provider` itself, as a sibling declaration file — the established per-provider-dimension patterns are (a) a function returning `map[string]<X>` keyed by provider name, constructed fresh per call, mirroring `BuiltinManifests()` (builtin.go:18), and (b) capability carried on the Manifest as an optional pointer field with SessionMode-style default/validation/merge (§3.3). A new `events.go` (or `stream.go`) next to render.go/parse.go holding `map[string]Extractor` keyed by name — NOT a method on Registry, which is deliberately a pure data structure ("No Validate/Resolve… pure data structure", registry.go header) — is the consistent choice. Extractor selection would then flow through the existing lifecycle: decode → merge → Validate → Resolve → Render (insertion point §5.3) → Execute → new parse stage alongside ParseOutput.

---

## 8. internal/provider/*_test.go conventions

### 8.1 executor_test.go (10 tests) — harness style

- **Real binaries + skip guard**: `mustBin(t, names...)` helper (line 17) does `exec.LookPath` and `t.Skipf` when absent — tests run `cat`, `sleep`, `printenv`, `false`, `echo` from PATH. **No stub scripts are written to tempdirs; no fakes.**
- Numbered doc comments per test ("// 3. THE KEYSTONE — …").
- **Timeout test (the keystone, test 3, lines 54–66)**: `CmdSpec{Command: "sleep", Args: []string{"30"}}` + `Execute(ctx, spec, 200*time.Millisecond, nil)`; asserts `errors.Is(err, context.DeadlineExceeded)` AND `elapsed > 5s` fails ("the process-group kill should have fired within ~3s (WaitDelay)") — proves cmd.Cancel fired and the group died fast. Test 4: parent `context.WithCancel` + goroutine `cancel()` after 150ms ⇒ `context.Canceled`.
- Other behaviors: cat echo (test 1), 1 MiB stdin round-trip (2), separate stderr + non-zero exit `cat /nonexistent` (5), env propagation via `printenv` (6), Stdin=="" doesn't hang (7), command-not-found wrapped start error (8), `false` exit 1 (9).
- **Verbose tests (10, 10b)**: `var buf bytes.Buffer; vb := ui.NewVerbose(&buf, true)`; assert substrings `"DEBUG: command: cat\n"`, `"DEBUG: raw output:\nfeat: hello\n"`, `"DEBUG: payload: 12 bytes"`; security asserts `!strings.Contains(got, "PATH")` / `"API_KEY"` (§19). 10b is the regression: positional delivery (Stdin=="", payload in Args, PayloadBytes set by hand) must still emit the payload line.
- Convention for new executor work: pass `nil` for vb except in verbose tests; `os.Environ()` for Env; `0` or a generous timeout for non-timeout tests.

### 8.2 Neighbor test conventions (brief)

- **render_test.go** (31 tests): golden tables — `TestRender_GoldenPerProvider` (:30) iterates all 6 built-ins asserting exact `wantCmd` + `wantStdin` strings; per-provider byte-for-byte (TestRender_Pi_ByteForByteCommitPi :86); fallback tests (system-prompt prepend, model default, env, flag delivery); `*_PayloadBytes` variants (:218/:239/:259); mutation-safety (`TestRender_DoesNotMutateManifest` :334); mode default/explicit/tooled/error tests (:364–:435); reasoning tokens (:452–:530); RenderMultiTurn golden table + turn-2-no-sys + non-append-gate + session-id stability (:566–:744).
- **builtin_test.go** (21 tests): per-provider field-by-field assertions (PiFields :228, ClaudeFields :290, OpenCodeFields :459, CodexFields :513, CursorFields :550, AgyFields :633); KeysAndCount :199 (six keys); NameMatchesKey :215; `Validate` on every built-in :355; **DecodeParity :369** (marshal→unmarshal round-trip equivalence); FreshEachCall :436; ChromeDisableContract :708; ListModelsCommand :774; rendered-command golden tests incl. tooled (:808, :835).
- **merge_test.go** (11 tests): PartialOverride_OnlyTouchedFieldChanges :40; ExplicitZeroPointerWins :113; NonEmptySliceReplacesWholesale :140; TooledFlagsReplacedWholesale :164; EmptyOrNilSlicePreservesBase :188; EnvKeyByKeyMerge :224; EnvNilOverridePreservesBase :239; DoesNotMutateInputs :252; EmptyOverrideIsIdentity :281; NamePreservedFromBase :296; MergedResultValidates :316.
- **manifest_test.go** (26 tests): Unmarshal full/partial-nil (:30, :111); Unmarshal explicit-zero-non-nil :162; Marshal omits nil pointers :202; Validate matrix (:239–:305, incl. BadSessionMode :294 — new enum fields need an equivalent); ValidateModel FR-R5b matrix (:318–:380); DetectCommand fallbacks (:392–:413); Resolve defaults/preserves/slices-nil/Command-nil (:424–:510).

All are stdlib `testing` (no testify): `t.Fatalf`/`t.Errorf` with "got %q, want %q" phrasing, table tests where natural, plain-Go assertions. `parse_test.go`, `registry_test.go`, `procgroup_windows_test.go`, `referencefiles_test.go` round out the package.

---

## 9. internal/cmd/models.go — Execute call and the bound

`renderModelBlock` (models.go:139) — the `models` command's list probe:

```go
// models.go:139-153
	timeout := 120 * time.Second
	if cfg != nil {
		timeout = cfg.Timeout // bound (FR25 knob; default 120s)
	}
	var vb *ui.Verbose
	if cfg != nil && cfg.Verbose {
		vb = ui.NewVerbose(cmd.ErrOrStderr(), true)
	}
	out, _, err := provider.Execute(cmd.Context(),
		provider.CmdSpec{Command: argv[0], Args: argv[1:], Stdin: ""}, // Env nil ⇒ inherit
		timeout, vb)
```

The bound is `cfg.Timeout` (the FR25 global knob, default 120s — set in config; hardcoded 120s fallback only when cfg is nil). A failure (timeout/not-found/non-zero) is NOT a hard error here: it falls through to the curated FR-D4 table + a one-line stderr notice, exit 0 (FR-L1). Note `cmd.Context()` (cobra's context) is the parent ctx. **All other Execute call sites and their bounds** (useful for the deadline work): `internal/generate/generate.go:374` (`msgTimeout`, per-role override resolved at :301 area, falls back to cfg.Timeout), `internal/generate/multiturn.go:172/183/194`, `internal/generate/workdesc.go:82/131/149` (same msgTimeout), `pkg/stagecoach/stagecoach.go:569` (`cfg.Timeout`, maps DeadlineExceeded → `generate.RescueError{Kind: ErrTimeout}` → exit 124).

---

## 10. Activity / event-stream code existence check — NEGATIVE (as expected)

At HEAD df98a9a: `find . -name 'activity*'` → **no matches** anywhere; `grep -ril 'event.stream|eventstream'` over *.go → **no matches**. No file, type, field, or function touching activity detection or event-stream output exists in the tree. The only "event"-adjacent text is prose in builtin.go (codex: "`--json` (JSONL events) remain fallback channels") and FR-T9 session work — unrelated code. **The v3.5 surface is greenfield; every seam it needs (Execute timeout ctx, buffered stdout capture, Render insertion point, SessionMode-style capability field, per-provider map registry, StderrHint, exit-124 mapping) is documented above.**

---

## Summary of expectation-vs-reality flags

1. **StderrHint location**: brief listed it under parse.go; it is in **executor.go:23–38** (parse.go has no stderr awareness at all — it sees only stdout).
2. **`DEBUG: cause`**: emitted only at **internal/cmd/default_action.go:381** (verbose-only, after the frozen §18.3 rescue recipe); zero occurrences in internal/provider.
3. **Signal escalation**: performed by os/exec via `cmd.WaitDelay` (SIGKILL unix / TerminateProcess windows), not by procgroup code itself — procgroup sets up Cancel + WaitDelay only.
4. **Streaming**: stdout/stderr are fully buffered `bytes.Buffer`s — nothing incremental exists; event-stream output requires replacing `cmd.Stdout = &out` with a tee-writer while preserving the return-strings contract.
5. Everything else (Execute signature, sentinel error values, Setpgid/PGID==PID, Manifest pointer design, SessionMode precedent, merge regimes, Render order + insertion point, builtin declaration style, test harness style, models.go bound, absence of activity code) matched the brief exactly.
