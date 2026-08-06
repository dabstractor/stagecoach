# Windows test support

Stagecoach's test suite runs natively on Windows (`windows-latest` is in the CI
`build-test` matrix alongside `ubuntu-latest` and `macos-latest`). This note
captures the cross-platform patterns the test infrastructure relies on, so future
contributions keep the suite green on all three OSes.

## Compiled stubs (no shell scripts as executables)

Several tests historically drove git/provider behavior through `#!/bin/sh` scripts
invoked directly via `exec.Command`. On Windows `CreateProcess` ignores the shebang,
so those scripts cannot run. They are replaced by tiny compiled Go binaries under
`cmd/`, built once per test process and cached by helpers in `internal/stubtest`:

| binary              | helper                     | replaces                          |
|---------------------|----------------------------|-----------------------------------|
| `cmd/stubagent`     | `stubtest.Build`           | the fake agent (pre-existing)     |
| `cmd/stubeditor`    | `stubtest.BuildEditor`     | `fakeeditor.sh` (--edit gate)     |
| `cmd/stubtee`       | `stubtest.BuildTee`        | `tee-wrap.sh` (multi-turn capture)|
| `cmd/stubcli`       | `stubtest.BuildCLI`        | provider stubs (`models` live-list)|
| `cmd/stubarbiter`   | `stubtest.BuildArbiter`    | `arbiter.sh` (decompose)          |
| `cmd/stubcapture`   | `stubtest.BuildCapture`    | `capture.sh` (decompose planner)  |

All stub builds pass `-buildvcs=false` so a test that `chdir`s into a temp git
repo doesn't trigger VCS stamping (which fails with exit 128).

## Git hooks

`#!/bin/sh` git hooks work on Windows because Git for Windows ships `sh.exe` and
the hooks runner (`internal/hooks/runner.go`) parses the shebang and routes the
script through `sh` (falling back to `sh` for shell-family shebangs). When a hook
body embeds a native path (e.g. `touch <marker>`), use a forward-slash form (MSYS
`sh` accepts `C:/...` style) — see `shellPath` in `internal/hooks`. Git's own
binary is exposed to isolated-PATH tests via its install dir (not a copied
`git.exe`, which can't find its sibling DLLs — `putGitOnPath`).

## Platform differences handled in tests

- **No executable bit on Windows.** `os.Stat` reports `0666` for every regular
  file, so `0o755` perm assertions and `hookExecutable`'s non-exec-skip are
  Unix-only (guarded with `runtime.GOOS == "windows"`).
- **`os.UserHomeDir` reads `%USERPROFILE%`, not `$HOME`.** Tests that assert the
  resolved home/cache path set both (see `setHomeEnv` in `internal/lock`).
- **Mandatory file locking.** A held fd blocks `RemoveAll`, so test probe locks
  are released before `t.TempDir` cleanup; concurrent git index operations use
  plumbing (`commit-tree`/`update-ref`) instead of porcelain `commit` to avoid
  `.git/index.lock` collisions.
- **`os.Symlink` needs a privilege.** Symlink-based tests skip when the symlink
  fails with a privilege error (GitHub Actions `windows-latest` has Developer
  Mode enabled, so they run there).

## Genuinely Unix-only tests

A few tests verify Unix-only *behavior* and are `//go:build !windows` with a
documented reason (not blanket skips): flock contention (`lock_windows.go` is a
documented no-op; the CAS is the safety guarantee), and the non-executable-hook
skip (Windows has no executable bit).
