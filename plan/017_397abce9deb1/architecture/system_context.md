# System context — Stagecoach v3.0 (plan 017)

## What this plan implements

v3.0 (PRD §10.5) adds exactly **two features** to an otherwise-complete v2.x codebase:

1. **Self-update — `stagecoach upgrade`** (PRD §9.29, FR-U1–U12, → G26). Install-method-aware,
   **delegate-first** updater: detects how the binary was installed and delegates to that channel's
   native updater; self-swaps ONLY for the direct-binary channel. Supersedes the v2.1 self-update
   rejection recorded in FUTURE_SPEC.md line 101 and PRD Appendix F.
2. **Expanded distribution surface** (PRD §21, → G27): npm wrapper, Winget, Nix flake, mise/asdf
   plugins — on top of the existing Brew/Scoop/AUR/go-install/GitHub-Releases (G9).

**Nothing else changes.** PRD §10.5: "No commit/CAS/rescue/lock/index/provider changes — the feature
is walled off from the generation core." The commit path (§9.1–§9.28) is untouched.

## Codebase state (reality check — what already exists)

The repo is a mature Go module (16 prior plans). v1 + v2.0–v2.9 are **fully implemented**:

- `cmd/stagecoach/main.go` — entrypoint. `var version = "dev"` injected via ldflags;
  `resolveVersion()` enriches bare "dev" with VCS info (`debug.ReadBuildInfo` → `vcs.revision` +
  `vcs.modified`). **This display-version function is NOT comparable**; v3.0 needs a separate
  comparable version for `--check`/direct-swap (FR-U5 step 1, FR-U6).
- `internal/cmd/` — cobra CLI. Root + subcommands: config (init/path/upgrade-config-schema),
  providers, hook/hookexec, integrate (gitalias/lazygit), lock, models, default_action.
  **No `upgrade` command exists** (binary self-update). NOTE: `config upgrade` is a *config-schema*
  migration, unrelated to binary self-update — do not confuse the names.
- `internal/exitcode/exitcode.go` — canonical exit codes 0/1/2/3/5/124. **No `6` (UpdateAvailable).**
  Pattern: `const (...)` block; `For(err)` maps errors → codes; `ExitError` struct forces a code.
- `internal/config/config.go` — `Config` struct (flat, plain-typed, RESOLVED); `Defaults()`;
  `CurrentConfigVersion = 3`. **No `[upgrade]` table** (FR-U10). Adding one is **additive** → per
  FR-B4 it must NOT bump config_version and must NOT emit an upgrade advisory.
- `internal/provider/`, `internal/generate/`, `internal/decompose/`, `internal/git/` — the commit
  core. **Unchanged by v3.0.** `net/http` is used NOWHERE in the repo today (verified) — §19's
  "no network calls on the commit path" holds; `stagecoach upgrade` is the explicit named exception.
- `internal/ui/`, `internal/signal/`, `internal/lock/`, `internal/watchdog/`, `internal/hook(s)/`,
  `internal/integrate/`, `internal/exclude/`, `internal/prompt/` — all present, unchanged.
- `pkg/stagecoach/stagecoach.go` — public library API (`GenerateCommit`, `Decompose`). Self-update is
  **CLI-only** — it does NOT belong in the public API (walled off; FR-U12).
- `providers/*.toml` — 7 reference manifests (pi/claude/agy/opencode/codex/cursor/qwen-code).
- `.goreleaser.yaml` — brews/scoops/aurs (currently `--skip=` for first release). 6 targets:
  linux/darwin/windows × amd64/arm64. Checksums file = `{project}_{version}_checksums.txt`, sha256.
- `.github/workflows/ci.yml` — build/test (ubuntu/macos/windows × go 1.24/1.25) + lint + vulncheck +
  coverage gate (≥85% on git/provider/generate/config).
- `.github/workflows/release.yml` — goreleaser on `v*` tags.
- `go.mod` — minimal: cobra, pflag, go-toml/v2, yaml.v3. `net/http` is stdlib → **no new dep needed**
  for the HTTP client.
- **No** `install.sh`, `package.json`, `flake.nix`, winget manifests, or mise/asdf plugin scripts.

### Conventions to follow (verified in code)

- **Subcommand registration**: define a `*cobra.Command` in `internal/cmd/<name>.go`, register via
  `init()` calling `rootCmd.AddCommand(...)` — **zero edits to root.go** (the hook/integrate/lock/
  providers pattern). Command groups that must run outside a git repo / skip config.Load override
  `PersistentPreRunE` with a no-op (see `internal/cmd/lock.go` — the exact template for `upgrade`,
  which is repo-independent and must NOT trigger config.Load's first-run bootstrap write, FR-B3).
- **Exit codes**: add a `const` + (if needed) a branch in `For()`. Return
  `exitcode.New(exitcode.X, err)` from RunE. `main.go` already maps via `exitcode.For`.
- **Config**: add a field to `Config` + seed in `Defaults()` + decode in `file.go`'s `fileConfig` +
  overlay in `load.go`. `[upgrade]` is a nested struct (Channel, SourceRepo) — global-only (FR-U10:
  per-repo `[upgrade]` is ignored with a `--verbose` note).
- **Flags**: register on `rootCmd.PersistentFlags()` in root.go `init()`; resolve via
  `fs.Changed`/`fs.Get*` in `loadFlags` (zero defaults so `Changed` reflects "user passed it").
  BUT: `stagecoach upgrade`'s own flags (--check/--rollback/etc.) are LOCAL to the upgrade command,
  not persistent global flags — they must NOT pollute the commit-path flag set. Register them on the
  upgrade command's local `Flags()`.
- **Errors**: typed sentinel errors (`errors.Is`) + `ExitError` for forced codes. See
  `internal/exitcode/exitcode.go`.
- **Tests**: table-driven; git tests use a temp repo + real git binary; provider tests use compiled
  stubs in `cmd/stub*`/`internal/stubtest`. Cross-platform via build tags + compiled stubs.
- **UI**: progress → stderr (`internal/ui/output.go`, `verbose.go`); `--verbose` is FR50.

## Ordering rationale (Phase 1 → Phase 2)

- **Phase 1 (self-update) implements install-method detection + delegation for ALL 9 channels**
  (FR-U2 cascade: brew, scoop, winget, pacman/AUR, npm, mise, asdf, Nix, go-install, direct;
  FR-U3 delegation table) **up front** — detection just queries package-manager DBs / path
  heuristics; it does not require the channel to be "published", only detectable if present.
- **Phase 2 (distribution) populates the channels**: the npm wrapper SETS `STAGECOACH_INSTALL_METHOD=npm`
  (FR-U2 last bullet) which Phase 1's npm-detection branch already reads; flake.nix/winget/mise/asdf
  are additional install paths. Phase 2 is largely independent release/CI plumbing.
- So: Phase 1 (full-channel detection) is the foundation; Phase 2 channels are independently
  shippable but logically follow. Phase 3 = changeset-level docs (Mode B).

## Public surface added by v3.0

- CLI: `stagecoach upgrade [--check|-c] [--version <v>] [--prerelease] [--force] [--rollback]
  [--install-method <m>] [--yes|-y] [--channel <stable|prerelease>] [--source-repo <owner/repo>]`.
- Exit code `6` = UpdateAvailable (`--check` found a newer version).
- Config: `[upgrade]` table (`channel`, `source_repo`) — global only.
- Env: `STAGECOACH_INSTALL_METHOD`, `STAGECOACH_GITHUB_TOKEN` (optional auth), `GITHUB_TOKEN`
  (inherited fallback).
- Git-config: none (upgrade is global, not per-repo).
- New files: `internal/upgrade/*` (the package), `internal/cmd/upgrade.go`,
  `package.json` + `install.mjs`/`install.sh` (npm wrapper), `flake.nix`, winget manifest(s),
  mise/asdf plugin scripts, `.github/workflows/*.yml` additions.
