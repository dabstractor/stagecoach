# Stagecoach — Validation Report

**Validated:** stagecoach @ `e730f7d` (main, "docs(arch): fix stale fast-path concurrency analysis")
**Validator:** automated `./validate.sh` (33 hard checks) + manual deep analysis
**Date:** 2026-08-08

---

## TL;DR

**The codebase is in excellent shape.** Every hard validation check passes — build, vet, the full
unit-test suite, the race detector on the core packages, the 85% coverage gate, the build-tagged e2e
suite, and 17 binary-level user-workflow tests driven against the compiled binary with a stub agent.
All headline invariants hold end-to-end through the real binary: the snapshot freeze (FR-M1b),
commit-identity transparency (FR-39a), the FR-R5b multi-backend prefix hard-error, duplicate rejection
(FR30-33), the rescue recipe (FR43-45), and the v3.2 file-disjoint decompose fast-path.

**No functional bugs were found.** The issues below are **documentation / code-style drift**, not
behavioral defects — none of them affect correctness, safety, or any user-facing workflow.

| Severity | Count | Category |
|----------|-------|----------|
| Critical | 0 | — |
| High     | 0 | — |
| Medium   | 1 | Documentation inconsistency (cursor verification status) |
| Low      | 3 | gofmt drift; agy.toml self-contradiction; cursor.toml stale marker |

---

## What was validated

### Static analysis & build
- `go build ./...` — clean.
- `go vet ./...` — clean.
- Cross-compile of all 6 goreleaser targets (`linux/arm64`, `darwin/{amd64,arm64}`, `windows/arm64`) — covered by CI; host native build verified.
- `gofmt -l` — **3 files flagged** (see issue #1).
- `golangci-lint` (CI's v1.61 config: errcheck/gosimple/govet/ineffassign/staticcheck/unused) — could not run locally (offline), but the configured linter set does **not** include `gofmt`, so the gofmt drift would not surface in CI lint. CI runs it on push.
- `govulncheck` — not installed locally; CI runs it.

### Tests
- `go test ./...` — **all 19 packages pass** (including `internal/cmd` ~30s, `internal/decompose` ~15s).
- `go test -race` on the 6 concurrency-sensitive packages (`git`, `provider`, `generate`, `config`, `decompose`, `lock`) — **clean, no data races**.
- Coverage gate (`make coverage-gate`, PRD §20.3 ≥85%): `git 85.9%`, `provider 90.0%`, `generate 89.3%`, `config 87.2%` — **all pass**.
- `go test -tags=e2e ./internal/e2e/...` — **passes** (the PRD §20.5 throwaway-repo harness: 7 scenarios S1–S7, stub-reachable by default).

### Binary-level user workflows (Phase 6 — driven against the compiled binary)
Each scenario seeds a fresh `git init` temp repo and asserts on history + exit code:

1. **Single-commit happy path** — staged files → atomic commit ✓
2. **FR-39a commit-identity transparency** — author/committer == user config; no `Co-Authored-By`/`Generated-by` trailer ✓
3. **`--dry-run`** — message printed, HEAD unchanged ✓
4. **Clean tree** → exit 2 ✓
5. **FR-R5b** bare model on pi (`--provider pi --model glm-5.2`) → hard error, exit 1 ✓
6. **v3.2 decompose fast-path** — 3 disjoint untracked files → 3 commits, clean tree, no stager agent ✓
7. **Fast-path deletion handling** — `git add` correctly stages deletions + mods + adds ✓
8. **FR-M1b freeze** — a sentinel file written *during* generation (via the stub's marker+sleep) is **excluded from the commit** and left untracked ✓ (verified on both single-commit and decompose paths manually)
9. **Duplicate rejection (FR30-33)** — duplicate subject rejected, retried to a unique one ✓
10. **`--template` (FR-F8)** — `$msg` substitution applied pre-commit ✓
11. **Rescue path (FR43-45)** — SIGINT mid-generation → tree SHA + `git commit-tree` recipe, exit 3 ✓
12. **`config init`** — populated bootstrap with `config_version = 3` ✓
13. **`config upgrade` (FR-B8)** — active settings preserved, version bumped, backup created ✓
14. **`config upgrade` (FR-B9)** — inert file is a no-op (no false "legacy" alarm) ✓
15. **`hook install`/`status`/`uninstall` (FR-H1/H3)** ✓
16. **FR-H2 foreign-hook refusal** — never clobbers an existing non-stagecoach hook ✓
17. **`integrate list`**, **`lock status`**, **`providers list/show`**, **`models`**, **`upgrade --check`/`--rollback`**, **missing-`--config` fails fast (exit 1)** ✓

### Manual deep-dive on the headline invariants
- **FR-M1b freeze boundary** was exercised manually with a real concurrent working-tree write (the stub writes a readiness marker after draining stdin, a background injector writes a sentinel on the marker, the run then commits): the sentinel lands in **no** commit and remains untracked — on **both** the single-commit and decompose fast-path paths. This is the product's headline safety property and it holds.
- **Commit-identity transparency** verified: a stagecoach commit is byte-for-byte indistinguishable in metadata from a hand-made commit (no stagecoach-branded author/email/trailer).

---

## Issues found

### Issue #1 — gofmt drift in 3 files (LOW)
**Severity:** Low (cosmetic; does not affect behavior and does not fail the configured CI lint).
**Evidence:** `gofmt -l .` reports:
```
internal/config/role_defaults.go
internal/provider/builtin.go
internal/provider/registry_test.go
```
**Detail:** All three are comment/struct-field *alignment* drift (tabs vs. spaces after re-aligned columns), e.g. in `role_defaults.go`:
```go
"planner": "gpt-5.4-nano",      // fast tier   ← one space short of gofmt's alignment
```
and in `builtin.go` the `agy`/`opencode` `Output`/`StripCodeFence` struct fields are mis-aligned by one tab.

**Why it isn't caught by CI lint:** `.golangci.yml` enables only `errcheck, gosimple, govet, ineffassign, staticcheck, unused` — **not** `gofmt`. So CI lint passes while `gofmt -l` flags drift. This is the kind of invisible-to-CI debt the validation script surfaces explicitly.

**Recommended action:** `gofmt -w internal/config/role_defaults.go internal/provider/builtin.go internal/provider/registry_test.go`. No behavior change.

---

### Issue #2 — cursor end-to-end verification status is documented 3 contradicting ways (MEDIUM)
**Severity:** Medium (a user/contributor reading the docs is misled about what is actually tested).

Three places make mutually-exclusive claims about whether the **cursor** provider has been verified end-to-end:

| Source | Claim |
|--------|-------|
| `README.md:434` (user-facing FAQ) | "**cursor is NOT yet verified end-to-end** — its manifest is assembled from `agent --help` and ships untested here… cursor is the one provider the maintainer can't validate without an account." |
| `internal/provider/builtin.go:439` (code) | "**CONFIRMED** (integration, 2026-07-09, cursor-agent): `--mode ask` DOES win over `-p`'s default full-tools profile" |
| `internal/provider/builtin.go:467` (code) | "TOOLED (stager) — **VERIFIED 2026-07-09** (cursor-agent) **end-to-end**: stages exactly the requested paths" |
| `providers/cursor.toml:88` (reference file) | "# **TO CONFIRM** (integration): that `--mode ask` wins over -p's default full-tools profile" |

The `cursor.toml` "TO CONFIRM" line is the *exact same claim* that `builtin.go` marks "CONFIRMED … 2026-07-09". So within the cursor provider's own artifacts there is a direct contradiction, and the README FAQ adds a third voice ("not verified / untested").

**Most likely truth:** cursor *was* verified end-to-end on 2026-07-09 (the code comments carry specific dates + the `git log` shows `5980a44 docs(provider/cursor): sync reference file + spec/doc with the cursor stager`), and the **`cursor.toml` reference file and the README FAQ are stale**. But it is genuinely ambiguous from the repo state alone, and a user reading the README would believe cursor is untested and (per the FAQ) be invited to help verify something the code already claims is done.

**Recommended action:** Decide which is authoritative and sync all three:
- If cursor is verified: update `README.md:434` (remove the "NOT verified" / call-for-help text) and `providers/cursor.toml:88` (replace "TO CONFIRM" with "CONFIRMED 2026-07-09").
- If cursor is *not* verified: correct the over-claims in `builtin.go:439,467`.

---

### Issue #3 — `providers/agy.toml` self-contradiction about verification item 4 (LOW)
**Severity:** Low (a comment-only inconsistency inside one reference file).

Within `providers/agy.toml` itself:
- Line 49: "pending a full `--help` re-verification pass (**item 4 itself is cleared**)."
- Line 118: `experimental = true # ships experimental (§12.5.1.1; **only item 4** — tooled/stager flags — **remains open**).`

These two comments in the same file disagree about whether PRD §12.5.1.1 item 4 (the agy tooled/stager combo) is cleared or still open. `builtin.go` agrees with line 49 ("VERIFIED 2026-07-09, agy v1.1.11"), so **line 118 is the stale one**.

**Recommended action:** Update `providers/agy.toml:118` to "all items cleared; ships experimental pending a full `--help` re-verification pass." (The `experimental = true` flag itself stays — it is consistent with `builtin.go`.)

---

### Issue #4 — `providers/cursor.toml` stale "TO CONFIRM" (LOW, subset of #2)
Captured under issue #2. The `--mode ask`-wins-over-`-p` claim is marked "TO CONFIRM" in the reference file but "CONFIRMED" in `builtin.go`.

---

## Things checked and confirmed NOT to be issues

- **PRD vs code on role tiering (FR-D3):** the input PRD table in the task prompt still shows the *old* FR-D3 ("planner = flagship/smart"), but the **repo's actual `spec/01-product.md` FR-D3 was updated** to "fast by default" — and the code (`internal/config/role_defaults.go`) matches the updated spec. Code and spec are in sync; the apparent divergence was a stale snapshot in the task prompt, not a repo defect.
- **Stager-capability table (P2 docs sync):** `docs/providers.md` provider table matches `internal/provider/builtin.go` exactly — 6 stager-capable (pi, claude, opencode, codex, cursor, agy), 1 not (qwen-code). The PRD §12.7 codex manifest lacks `tooled_flags` (spec stale), but per AGENTS.md hard rule #1 the spec is not edited outside an interactive session; this is a **known, documented** spec gap, not a code bug.
- **Exit codes:** verified exit 0 (success), 1 (error/missing-config), 2 (nothing-to-commit), 3 (rescue), and the documented "fail fast on missing `--config`" all behave correctly. (An earlier manual probe suggested a wrong exit code, but that was a `| head` pipe masking the real code — the binary is correct.)
- **Commit-identity transparency (FR-39a):** holds — no stagecoach branding in any commit's author/committer/message.
- **Concurrent working-tree exclusion (FR-M1b):** holds on both commit paths — the v3.2 fast-path respects the freeze identically to the tooled-stager path.

---

## How to reproduce

```bash
./validate.sh                # full (race + e2e + network probe; ~3-4 min)
./validate.sh --quick        # build/vet/fmt/unit/coverage only (~1 min)
./validate.sh --no-network   # skip the upgrade --check GitHub API probe
```

Exit code is 0 only if every hard check passes. Warnings (gofmt drift, offline lint/vulncheck) are informational and do not fail the run. Temp artifacts go to a self-contained `mktemp -d` and are cleaned up; the repo is never mutated.

---

## Conclusion

Stagecoach is **production-ready on this commit** as far as functional behavior is concerned. The
findings are confined to documentation consistency and formatting — worth cleaning up, but none of
them represent a correctness, safety, or workflow regression. The most important one to resolve is
**issue #2** (the cursor verification contradiction), because it is user-visible in the README FAQ
and could mislead both users and contributors about the tested state of a shipped provider.