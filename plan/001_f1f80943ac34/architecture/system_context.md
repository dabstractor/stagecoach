# System Context

## 1. Repository state

The target repo `/home/dustin/projects/stagehand-hack` is **greenfield**. It contains:

- `PRD.md` (92 KB, the full specification — self-contained with appendices)
- `plan/001_f1f80943ac34/` (this planning workspace: `prd_snapshot.md`, `architecture/`, `artifacts/`, `prps/`)
- `.git` (a gitfile pointing elsewhere; effectively an empty working tree)

There is **no Go code, no `go.mod`, no `go.sum`, no config, no Makefile** yet. Every package in PRD §14 is to be created from scratch. This is a clean-room implementation, not a refactor.

## 2. Toolchain & environment (verified on host, 2026-06-30)

| Component | Required (PRD) | Installed | OK? |
|---|---|---|---|
| Go | ≥ 1.22 | **go1.26.4** (linux/amd64) | ✅ |
| git | ≥ 2.20 | **2.54.0** | ✅ |
| `pi` | provider target | **0.80.2** | ✅ |
| `claude` (Claude Code) | provider target | **2.1.69** | ✅ |
| `gemini` (Gemini CLI) | provider target | **0.19.4** | ✅ |
| `opencode` | provider target | **1.1.23** | ✅ |
| `codex` (Codex CLI) | provider target | **codex-cli 0.142.4** | ✅ |
| `agent` (Cursor Agent) | provider target | **2026.06.26-7079533** | ✅ |

All six agents are present on `$PATH` at `/home/dustin/.local/bin/` (and `cursor` at `/usr/bin/cursor`). This **validates PRD §22.2 Assumption 1** ("user has at least one supported agent installed") for the author's machine and means every manifest can be verified against a real end-to-end run during implementation (PRD §20.1 layer 4, `//go:build integration_real`).

The PRD's `pi` provider routing (`--provider zai`, model `glm-5-turbo` / `glm-5.2`) is the author's existing GLM configuration in `commit-pi` and is user-specific. `pi --provider zai` resolves GLM models (observed `z-ai/glm-4.6` family). The manifest leaves `default_provider=""`; the user sets it.

## 3. Build & dependency posture (from PRD §22.3, confirmed feasible)

- Go modules, single static binary.
- `cobra` for CLI/subcommands (`providers`, `config`). Recommend.
- `pelletier/go-toml/v2` for config parsing.
- **No `go-git`.** Shells out to the real `git` binary (matches reference impl, guarantees identical semantics). Confirmed git 2.54 present.
- `goreleaser` for cross-platform release (linux/darwin/windows × amd64/arm64), Homebrew tap, Scoop, AUR, checksums.

All four deps are mature, stable, and widely used with Go 1.22+. Go 1.26 on the host is a superset. No version conflicts anticipated.

## 4. The reference implementation (CRITICAL — read `reference_impl.md`)

The PRD is a *port-and-generalize* of two proven zsh scripts at `/home/dustin/projects/git-scripts/`:
- `commit-pi` (9517 bytes) — daily-driver, the source of behavioral truth.
- `commit-claude` (8667 bytes) — the fork that motivates the provider abstraction.

These exist and were read in full. **Their behavior is the ground truth for the generate/loop/rescue/git-plumbing logic.** See `reference_impl.md` for the line-by-line behavioral contract and the **discrepancies between the reference and the PRD** that the implementation must reconcile (loop nesting, payload ordering, claude bare-flags, JSON→raw contract change, auto-stage-all being new).

## 5. Scope of this plan

This plan decomposes the **v1.0 ship list** (PRD §10.1): everything marked P0 or P1 in PRD §9. It produces a single Go binary `stagehand` with six built-in provider manifests, the snapshot-based atomic-commit core, style learning + anti-duplicate, config precedence, `providers`/`config` subcommands, `--dry-run`/`--verbose`, and distribution scaffolding. Multi-commit decomposition (v2, PRD §10.3) is explicitly out of scope but the architecture must not preclude it (PRD §11.3).
