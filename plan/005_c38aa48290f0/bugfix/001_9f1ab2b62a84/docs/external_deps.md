# External Dependencies

## Current Dependencies (go.mod)
- `github.com/pelletier/go-toml/v2` v2.4.2 — TOML parsing for config files
- `github.com/spf13/cobra` v1.10.2 — CLI framework
- `github.com/spf13/pflag` v1.0.10 — flag parsing (cobra dependency)
- `gopkg.in/yaml.v3` v3.0.1 — YAML parsing for lazygit config

## Constraints
- **NO new dependencies.** The project is deliberately minimal.
- Platform-specific code uses **stdlib-only** `syscall` package (see `procgroup_unix.go`, `procgroup_windows.go`, `signal_unix.go`).
- `golang.org/x/term` and `golang.org/x/sys` are NOT available and MUST NOT be added for the IsTerminal fix. Use raw `syscall.Syscall(SYS_IOCTL, ...)` instead.

## Verified Platform Constants (stdlib syscall)
- **Linux**: `syscall.TCGETS` (0x5401) + `syscall.SYS_IOCTL` — both in Go stdlib
- **Darwin/macOS**: `syscall.TIOCGETA` (0x40487413) + `syscall.SYS_IOCTL` — both in Go stdlib
- **Windows**: `syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleMode")` — same pattern as `procgroup_windows.go`

## Supported Build Targets (per .goreleaser.yaml)
- linux/amd64, linux/arm64
- darwin/amd64, darwin/arm64
- windows/amd64, windows/arm64
- CGO_ENABLED=0 (pure Go, static binary)
