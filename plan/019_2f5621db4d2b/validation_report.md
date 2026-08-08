# Stagecoach — Validation Report

**Validator:** automated `validate.sh` + manual workflow simulation
**Date:** 2026-08-08
**Build:** `stagecoach version dev (79f59d0-dirty)`
**Repo state:** working tree had only this validator's own outputs staged (`validate.sh`, `validation_report.md`, `plan/019…/prd_index.txt`); no source changes.

---

## TL;DR

The application is **functionally healthy** — all 34 end-to-end user-workflow simulations pass (single commit, multi-commit decompose with the file-disjoint fast-path, one-file shortcut, arbiter reconciliation, FR-M1b concurrent-edit exclusion, FR-R5b hard error, duplicate rejection, exclusions, format modes, `--edit`/`--push`/`--dry-run`/`--reasoning`, config upgrade v2→v3, etc.), coverage clears the 85% gate on all four core packages, the e2e suite is green, and there are no panics/TODOs in shipped code.

**Three issues were found, all in the provider-manifest / documentation layer (none in the commit-generation core):**

| # | Severity | Area | One-liner |
|---|----------|------|-----------|
| 1 | **HIGH** (breaks CI) | `providers/cursor.toml` + test | Cursor reference manifest is missing `tooled_flags` the compiled-in code has → `TestProviderReferenceFiles_DecodeParity/cursor` fails → `go test ./...` and `make coverage-gate` fail. |
| 2 | **MEDIUM** | docs vs code comment | Cursor "end-to-end verified" status contradicts itself across `builtin.go`, `docs/providers.md`, and `README.md`. |
| 3 | **LOW** | formatting | Two files are not `gofmt`-clean; CI's linter config doesn't enable `gofmt`, so it slips through. |

Details below. None of these affect runtime commit correctness (verified by the 34 passing workflows + the in-process decompose/e2e suites).

---

## Bug #1 — Cursor reference manifest drift (HIGH, breaks CI)

### What
`internal/provider/builtin.go` → `builtinCursor()` ships `TooledFlags: ["--trust", "--yolo"]` (added in commit `f88042c feat(provider): add cursor tooled flags`). The shipped reference TOML, `providers/cursor.toml`, does **not** document a `tooled_flags` field. The two are required to be byte-for-byte equal (modulo comments), and the guard test enforces it.

### Evidence
```
$ go test -run TestProviderReferenceFiles_DecodeParity ./internal/provider/
--- FAIL: TestProviderReferenceFiles_DecodeParity (0.00s)
    --- FAIL: TestProviderReferenceFiles_DecodeParity/cursor (0.00s)
        referencefiles_test.go:60: decoded cursor does not match builtin
              built-in: {... BareFlags:[--mode ask --trust] TooledFlags:[--trust --yolo] ...}
              decoded:  {... BareFlags:[--mode ask --trust] TooledFlags:[] ...}
FAIL
```
The runtime is correct — `stagecoach providers show cursor` exposes `tooled_flags = ['--trust', '--yolo']` from the compiled-in manifest. Only the human-readable reference **file** is stale.

### Blast radius
- `go test ./...` → **FAIL** (the `internal/provider` package exits non-zero).
- `make coverage-gate` → **FAIL** — *not* because coverage is low (provider is 91.6%, well above the 85% floor) but because the failing test exits the package non-zero before the gate's awk reads its numbers. CI's `coverage` job (`ci.yml`) runs the same `go test -coverprofile ./...` + awk gate and would fail identically.
- This is the **only** failing test across 1,564 test functions; the other six providers' reference files are in sync.

### Root cause
Commit `f88042c` added cursor's tooled flags to the **code** but did not update `providers/cursor.toml`. (The analogous commits for agy/codex/opencode — `c34f480`, `f40ac87`, `c9e5fb3` — and the follow-on docs sync `ba03c5b docs: sync stager-capability table with code` — updated `docs/providers.md` but the cursor reference file itself was missed; its last touch predates `f88042c`.)

### Fix (not applied — validator only)
Add the tooled/stager block to `providers/cursor.toml` matching `builtinCursor()`, mirroring the structure used by `providers/ag.toml` / `providers/codex.toml` / `providers/opencode.toml`:
```toml
# --- tooled mode (stager; §11.5) ---
tooled_flags = [
  "--trust",   # skip the workspace-trust prompt (else -p would block)
  "--yolo",    # auto-approve tool calls non-interactively (alias for --force)
]
```
Then `gofmt`/re-test to confirm `TestProviderReferenceFiles_DecodeParity` passes.

---

## Bug #2 — Cursor "end-to-end verified" status contradicts itself (MEDIUM)

### What
Three sources disagree on whether cursor's manifest has been driven through a real end-to-end run:

| Source | Claim |
|--------|-------|
| `internal/provider/builtin.go` (comment above `builtinCursor()`'s `TooledFlags`) | "TOOLED (stager) — **VERIFIED 2026-07-09 (cursor-agent) end-to-end**: stages exactly the requested paths." |
| `docs/providers.md` (line ~112) | Lists cursor among "the other five stager-capable providers" (pi, agy, codex, opencode, cursor) as verified unscoped stagers. |
| `README.md` (line ~425, "End-to-end verification status") | "**cursor is NOT yet verified end-to-end** — its manifest is assembled from `agent --help` and ships untested here. … cursor is the one provider the maintainer can't validate without an account." |
| commit `f88042c` message | "Wire TooledFlags into the cursor manifest …" — **no** verification claim. |

### Why it matters
- If the code comment + `docs/providers.md` are right (cursor WAS verified end-to-end), the README undersells cursor and misleads potential contributors into re-doing verification that's done.
- If the README is right (cursor NOT verified — needs a Cursor subscription the maintainer lacks), then the `builtin.go` comment's dated "VERIFIED 2026-07-09 end-to-end" claim is an **over-claim** copied from the agy/codex pattern, and `docs/providers.md` propagates it. Users would be led to trust an untested stager path.

Either way it is a genuine internal contradiction that should be reconciled. The commit message (which describes only "wiring", not verification) leans toward the README being the truthful source and the code comment over-claiming — but this needs the maintainer's confirmation (do they have a Cursor account?).

### Fix (not applied — validator only)
Reconcile to one truth. If cursor was genuinely verified end-to-end on 2026-07-09, update `README.md`'s verification-status paragraph to list cursor alongside pi/agy/codex/opencode/claude and drop the plea for a Cursor subscriber. If it was not, correct the `builtin.go` comment to drop the "VERIFIED … end-to-end" claim and mark cursor's stager flags as `# TO CONFIRM` (parity with the spec's `§12.7` "TO CONFIRM" convention) and add a note to `docs/providers.md`.

---

## Bug #3 — Two files are not `gofmt`-clean (LOW)

### What
`gofmt -l` flags two files; the diff is purely structural (struct-field / comment alignment):

```
$ gofmt -l internal/ cmd/stagecoach/ pkg/
internal/provider/builtin.go
internal/provider/registry_test.go
```

- `internal/provider/builtin.go` — in the **agy** manifest (~line 246) the `Output` / `StripCodeFence` / `Experimental` columns are one space short of alignment with their neighbors; in the **opencode** manifest (~line 353) `Output` / `StripCodeFence` are one space *over*. (Both likely artifacts of the stager-flag additions.)
- `internal/provider/registry_test.go` — trailing-comment spacing in the table-driven stager-selection cases (~line 364, 368).

### Why CI doesn't catch it
`.golangci.yml` enables only: `errcheck, gosimple, govet, ineffassign, staticcheck, unused`. It does **not** enable `gofmt` or `gofumpt`. `go vet` doesn't check formatting either. So these drifts pass the `lint` job silently. (Cosmetic only — `gofmt` differences never change behavior — but it's the Go standard and trivial to fix.)

### Fix (not applied — validator only)
```
gofmt -w internal/provider/builtin.go internal/provider/registry_test.go
```
Optionally, add `gofmt`/`gofumpt` to `.golangci.yml`'s enable list (or a separate `fmt` CI step) so future drift is caught.

---

## What passed (confidence surface)

The validator exercised the full PRD surface against a built binary driving a deterministic stub agent (no network, no real agent, no account). **34/34 workflow simulations passed**, including the headline safety/correctness guarantees:

| Workflow | Result |
|----------|--------|
| Single staged commit (`write-tree` → generate → `commit-tree` → `update-ref` CAS) | ✅ |
| **Multi-commit decompose, file-disjoint fast-path (FR-M13/M14)** — 3 disjoint files → 3 commits, working tree clean | ✅ |
| **FR-M2b one-file shortcut** — planner canary NOT fired (planner bypassed) | ✅ |
| `--single` escape hatch (2 dirty files → 1 commit) | ✅ |
| `--commits 3` forced count (fast-path) | ✅ |
| **Arbiter leftover reconciliation** — unclaimed path → `target:null` → new commit, clean tree | ✅ |
| **FR-M1b start-of-run freeze** — file written mid-decompose lands in NO commit, stays in working tree | ✅ |
| **FR-R5b** — bare model `glm-5.2` on pi → hard error (not silent empty output) | ✅ |
| Duplicate rejection (FR30-33) — exact-subject match retried to a unique subject | ✅ |
| `--exclude` / `.stagecoachignore` payload-only (FR-X) — file committed, `[excluded]` placeholder, no body | ✅ |
| Binary file filtering (FR3a) — `[binary]` placeholder, no raw bytes | ✅ |
| `--template '$msg (#205)'` + `--locale` (FR-F6/F8) | ✅ |
| `format=gitmoji` (emoji table embedded, no network) / `format=conventional` (replaces style examples) | ✅ |
| `--context` injected, labeled authoritative (FR-F7) | ✅ |
| `--push` no-upstream → exit 1, commits stand (FR-P1/P2) | ✅ |
| `--edit` — `$EDITOR` appends body, committed verbatim (FR-E1/E3) | ✅ |
| `--dry-run` — no commit | ✅ |
| nothing-to-commit → exit 2; `--no-auto-stage` dirty → exit 2 | ✅ |
| `config upgrade` v2→v3 — `default_provider` folded into model prefix, dead key removed (FR-B7) | ✅ |
| `--reasoning high` on stub = graceful no-op (FR-R6) | ✅ |
| `--verbose` surfaces payload size + token estimate (FR50) | ✅ |
| `providers list/show`, `config init/path`, `hook install/status/uninstall`, `lock status`, `upgrade --check`, `integrate list` | ✅ |

**Other phases:**
- `go build ./...` — ✅ clean
- `go vet ./...` — ✅ clean
- Coverage gate (PRD §20.3 ≥85%): git **85.9%**, provider **91.6%**, generate **89.4%**, config **87.1%** — ✅ all pass (Phase 5 reports PASS; Phase 4's coverage-gate-via-Make fails only because the Bug-#1 test exits the provider package non-zero).
- E2E suite (`go test -tags e2e`, stub mode) — ✅ green
- Packaging: npm wrapper syntax, asdf/mise `shellcheck`, `flake.nix`+`flake.lock`, `.goreleaser.yaml` — ✅ all present/clean
- Shipped code: **0** `panic(`/`TODO`/`FIXME` occurrences in `internal/` + `cmd/stagecoach/`

---

## Residual risks / notes (not bugs)

- **`upgrade --check` on a dev build** returns exit 0 with "development build — cannot compare" (the `dev` version isn't a comparable semver). This is the documented, intended behavior for unreleased builds; the exit-6 (FR-U6) path only fires for a released binary behind a newer tag.
- **`stagecoach` provider detection** reports `pi/opencode/cursor/agy/codex/claude` as "detected" on this machine because those CLIs happen to be installed in this environment — environment-specific, not a stagecoach behavior.
- The **cursor** provider itself could not be driven through a real end-to-end run by this validator (no Cursor subscription) — the stub-based workflow coverage for cursor's *manifest shape* is complete, but the real-agent verification question is exactly the contradiction flagged in Bug #2.

---

## How to reproduce

```bash
./validate.sh                 # all 8 phases
./validate.sh workflows       # just the binary user-workflow simulations
./validate.sh unit coverage   # just those phases
```
The script is self-contained (builds stagecoach + stubagent into a temp dir, uses isolated throwaway git repos + an isolated `$HOME` so it never touches the user's real config), and exits non-zero iff any phase fails. The two red phases (fmt, unit) correspond to Bug #3 and Bug #1 respectively; Bug #2 is a doc contradiction noted here, not an automated check.