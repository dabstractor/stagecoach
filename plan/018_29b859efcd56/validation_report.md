# Stagecoach — Validation Report

**Generated:** 2026-08-06
**Binary:** `stagecoach version dev (2670774-dirty)`
**Validator scope:** deep codebase analysis + automated `validate.sh` (4 phases) + manual E2E workflow simulation.

---

## TL;DR

Stagecoach is a **mature, exceptionally well-engineered** Go codebase (124 production files, 145 test files, ~90% coverage on the 4 core packages). The snapshot-based atomic-commit core, multi-commit decomposition, provider-manifest system, config layering, hook/integrate/upgrade subcommands, and the FR-R5b / FR-B7 / FR-39a invariants all behave exactly as the PRD specifies — every user-facing workflow I drove through the real binary worked correctly.

**One real bug was found, and it breaks CI:**

| # | Severity | Finding |
|---|----------|---------|
| 1 | 🔴 **High (CI-red)** | A **data race** in two tests under `-race` trips the race detector. CI runs `go test -race ./...` (`.github/workflows/ci.yml:76`), so **CI is currently red**. Root cause is test-harness-side (the tests mutate a shared manifest's `Env` map after handing it to a goroutine), not a production concurrency bug — but it gates the whole pipeline. |

No data-loss, correctness, or security defects were found in the commit/decompose/config/hooks paths. The full results below.

---

## Validation methodology

`validate.sh` runs four phases (full source in `./validate.sh`):

1. **Static analysis** — `go vet`, `gofmt -l`, `go build`, `--version`.
2. **Unit & integration tests** — `go test ./...` (fast) plus a targeted `go test -race ./internal/generate` (the race-enabled probe CI relies on).
3. **Coverage gate** — `make coverage-gate` (PRD §20.3: the 4 core packages must clear 85%).
4. **End-to-end user workflows** — the real binary driven through **stub providers** (no real agent quota spent, no network on the commit path), mirroring the journeys documented in `README.md` / `docs/`:
   - single-commit happy path & `--dry-run`
   - **stage-while-generating freeze** (§13.4 — the product's key IP; a sentinel staged *during* generation must not enter the commit)
   - **multi-commit decomposition** (§13.6 — dirty tree → N logical commits, per-concept isolation)
   - FR-R5b hard-error (bare model on `pi` rejected)
   - `config init`, `config upgrade` v2→v3 migration (FR-B7), idempotency, backup
   - `hook install/status/uninstall`, `integrate list`, `providers list/show`, `models`, `lock status`
   - `upgrade --check` (the single named network call)
   - `--edit` review gate (commit + empty-abort)
   - exit-code edge cases (nothing-staged → 2, clean-tree → 2)

**Result: 27 / 27 E2E checks PASS; 1 / 1 race check FAIL (the bug).**

---

## 🔴 Finding 1 — Data race in `internal/generate` test harness (CI-red)

**Symptom**

`go test -race ./internal/generate/` fails two tests:

```
--- FAIL: TestInvariants (1.29s)
--- FAIL: TestCommitStaged_GenerationFreeze_HoldsForLiveStagedSentinel (0.91s)
WARNING: DATA RACE
```

**Reproduce**

```bash
go test -race -count=1 ./internal/generate/
```

(The non-race `go test ./...` passes — the race detector is required to surface it.)

**Root cause**

Both tests follow the same pattern: they build a stub `provider.Manifest` `m`, spawn a goroutine running `generate.CommitStaged(ctx, Deps{Manifest: m}, cfg)`, and **then mutate `m.Env`** from the test goroutine while `CommitStaged` is concurrently reading it.

- `internal/generate/invariants_test.go:245–256` — spawns goroutine (line 245), then `m.Env["STAGECOACH_STUB_MARKER"] = marker` (line 256).
- `internal/generate/repro_freeze_test.go:79–91` — spawns goroutine (line 79), then `m.Env["STAGECOACH_STUB_MARKER"] = markerPath` (line 91).
- The concurrent **read** is `internal/provider/render.go:191` — `for k, v := range r.Env` (map iteration during `Manifest.Render`, called from `CommitStaged` at `generate.go:369`).

Go maps are not safe for concurrent read/write; the race detector flags the map-iter-start / map-assign collision.

**Impact**

- 🔴 **CI is red.** `.github/workflows/ci.yml` job `test-race` runs `go test -race ./...` on every push/PR (line 76). This test failure fails the matrix on `ubuntu-latest` and `windows-latest`, blocking merges and releases.
- 🔴 **`make test` is broken** locally for the same reason (`Makefile` `test` target uses `-race`).
- The **production code is correct** — `Manifest.Render` only *reads* `Env`; it never mutates the manifest. The race is purely in the test harness. The actual invariant under test (the generation freeze) **holds** — the tests' own assertions pass; only the race detector fails them.
- No runtime impact for end users (the manifest's `Env` is populated once at config-load and never mutated during a real run).

**Suggested fix direction (for the maintainer — not applied by this validator)**

Set `m.Env["STAGECOACH_STUB_MARKER"] = marker` *before* launching the `CommitStaged` goroutine (i.e. move the env assignment above the `go func()`), or construct the manifest with the marker env baked in via `stubtest.Options`. The marker is only needed so the test can poll for "generation in-flight"; assigning it up-front preserves that signal without the concurrent write.

---

## ✅ What passed (full confidence in these surfaces)

### Build & static analysis
- `go vet ./...` — clean.
- `gofmt -l .` — every Go file formatted.
- `go build ./cmd/stagecoach` — compiles to a single static binary.
- `--version` self-identifies via embedded VCS (`dev (2670774-dirty)`).

### Test suite & coverage
- `go test ./...` (no `-race) — **all 19 packages pass**.
- **Coverage gate (PRD §20.3) — PASS.** All four core packages clear the 85% floor:
  - `internal/git` — 86.7%
  - `internal/provider` — 91.6%
  - `internal/generate` — 89.4%
  - `internal/config` — 87.1%

### End-to-end user workflows (27/27 PASS, via stub providers)
| Workflow | PRD ref | Result |
|---|---|---|
| Single-commit happy path | §13.1–5 | ✅ commit created with the stub's message |
| `--dry-run` | FR49 | ✅ message printed, HEAD unchanged |
| **Stage-while-generating freeze** | §13.4, G4 | ✅ sentinel staged mid-generation is **excluded from the commit** and **remains staged** — the key IP holds |
| **Multi-commit decomposition** | §13.6, G11 | ✅ dirty tree → 2 logical commits, clean tree afterward, per-concept file isolation (x.go→c1, README.md→c2) |
| Decompose duplicate rejection | §9.7, FR32 | ✅ a second concept emitting a duplicate subject is rescued (correct behavior) |
| FR-R5b model-prefix hard error | §9.15, FR-R5b | ✅ bare `--model glm-5.2` on `pi` exits 1 with actionable message |
| `config init` populated bootstrap | §9.17, FR-B1 | ✅ writes a working config with detected provider + per-role models |
| `config upgrade` v2→v3 migration | §9.17, FR-B7 | ✅ folds `default_provider` into the model prefix (`zai/glm-5.2`), comments the removed key, bumps `config_version`, leaves a timestamped backup, idempotent on re-run |
| `hook install / status / uninstall` | §9.20, FR-H1/H3 | ✅ marker-tagged hook round-trips cleanly; status reports `stagecoach (v1)` |
| `integrate list` | §9.21, FR-I1 | ✅ lists `git-alias` + `lazygit` targets |
| `providers list / show` | §9.11, FR46/47 | ✅ lists built-ins; `show pi` confirms `provider_flag` (multi-backend) and blank `default_model` (FR-D2) |
| `models` | §9.23, FR-L1 | ✅ runs; never an HTTP call |
| `lock status` | §9.27, FR-K4 | ✅ read-only, reports "no run lock" with exit 0 |
| `upgrade --check` | §9.29, FR-U6 | ✅ reaches GitHub Releases, returns cleanly |
| `--edit` review gate | §9.22, FR-E1/E4 | ✅ editor runs, message committed; empty editor → exit 1 "empty commit message — aborted" |
| Nothing-staged → exit 2 | §15.4 | ✅ |
| Clean-tree → exit 2 | §15.4 | ✅ |

### Invariants verified by the passing unit suite (worth calling out)
- **Commit-identity transparency (FR-39a)** — `TestCommitIdentityTransparency` + `TestNoIdentityWritesInProduction` assert no `GIT_AUTHOR_*`/`GIT_COMMITTER_*` env and no `user.name`/`user.email` config writes in any production path. A stagecoach commit is indistinguishable from a hand-made one.
- **Decompose start-of-run freeze (FR-M1b/M1c/M1d)** — the sentinel-across-arbiter-gate tests pass (the v2.2 arbiter-freeze loophole is closed).
- **Closed-loop token budget (FR3j)** — `EstimateTokens(assembledFullPrompt) ≤ token_limit` enforced.
- **Mid-chain arbiter rebuild fidelity** — deterministic `OverlayTreePaths` reconstruction.

---

## Other observations (not bugs; context for the maintainer)

1. **`cursor` provider is unverified end-to-end** — the README and `providers/cursor.toml` both flag this honestly ("cursor is NOT yet verified end-to-end … ships untested here"). This is documented, not a defect. The other six providers (pi, agy, codex, opencode, claude) are verified.

2. **`qwen-code` provider is marked `experimental`** with several `# TO CONFIRM` fields — consistent with the PRD's progressive-verification ethos (§12.7.2). Not a bug; a tracked limitation.

3. **`upgrade --check` on a `dev` build** prints `stagecoach dev (latest: v0.1.0; development build — cannot compare)` and exits 0. Reasonable for a non-comparable build; FR-U6's exit-6 path only fires for tagged releases that are behind.

4. **The author's real CLIs (`pi`, `claude`, `codex`, `cursor`, `opencode`, `agy`) are on `$PATH`** on this machine, so `providers list` reports them as detected. `validate.sh` deliberately wires **stub** providers so it spends no real quota — but note that any test config that omits `[defaults] provider = "<stub>"` will auto-detect `pi` and make a real (billable) call. (The first draft of `validate.sh` had exactly this gap and was corrected.)

---

## Files written by this validation

- `validate.sh` — the executable validation script (4 phases; run with `./validate.sh`).
- `validation_report.md` — this report.

Both are temporary and safe to delete. No source files, `PRD.md`, `plan/`, `tasks.json`, or `.gitignore` were modified. The stagecoach git working tree is clean apart from these two new untracked files.