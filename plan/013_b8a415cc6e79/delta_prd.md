# Delta PRD — Provider Lineup Correction (remove EOL `gemini`; re-verify `agy` v1.1.0)

| Field | Value |
| --- | --- |
| **Delta scope** | Two related content corrections to the Stagecoach provider lineup, with no new product surface |
| **Base PRD** | `PRD.md` (v2.6 spec) |
| **Date** | 2026-07-09 |
| **Previous session** | plan/012_963e3918ec08 — completed the `stagehand` → `stagecoach` rename (Phase P1, all milestones Complete) |
| **Implementation status** | **Code is already implemented and committed** (`2f77bd0`, `010ecee`, `cdbccf5`). Build, `go vet`, and the provider/config/cmd test suites pass. The delta's remaining work is **documentation sync only** (residual drift in `docs/providers.md`). |

---

## What changed (diff summary)

A focused diff of the two PRDs reveals exactly one theme: **the provider lineup is corrected.** Everything below is a consequence of these two facts.

### 1. The `gemini` (Gemini CLI) built-in provider is REMOVED

Gemini CLI was superseded by **`agy`** (the Antigravity CLI) on 2026-06-18 and is no longer shipped. The PRD propagates this removal across ~13 locations: §12.5 becomes a "REMOVED" stub; `gemini` is dropped from the §12 terminology/examples lists ("Providers with a fixed backend …"), §6.1 **G2** (`Gemini CLI` → `agy` as one of the four shipped agents), §7.1 persona ("runs `gemini`" → "runs `agy`"), §9.16 **FR-D1** cascading priority order, **FR-D3** tier-strategy mention, **FR-D4** default-model table (the `gemini` row is deleted), §10.1 v1.0 shipped-manifest list, §14 package layout (`providers/gemini.toml` removed), §15.3 `providers list` default order, §22.2 assumptions, Appendix D quick reference (the `gemini` row deleted), and Appendix B.5 rescue example (`gemini-3.1-pro in gemini` → `Gemini 3.5 Flash (Low) in agy`).

### 2. The `agy` manifest is CORRECTED against real end-to-end verification

The previous `agy` manifest was authored from docs and inherited the gemini-cli lineage's flags. Verified against `agy --help` + live stdin runs on **2026-07-08 (agy v1.1.0)**, agy has **diverged** from that lineage. §12.5.1's manifest is rewritten:

- `model_flag`: `-m` → **`--model`** (agy v1.1.0 rejects `-m`).
- `print_flag`: `-p` → **`""`** (non-nil empty). `-p`/`--print`/`--prompt` is **value-taking** in v1.1.0, so a bare `-p` fails with "flag needs an argument"; agy reads the prompt from **stdin** when `-p` is absent.
- `default_model`: `gemini-3.1-pro` → **`Gemini 3.5 Flash (Low)`** — agy's `--model` takes the **`agy models` display label verbatim** (reasoning suffix included; API-style ids are silently ignored).
- `bare_flags`: `["--approval-mode", "default"]` → **`["--mode", "plan"]`** — `--approval-mode` was removed; `--mode plan` is the read-only equivalent (verified to emit clean commit messages).
- Adds `list_models_command = ["agy", "models"]` and `experimental = true`.
- §12.5.1.1 status flips: items **1–3 RESOLVED** (non-TTY stdout drop / model flag / prompt delivery + read-only mode); item **4 OPEN** (tooled/stager flags — agy still cannot serve as a stager).
- §22.1 risk row: the `agy` non-TTY stdout drop (issue #76) is marked **RESOLVED 2026-07-08**; agy stays `experimental` only for the unrelated item 4.
- §12.5.2 (qwen-code): clarified that it mirrors the **gemini-lineage**, NOT agy (which diverged); qwen-code's own flags remain `# TO CONFIRM`.

### Sizing

This is a **medium-small delta** — a provider removal plus a manifest correction, both already implemented, propagated across documentation. Per the proportional-sizing rule, the PRD carries **1 phase, 2 milestones, and one cross-cutting doc-sync task**. It does not touch the snapshot/CAS/rescue/lock/decompose/config-precedence cores.

---

# Phase P2 — Provider lineup correction

**Goal.** The built-in provider set reflects reality: `gemini` is gone; `agy` is correct against its actual v1.1.0 flag surface; every cross-reference (priority order, default-model table, package layout, assumptions, appendices, docs) agrees.

**Why now.** agy superseded gemini-cli on 2026-06-18; the gemini built-in is dead surface. agy's v1.1.0 divergence (value-taking `-p`, no `--approval-mode`, `--model`-only) meant the inherited manifest silently mis-rendered every `agy` invocation. Both are correctness fixes, not features.

**Relationship to the rename session (P1).** P1 (plan/012) renamed the project `stagehand` → `stagecoach` but did **not** touch provider lineup. This phase is independent of P1 and composes cleanly on top of it.

**Implementation note (read first).** The code-side work is **already done and committed** — verify, do not redo:
- `2f77bd0 Re-verify and fix agy manifest against v1.1.0` → `internal/provider/builtin.go`, `internal/provider/builtin_test.go`, `providers/agy.toml`
- `010ecee Remove gemini-cli provider, switch opencode to stdin delivery` → `internal/cmd/*`, `config.go`, `README.md`, `docs/*.md` (partial), test files
- `cdbccf5 Purge gemini-cli from PRD after built-in removal` → `PRD.md`

`go build ./...`, `go vet ./...`, and `go test ./internal/provider/... ./internal/config/... ./internal/cmd/...` all pass. The implementer's job is therefore: (a) confirm the code matches the corrected spec (light verification), and (b) finish the **residual documentation drift** that the commits did not fully sweep (see P2.M3).

---

## Milestone P2.M1 — Remove the `gemini` (Gemini CLI) built-in provider

The `gemini` provider is no longer shipped. Confirm its removal from the registry and all cross-references, and ensure no stale "gemini as a built-in" framing survives in shipped docs. (The word "gemini" legitimately persists as **model names** — e.g. `gemini-3.5-pro`, `Gemini 3.5 Flash` — because `agy` runs the Gemini model family; those are correct and must NOT be touched. The remaining `gemini` strings in `internal/config/*_test.go` and `internal/cmd/models_test.go` are arbitrary provider-name / model-string fixtures exercising config-resolution mechanics, also correct.)

### Task P2.M1.T1 — Confirm `gemini` is removed from the built-in set and all references

- **Subtask P2.M1.T1.S1 — Verify code-side removal.**
  - INPUT: the committed codebase (`2f77bd0`, `010ecee`, `cdbccf5`).
  - LOGIC: confirm (a) `internal/provider/builtin.go` defines exactly **seven** built-ins — pi, claude, agy, qwen-code, opencode, codex, cursor — and has **no** `gemini` `Name:` entry (only historical references in comments noting gemini-cli is EOL); (b) `internal/provider/registry.go` `preferredBuiltins` (FR-D1) is `["pi", "opencode", "cursor", "agy", "qwen-code", "codex", "claude"]` with no `gemini`; (c) `providers/gemini.toml` does **not** exist; (d) `providers/agy.toml`, `providers/qwen-code.toml` reference gemini only in comments (lineage / model-family).
  - OUTPUT: a one-line confirmation per item. No code change expected; if anything is found, restore the removed state per the spec.
  - DOCS: none (code-verification).

- **Subtask P2.M1.T1.S2 — Confirm PRD §12.5 carries the REMOVED stub and cross-references are clean.**
  - INPUT: `PRD.md`.
  - LOGIC: confirm §12.5 reads "~~Built-in provider: Gemini CLI~~ — REMOVED (superseded by agy, §12.5.1)"; confirm `gemini` is dropped from §12 fixed-backend list, §6.1 G2, §7.1 persona, §9.16 FR-D1/FR-D3/FR-D4 (no gemini row), §10.1, §14 (no `gemini.toml`), §15.3, §22.2, Appendix D (no gemini row), Appendix B.5.
  - OUTPUT: confirmation; this is already the committed state (`cdbccf5`).
  - DOCS: the PRD edits themselves **are** this subtask's documentation surface (Mode A) — they are the spec-of-record for the removal.

---

## Milestone P2.M2 — Correct the `agy` manifest against agy v1.1.0 (2026-07-08)

agy diverged from the gemini-cli lineage; the inherited manifest mis-rendered invocations. The corrected manifest is already in `builtin.go`/`providers/agy.toml`; confirm it matches the spec and the verification status is recorded.

### Task P2.M2.T1 — Confirm the corrected `agy` manifest and verification status

- **Subtask P2.M2.T1.S1 — Verify the `agy` manifest fields in code.**
  - INPUT: `internal/provider/builtin.go` `builtinAgy()` (around line 200) and `providers/agy.toml`.
  - LOGIC: confirm the manifest matches §12.5.1 exactly: `PrintFlag = strPtr("")` (non-nil empty), `ModelFlag = strPtr("--model")`, `PromptDelivery = "stdin"`, `DefaultModel` = the `"Gemini 3.5 Flash (Low)"` display label, `BareFlags = ["--mode", "plan"]` (no `--approval-mode`), `ListModelsCommand = ["agy", "models"]`, `Experimental = true`, `TooledFlags` nil (cannot stager until item 4). Confirm `internal/config/role_defaults.go` agy defaults are the `Gemini 3.5 Flash (High/Medium/Low)` display labels.
  - OUTPUT: confirmation that code == spec. The corrected render is `agy --model "<label>" --mode plan < <sys+user payload via stdin>` (no `-p`).
  - DOCS: the manifest comments in `builtin.go`/`providers/agy.toml` carry the 2026-07-08 verification record — they are the in-code documentation (Mode A); confirm they are present and dated.

- **Subtask P2.M2.T1.S2 — Confirm PRD §12.5.1/§12.5.1.1/§22.1 reflect verification.**
  - INPUT: `PRD.md`.
  - LOGIC: confirm §12.5.1 shows the corrected manifest; §12.5.1.1 marks items 1–3 RESOLVED (2026-07-08, agy v1.1.0) and item 4 OPEN; §22.1 risk row marks the non-TTY stdout drop RESOLVED 2026-07-08; §12.5.2 qwen-code notes agy diverged. This is already the committed PRD state.
  - OUTPUT: confirmation.
  - DOCS: the PRD sections themselves are this subtask's documentation surface (Mode A).

---

## Milestone P2.M3 — Sync changeset-level documentation (Mode B)

The three implementation commits swept code, the PRD, `README.md`, and **partially** `docs/*.md`, but did **not** fully reconcile `docs/providers.md` with the corrected `agy` manifest or the removal of `gemini`. This milestone depends on P2.M1 and P2.M2 (it only makes sense once the lineup correction is confirmed) and closes the documentation drift so the shipped docs are not stale.

### Task P2.M3.T1 — Fix residual `docs/providers.md` drift

This is the **only remaining concrete work** in the delta. Four specific, small edits in `docs/providers.md`:

- **Subtask P2.M3.T1.S1 — Correct the `agy` quick-reference table row.**
  - INPUT: `docs/providers.md` ~line 85, the providers quick-reference table.
  - LOGIC: the `agy` row currently reads `| \`agy\` | stdin | \`-p\` | \`--model\` | \`Gemini 3.5 Flash (Low)\` | (prepended) | Read-only constraint (\`--approval-mode default\`) | — no |`. Two cells are stale against the verified manifest (§12.5.1): the **print** cell `\`-p\`` should be **`—`** (agy v1.1.0's `-p` is value-taking; `PrintFlag=""` — agy reads stdin, no print flag); the **bare essentials** cell should read `Read-only constraint (\`--mode plan\`)` (`--approval-mode` was removed). Delivery (`stdin`), model flag (`--model`), default model (`Gemini 3.5 Flash (Low)`), sys-prompt (prepended), and stager (`— no`) cells are already correct.
  - OUTPUT: the `agy` row matches the corrected manifest exactly.
  - DOCS: this edit **is** the documentation update (Mode A target file = `docs/providers.md`).

- **Subtask P2.M3.T1.S2 — Fix the verification date and the provider count.**
  - INPUT: `docs/providers.md` ~line 88 (note paragraph) and ~line 92 ("Tools-disable asymmetry" intro).
  - LOGIC: (a) The note says the #76 non-TTY stdout drop "no longer reproduces as of **2026-07-03**"; align it with the PRD's verified date **2026-07-08 (agy v1.1.0)**, and optionally reframe that agy stays `experimental` only pending item 4 (stager flags). (b) The "Tools-disable asymmetry" intro says "**eight** built-in providers"; with `gemini` removed there are **seven** — change "eight" → "seven".
  - OUTPUT: the date matches the PRD; the provider count is seven.
  - DOCS: this edit **is** the documentation update (Mode A target file = `docs/providers.md`).

- **Subtask P2.M3.T1.S3 — Final cross-reference grep + smoke test.**
  - INPUT: the full repo (excluding `plan/`).
  - LOGIC: (a) `git grep -in 'gemini' -- docs/ README.md providers/` — every remaining hit must be either a **model name** (e.g. `gemini-3.5-pro`, `Gemini 3.5 Flash`) or a **lineage/history comment** (EOL, superseded, fork), never a statement that `gemini` is a shipped built-in. Fix any that imply otherwise. (b) Confirm `docs/cli.md`, `docs/configuration.md`, `docs/how-it-works.md`, `docs/README.md` do not list `gemini` as a built-in (the `010ecee` commit touched these; verify no regression). (c) Smoke test: `make build && ./bin/stagecoach providers list` lists exactly seven built-ins with no `gemini`; `./bin/stagecoach providers show agy` prints `print = ""` (or equivalent), `model_flag = "--model"`, `--mode plan` in bare essentials.
  - OUTPUT: grep is clean of stale `gemini`-as-built-in framing; `providers list`/`show agy` match the spec.
  - DOCS: Mode B final verification — confirms the whole delta is coherent across PRD, code, manifests, and docs.

---

## Out of scope

- No change to any core: snapshot/CAS atomic commit (§13), rescue (§18.3), per-repo lock (§18.5), decompose pipeline (§13.6), config precedence (§9.8/§16.1), per-role resolution (§9.15/§16.4), or the multi-turn/work-description modes (§9.24/§9.26).
- No new CLI flags, env vars, git-config keys, or config-file schema changes.
- No change to `qwen-code`'s manifest values (still `# TO CONFIRM` per FR-D5); only its §12.5.2 framing (agy-divergence note) is clarified.
- Adding a `This revision (v2.7)` header block to `PRD.md` is **not** required (the current PRD did not add one), but is permitted if the author wants the agy re-verification + gemini removal formally attributed in the revision history; it is a one-line editorial addition, not a task here.
