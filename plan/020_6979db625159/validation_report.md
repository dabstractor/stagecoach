# Stagecoach — Validation Report

**Date:** 2026-08-08
**Validator:** automated + manual deep analysis
**Baseline commit:** `c894c2b` (docs(install): replace last Winget refs with Chocolatey)
**Scope:** full codebase validation against PRD `spec/SPEC.md`, with emphasis on the recently-completed Phase 1 (Winget → Chocolatey + PowerShell installer) and Phase 2 (zai/glm-5.2 → anthropic/claude-haiku model-example cleanup) tasks.

---

## Executive summary

**The codebase is in excellent shape.** All automated validation phases pass — build, vet, lint, vulncheck, the full race-enabled test suite (20 packages), the e2e throwaway-repo harness, the 85 % coverage gate on the four core packages, cross-compilation of all six goreleaser targets, the npm/asdf/goreleaser distribution smoke, and a PowerShell AST parse of `install.ps1`. A live end-to-end workflow simulation driving the compiled binary with a stub provider exercises ten real user journeys from the README (single commit, `--dry-run`, **stage-while-generating safety**, all canonical exit codes, hook lifecycle, `--no-verify`, config bootstrap, the upgrade delegation table) — every one behaves exactly as the PRD specifies.

The headline product property — **stage-while-generating** (the user can stage the next batch during generation without the in-flight commit sweeping it in) — was verified live: a file staged *during* a 1.2 s simulated generation window is correctly excluded from the resulting commit and left staged for the next run. The atomic-rescue path leaves HEAD byte-for-byte unchanged.

Both recently-completed task phases are clean and internally consistent:
- **Phase 1 (Chocolatey + PowerShell installer):** zero Winget references remain in code, CI config, or user-facing docs (the only Winget mentions are the intentional historical design-decision records in the read-only spec, which the validator must not edit). `ChannelChocolatey` is correctly detected and PRINT-delegated; the goreleaser `chocolateys:` pipe validates; `install.ps1` parses clean and its `-DryRun` resolves the latest release (`v0.1.10`) with correct asset URLs over the real GitHub API.
- **Phase 2 (model cleanup):** zero stale `zai/glm-5.2` / `glm-5-turbo` tokens outside the spec/plan; `anthropic/claude-haiku` is the consistent example throughout. The remaining `z.ai` mentions are legitimate (z.ai described as an OpenAI-compatible proxy endpoint, not the deprecated default model).

Three minor findings are documented below — all low severity, none affecting correctness or the commit-generation core.

---

## Validation performed

### Automated phases (all PASS — see `./validate.sh`)

| # | Phase | Result | Notes |
|---|---|---|---|
| 1 | `go build ./...` + cross-build (6 targets) | ✅ PASS | linux/darwin/windows × amd64/arm64 all compile (CGO_ENABLED=0) |
| 2 | `go vet ./...` | ✅ PASS | clean |
| 3 | `golangci-lint run` | ✅ PASS | clean (local v1.64.8; CI pins v1.61, same v1 schema) |
| 4 | `govulncheck ./...` | ✅ PASS | no vulnerabilities |
| 5 | `go test -race ./...` | ✅ PASS | all 20 packages, ~75 s |
| 6 | Coverage gate (≥85 % on 4 core pkgs) | ✅ PASS | git 85.9 %, provider 90.0 %, generate 89.3 %, config 87.2 % |
| 7 | E2E scenarios (`-tags=e2e`, stub mode) | ✅ PASS | full PRD §20.5 regression set |
| 8 | Distribution smoke (npm + asdf + goreleaser) | ✅ PASS | shim smoke, installer smoke, shellcheck, `goreleaser check` |
| 9 | `install.ps1` AST parse + `-DryRun` | ✅ PASS | 0 parse errors; resolves v0.1.10 + correct asset URLs |
| 10 | Live workflow simulation (10 journeys) | ✅ PASS | see "Live workflow verification" below |
| 11 | Coherence (no Winget / no stale glm) | ✅ PASS | both task phases complete |

### Live workflow verification (Phase 10 detail)

Driven the compiled `stagecoach` binary against a stub provider in throwaway git repos — mirroring the actual README user journeys:

1. **Single staged commit** → correct tree, correct message, HEAD advances. ✅
2. **`--dry-run`** → message printed, HEAD unchanged, file stays staged. ✅
3. **Stage-while-generating** → a file staged *during* a 1.2 s generation window is **excluded** from the commit (tree contains only the pre-generation file) and **remains staged** afterward. ✅ (the headline invariant, verified live)
4. **Exit 2 — nothing to commit** (clean tree) → correct. ✅
5. **Exit 2 — nothing staged + `--no-auto-stage`** (dirty tree) → correct. ✅
6. **Exit 3 — rescue** (empty/unparseable output) → rescue recipe printed, HEAD **unchanged**. ✅
7. **Exit 1 — unknown provider** → fails fast pre-snapshot. ✅
8. **Exit 1 — FR-R5b multi-backend bare model** (`--provider testmulti --model bare`) → hard error "must be inference/model, e.g. anthropic/claude-haiku". ✅
9. **Commit hooks** → `pre-commit` runs by default on the plumbing path (FR-V1); `--no-verify` skips it (FR-V5). ✅
10. **`config init` bootstrap** → writes a populated working config (`config_version = 3`, `provider = "pi"`, per-role model tables). ✅
11. **`upgrade --install-method chocolatey`** → PRINT channel: prints `choco upgrade stagecoach -y` + admin note, exits 0 (FR-U1/U4). ✅
12. **`upgrade --install-method bogus`** → hard error exit 1 (FR-U2). ✅
13. **Hook lifecycle** (`install`/`status`/`exec`/`uninstall`) → fills message at top, preserves git comment block, no-ops on `source=message`. ✅

---

## Findings

Severity legend: 🟢 info / none · 🟡 low (cosmetic or doc-only) · 🟠 medium (functional edge case) · 🔴 high (correctness/safety).

### 🟡 [LOW] Stale documentation: `docs/packaging.md:5` contradicts `release.yml`

**Location:** `docs/packaging.md` line 5.
**Issue:** The maintainer-facing doc states:

> Homebrew/Scoop/AUR are pending their target repos (release.yml runs goreleaser with `--skip=homebrew,scoop,aur` until those exist).

This is **factually wrong** as of the current `release.yml`:
- `.github/workflows/release.yml:56` runs `args: release --clean` — **no `--skip` flags**.
- The adjacent comment (release.yml:53–55) explicitly states: *"Homebrew + Scoop + AUR pipes are all ENABLED … No `--skip` flags remain."*

**Impact:** Maintainer-facing only; no effect on the shipped binary, the commit core, or the Chocolatey migration (Phase 1). A maintainer reading `packaging.md` would believe Homebrew/Scoop/AUR are still disabled when they are in fact live.
**Recommendation:** Update `docs/packaging.md` line 5 to reflect that Homebrew/Scoop/AUR/Chocolatey pipes are all enabled in `release.yml` (the `--skip` guidance is obsolete). One-line fix; out of scope for the validation agent to apply.

### 🟡 [LOW] Stale TODO comment: `README.md:8`

**Location:** `README.md` line 8.
**Issue:** `<!-- TODO: add LICENSE file and badge -->` remains, but `LICENSE` exists (added 2026-08-06, 1073 bytes).
**Impact:** None functionally — it is an HTML comment, invisible in rendered Markdown. Purely a stale leftover.
**Recommendation:** Remove the comment line (or replace with a license badge if desired).

### 🟠 [MEDIUM] `--install-method` not validated on the `upgrade --check` path

**Location:** `internal/cmd/upgrade.go` (`dispatchUpgrade`) + `internal/cmd/upgrade_run.go` (`runCheck`).
**Issue:** FR-U2 specifies that an explicit invalid `--install-method` override is a hard error (`ErrUnknownChannel`) — "a typo should surface, not silently default to direct." This is correctly enforced on the normal upgrade path:

```
$ stagecoach upgrade --install-method bogus --yes   →  exit 1  ("unknown --install-method …")  ✅
```

But the `--check` path (`runCheck`) is dispatched **before** `upgradeDetect` runs, so it never validates the flag:

```
$ stagecoach upgrade --check --install-method bogus  →  exit 0  (flag silently ignored)  ⚠
```

A user who typos `--install-method` and runs `--check` (the CI/cron probe) gets no feedback that their flag value is invalid; they only discover it when they drop `--check`.

**Impact:** Low. `--check` is documented as side-effect-free (FR-U6: "delegating nothing, downloading nothing, swapping nothing") and arguably doesn't need the install method. No incorrect behavior results — the flag is simply not validated on that one path. The actual upgrade path validates correctly.
**Recommendation:** Either (a) validate the `--install-method` flag value early in `dispatchUpgrade` (before the `--check`/normal branch), or (b) document that `--check` ignores `--install-method` (it doesn't need it). Trivial code change if pursued.

---

## Phase 1 (Chocolatey + PowerShell installer) — verification detail

All Winget → Chocolatey migration work is complete and coherent:

| Check | Result |
|---|---|
| `rg -ni winget` in code/docs/README (excl. read-only spec/plan) | **0 hits** ✅ |
| `ChannelChocolatey` const + detection probe (`choco list --local-only stagecoach`) | ✅ correct |
| Chocolatey path heuristic (`ProgramData/chocolatey/`, separator-agnostic) | ✅ correct |
| Chocolatey is a PRINT channel (`choco upgrade stagecoach -y`, admin note, never self-swap) | ✅ correct (FR-U1/U4) |
| `goreleaser check` validates the `chocolateys:` pipe | ✅ PASS |
| `release.yml` wires `CHOCOLATEY_API_KEY` to the goreleaser job env; no `winget` job, no `WINGET_TOKEN` | ✅ |
| `install.ps1` AST parse (pwsh) | ✅ 0 errors |
| `install.ps1 -DryRun` against live GitHub API | ✅ resolves `v0.1.10` + correct `stagecoach_0.1.10_windows_amd64.zip` URL |
| `docs/packaging.md` Chocolatey section + PowerShell installer subsection | ✅ accurate |
| `docs/cli.md` upgrade channel list | ✅ lists Chocolatey (not Winget) |
| README install paths | ✅ Chocolatey + PowerShell installer |

The only Winget mentions in the entire tree are in `spec/` — and those are the **intentional historical design-decision records** (the v3.0 row that introduced Winget, and the v3.3 row that documents replacing it with Chocolatey). Per `AGENTS.md` hard rule #1, the spec is read-only and these records are the correct, authoritative history. They are **not** stale references.

## Phase 2 (model-example cleanup) — verification detail

All `zai/glm-5.2` → `anthropic/claude-haiku` cleanup is complete:

| Check | Result |
|---|---|
| `rg "zai/glm\|glm-5\.2\|glm-5-turbo"` in code/docs/README (excl. spec/plan) | **0 hits** ✅ |
| `providers/pi.toml` example comments use `anthropic/claude-haiku` | ✅ |
| `internal/provider/builtin.go` pi `DefaultModel` = `""` (FR-D2 decoupled) | ✅ |
| Runtime error messages (manifest.go, render.go, roles.go) cite `anthropic/claude-haiku` | ✅ |
| `internal/config/bootstrap.go` template comments use `anthropic/claude-haiku` | ✅ |
| Test fixtures (provider/config/decompose/generate/ui/cmd) | ✅ all consistent |

Remaining `z.ai` references (README:245, providers/codex.toml:83, builtin.go:382) are **legitimate** — they describe z.ai as an OpenAI-compatible proxy/endpoint that agents can be pointed at, *not* the deprecated default model. These are correct and in scope.

---

## Things explicitly verified NOT broken

- **Snapshot atomicity / CAS** — rescue path leaves HEAD unchanged; CAS abort path tested in the e2e suite (`S7_CASAbort_HeadMoved`).
- **Concurrent-file freeze (FR-M1b/M1c/M1d)** — e2e `S3_ConcurrentFile_Excluded` confirms a file written mid-run is excluded from every commit and left in the working tree, including across the arbiter gate.
- **Commit-identity transparency (FR-39a)** — no stagecoach-branded author/committer; commits are authored as the resolved git identity.
- **No shell injection** — all commands built as `[]string` via `exec.Command`, diff via stdin (§19).
- **No panics / no TODOs** in shipped (non-test) code.
- **No committed build artifacts** — `coverage.out`, `bin/`, `dist/`, `stubagent`, `*.exe` all gitignored; working tree clean.
- **`config upgrade` on a current file is a correct no-op**; on an inert file would not false-alarm (FR-B9).
- **Hook never-block contract** — `hook exec` exits 0 on generation failure (FR-H5).

---

## Conclusion

**Confidence: high.** The codebase builds, lints, and tests clean across all supported platforms; meets the coverage gate; has no known vulnerabilities; and the live workflow simulation confirms the product's core invariants — snapshot atomicity, stage-while-generating safety, correct exit-code semantics, hook integration, and the delegate-first upgrade architecture — all behave per spec. The recently completed Chocolatey migration and model-token cleanup are fully realized and internally consistent across code, CI, and docs.

The three findings are all low-to-medium severity documentation/UX polish items (two stale doc strings, one inconsistent flag-validation edge case on a side-effect-free probe path). None affect correctness, the commit-generation core, repository safety, or the distribution/release pipeline. They can be addressed in a follow-up without urgency.

**Recommended next steps (for the maintainer, not the validator):**
1. Update `docs/packaging.md:5` to reflect that Homebrew/Scoop/AUR/Chocolatey pipes are enabled (the `--skip` guidance is stale).
2. Remove the stale `<!-- TODO: add LICENSE -->` comment in `README.md:8`.
3. Decide whether `upgrade --check` should validate `--install-method` (one-line fix) or whether ignoring it is acceptable (document it).