# Stagecoach

> **Stagecoach writes your commit messages using the AI agent you already pay for.**

No API key. No per-token billing. It shells out to Claude Code, Codex, pi, opencode, agy,
or Cursor — whatever you already have installed — and spends your existing coding-plan quota
instead. Stage while it thinks; it commits only what was staged when it started, atomically, and
can never corrupt your repo. With a dirty working tree and nothing staged, it automatically
decomposes your changes into a sequence of logically-coherent commits.

[View on GitHub](https://github.com/dabstractor/stagecoach){.md-button.md-button--primary }

---

## Install

```bash
# Direct binary (curl|sh one-liner from GitHub Releases)
curl -fsSL https://github.com/dabstractor/stagecoach/raw/main/install.sh | bash

# Homebrew (macOS / Linuxbrew)
brew install dabstractor/stagecoach/stagecoach

# Go install (anywhere with Go)
go install github.com/dabstractor/stagecoach/cmd/stagecoach@latest

# Windows (Scoop)
scoop bucket add stagecoach https://github.com/dabstractor/stagecoach-bucket
scoop install stagecoach/stagecoach
```

See the [overview](overview.md) for Chocolatey, npm, Nix, mise/asdf, the apt repo, and the dnf repo.

---

## Why not opencommit / aicommits?

The incumbent tools own the HTTP call to the model, so they can normalize providers, handle
retries, and abstract auth. But once you own the HTTP call, **you cannot use a coding-plan
subscription** — that quota is gated behind your agent's CLI and is not reachable over the public
API.

Stagecoach inverts the architecture: it shells out to your installed CLI agent, trading provider
normalization for **quota reuse**. The agent brings its own auth and billing. That trade-off —
give up control of the model call in exchange for access to the user's existing quota — is the
entire product.

---

## Features

- **No API key, no per-token billing** — reuses your agent's existing coding-plan quota.
- **Snapshot-based, atomic commits** — stage while it thinks; never corrupts your repo.
- **Multi-commit decomposition** — auto-splits a dirty tree into a sequence of logical commits.
- **Per-role model routing** — planner / stager / message / arbiter, each on the right model.
- **22-field provider manifests** — pi, Claude Code, Codex, Cursor, opencode, agy.
- **Git hook mode + IDE integrations** — lazygit and a git alias, with a no-mangle write protocol.
- **Message shaping** — `--format` (conventional/gitmoji/plain; append `+body` for a subject+body), `--locale`, `--template`.
- **Self-update** — `stagecoach upgrade` delegates to your package manager; never fights it.