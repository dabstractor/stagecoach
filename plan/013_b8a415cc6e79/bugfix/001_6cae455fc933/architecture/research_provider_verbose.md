# Research: Provider Executor & Verbose Layer (Issues 4, 5, 6)

All claims verified against current source with exact line numbers.

## Issue 4 — `STAGECOACH_VERBOSE=2` rejected (CONFIRMED)

### Root cause: `Config.Verbose` is `bool`
config.go:70: `Verbose bool`. load.go:246-251 parses via `strconv.ParseBool` (rejects "2").
CLI flag root.go:166 is `BoolVarP`. No `VerboseLevel` int concept exists.

### VERBOSE=2 "deferred" comments
verbose.go:30-33: "deferred to a future VERBOSE=2 — Config.Verbose is a bool, so VERBOSE=2 is
currently un-parseable and out of scope"
verbose.go:62-64: "would require Config.Verbose to become an int; currently it is a bool and
ParseBool('2') errors"

### Scope map for promoting to int (OUT OF SCOPE for bugfix — for reference)
| File | Site |
|------|------|
| config.go:70 | `Verbose bool` → `int` |
| config.go:190 | `Defaults(): Verbose: false` → `0` |
| load.go:246-251 | `ParseBool` → parse int |
| load.go:356-359 | `fs.GetBool` → `GetInt` |
| cmd/root.go:166 | `BoolVarP` → `IntVar` |
| ui/verbose.go:35-42 | `on bool` + `NewVerbose(w, on bool)` → level int |
| file.go:46,224-225,346-347 | `Verbose bool` + overlay |
| git.go:163-166 | `gitConfigBool` |
| Callers: stagecoach.go:148,212; default_action.go:399; hookexec.go:122; models.go:146-148 |

**Minimal fix:** special-case "2" in loadEnv with clear message ("not yet supported").

## Issue 5 — payload-size line missing for positional/flag (CONFIRMED)

### The call site
executor.go:64: `vb.VerbosePayload(len(spec.Stdin))` — the ONLY call site.

### Delivery-mode switch (render.go:163-178)
```go
switch *r.PromptDelivery {
case "stdin":      spec.Stdin = payload
case "positional": spec.Args = append(spec.Args, payload)
case "flag":       spec.Args = append(spec.Args, *r.PromptFlag, payload)
}
```
Identical switch at render.go:274-282 (RenderMultiTurn).

For positional/flag: `spec.Stdin == ""` → `len(spec.Stdin) == 0` → `VerbosePayload(0)` → no-op.

### VerbosePayload guard (verbose.go:94-100)
```go
func (v *Verbose) VerbosePayload(bytes int) {
    if v == nil || v.w == nil || !v.on || bytes <= 0 { return }
    fmt.Fprintf(v.w, "DEBUG: payload: %d bytes (~%d tokens est)\n", bytes, (bytes+3)/4)
}
```

### CmdSpec (render.go:22-29) — intentionally does NOT carry delivery mode
```go
type CmdSpec struct {
    Command string
    Args    []string
    Stdin   string   // payload for stdin delivery; "" → os.DevNull
    Env     []string
}
```

### Delivery mode constants (manifest.go:11,19)
`DefaultPromptDelivery = "stdin"`; valid: stdin, positional, flag.

### Provider delivery modes (builtin.go)
- **stdin**: pi, claude, agy, qwen, opencode, codex
- **positional**: cursor
- **flag**: no builtin (user-defined only)

### Clean fix
Add `PayloadBytes int` to CmdSpec. Set `spec.PayloadBytes = len(payload)` in BOTH Render delivery
switches. Change executor.go:64 to `vb.VerbosePayload(spec.PayloadBytes)`.

## Issue 6 — --model shadowed by per-role config (CONFIRMED, by-spec)

### Precedence path
1. `--model` sets `cfg.Model` (global) — load.go:353-355
2. `[role.message] model` sets `cfg.Roles["message"].Model` — file.go overlay
3. `ResolveRoleModel("message", cfg)` (roles.go:34-57) — per-role beats global:
```go
if rc.Model != "" { model = rc.Model }   // per-role wins
if model == "" { model = cfg.Model }     // global as FALLBACK only
```

### FR-R5b validates the RESOLVED (shadowing) model
default_action.go:176-205: `ResolveRoleModel("message")` → `ValidateModel(labelModel)`.
When per-role model is set, the bare `--model` is never validated or used. No error, no warning.

### No shadow hint exists (grep-confirmed)
No `shadow`/`override-notice` diagnostic anywhere in `internal/`.

### Fix location
default_action.go ~line 176 (after ResolveRoleModel). Check `fs.Changed("model")` AND
`cfg.Roles["message"].Model != ""` → `vb.VerboseWarn("note: --model shadowed by [role.message].model; use --message-model to override")`.
Verbose sink available at default_action.go:399.
