# Signal Handler Architecture — Extension Points for FR-K3 + FR-K1

## Current state (internal/signal/signal.go)

The signal handler is a singleton (`var active atomic.Pointer[Handler]`) installed once in `main.go`.
It catches `{SIGINT, SIGTERM}` via `signal.Notify(h.ch, os.Interrupt, syscall.SIGTERM)` at line 103.

### Key structures

```go
// signal.go:26-50
type Options struct {
    RescueFormat func(treeSHA, parentSHA, candidate string) string
    Out          io.Writer
    Kill         func(pid int, sig os.Signal) error
    Exit         func(int)
    OnRescueExit func()  // exit-path lock-release seam (wired to lock.ReleaseCurrent in main.go)
}

// signal.go:52-69
type Handler struct {
    opts   Options
    ch     chan os.Signal
    cancel context.CancelFunc
    childPID atomic.Int64
    mu            sync.Mutex
    snapTree, snapParent, snapCandidate string
    stopped atomic.Bool
}
```

### Install (signal.go:76-106)
```go
func Install(parent context.Context, opts Options) (context.Context, *Handler) {
    // ... default opts ...
    ctx, cancel := context.WithCancel(parent)
    h := &Handler{opts: opts, cancel: cancel, ch: make(chan os.Signal, 1)}
    signal.Notify(h.ch, os.Interrupt, syscall.SIGTERM)  // ← LINE 103: FR-K3 EXTENDS HERE
    active.Store(h)
    go h.run()
    return ctx, h
}
```

### run goroutine (signal.go:111-119)
```go
func (h *Handler) run() {
    for sig := range h.ch {
        h.handle(sig)
        return  // exit on first signal (handler calls Exit; test fakes return instead)
    }
}
```

### handle — the single rescue/exit routine (signal.go:122-155)
```go
func (h *Handler) handle(sig os.Signal) {
    if h.stopped.Load() { return }  // RestoreDefault already called → no-op
    if pid := h.childPID.Load(); pid > 0 { _ = h.opts.Kill(int(pid), sig) }
    h.cancel()
    h.mu.Lock()
    tree, parent, cand := h.snapTree, h.snapParent, h.snapCandidate
    h.mu.Unlock()
    if tree != "" {
        fmt.Fprintln(h.opts.Out, h.opts.RescueFormat(tree, parent, cand))
        h.opts.OnRescueExit()  // ← release lock before exit — BOTH branches
        h.opts.Exit(3)
        return
    }
    h.opts.OnRescueExit()
    h.opts.Exit(exitCodeForSignal(sig))  // 130 SIGINT / 143 SIGTERM
}
```

### RestoreDefault (signal.go:207-215)
```go
func RestoreDefault() {
    if h := active.Load(); h != nil {
        if h.stopped.CompareAndSwap(false, true) {
            signal.Stop(h.ch)
            close(h.ch)
        }
    }
}
```

### exitCodeForSignal (signal_unix.go:20-30)
```go
func exitCodeForSignal(sig os.Signal) int {
    switch sig {
    case os.Interrupt, syscall.SIGINT: return 130
    case syscall.SIGTERM:              return 143
    default:                           return 1
    }
}
```

## FR-K3 extension (SIGHUP)

### 1. Add platform-specific signal set
Replace `signal.Notify(h.ch, os.Interrupt, syscall.SIGTERM)` with:
```go
signal.Notify(h.ch, caughtSignals()...)
```

**signal_unix.go** (`//go:build !windows`):
```go
func caughtSignals() []os.Signal {
    return []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}
}
```

**signal_windows.go** (`//go:build windows`):
```go
func caughtSignals() []os.Signal {
    return []os.Signal{os.Interrupt, syscall.SIGTERM}
}
```

### 2. Add SIGHUP exit code
**signal_unix.go** — add to `exitCodeForSignal`:
```go
case syscall.SIGHUP: return 129  // 128 + 1
```

### Why no handle()/run()/RestoreDefault changes needed
- `run()` is signal-agnostic: it forwards whatever signal arrived to `handle(sig)`.
- `handle(sig)` is signal-agnostic: it forwards, cancels, rescues-or-exits.
- `RestoreDefault()`'s `signal.Stop(h.ch)` disarms ALL signals in the Notify set, so SIGHUP is disarmed for free.

## FR-K1 extension (signal.Trigger export)

The watchdog's polling fallback detects parent death but needs to programmatically invoke the
rescue/exit path. `handle` is unexported, so a new exported wrapper is needed:

```go
// Trigger routes a synthetic signal through the rescue path (parent-death watchdog).
// No-op when no handler installed or after RestoreDefault (stopped guard in handle()).
// This is the ONLY new public API the watchdog needs from the signal package.
func Trigger(sig os.Signal) {
    if h := active.Load(); h != nil {
        h.handle(sig)  // reuses exact same forward→cancel→rescue→exit logic
    }
}
```

The watchdog calls `signal.Trigger(syscall.SIGTERM)` on parent death. This:
- Respects the `stopped` guard (first line of `handle()`) — no-op past `RestoreDefault`.
- Calls `OnRescueExit()` (= `lock.ReleaseCurrent`) before exit — lock file removed.
- Routes through rescue if snapshot armed (exit 3) or plain exit (143 for SIGTERM).
- Is stdlib-only (no new imports in signal package).

### Alternative: Linux prctl fast path needs NO signal package change
`prctl(PR_SET_PDEATHSIG, SIGTERM)` makes the kernel deliver a real SIGTERM to the process on
parent death. SIGTERM is already in the caught set, so it flows through `run`→`handle` naturally.
The signal package is unaware the SIGTERM came from the kernel rather than a terminal.
