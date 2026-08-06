# Stagecoach — Validation Report

**Validated:** 2026-08-06
**Scope:** Full codebase — build, static analysis, unit/integration/e2e tests, coverage gate, and 26 end-to-end user-workflow scenarios driven through a stub provider (no real AI agent required).
**Result:** ✅ **All hard checks PASS (30/30).** 2 real bugs and 2 cosmetic findings surfaced as warnings during deeper end-to-end exploration.

---

## How validation was performed

1. **Static analysis** — `go vet ./...`, `gofmt -l`, and `golangci-lint` (when present).
2. **Build** — `go build ./...` for all packages + the two scenario binaries (`stagecoach`, `stubagent`).
3. **Tests** — `go test ./...` (unit + integration), `go test -tags e2e ./internal/e2e/...` (the PRD §20.5 throwaway-repo harness), and `make coverage-gate` (PRD §20.3 ≥85% on the 4 core packages).
4. **End-to-end workflows** — 26 scenarios run the **compiled `stagecoach` binary as a subprocess** against fresh `git init` temp repos, using `cmd/stubagent` (a fake agent whose output is controlled by `STAGECOACH_STUB_*` env vars) wired via a `[provider.stub]` config. This is the same pattern as `internal/e2e`, so it catches CLI-routing, config-load, and real-git-plumbing bugs that in-process tests cannot reach. Run it yourself: `./validate.sh`.

### Baseline (everything green)

| Check | Result |
|---|---|
| `go build ./...` | ✅ clean |
| `go vet ./...` | ✅ clean |
| `go test ./...` (all packages) | ✅ pass |
| `go test -tags e2e ./internal/e2e/...` | ✅ pass |
| Coverage gate (git/provider/generate/config ≥85%) | ✅ pass (86.7 / 91.6 / 89.2 / 87.1 %) |
| npm wrapper smoke + install tests | ✅ pass |
| asdf/mise plugin smoke test | ✅ pass |

---

## Bugs found

### BUG-1 (Medium) — `--edit` strips the blank line between subject and body

**Location:** `internal/generate/finalize.go`, `stripCommentsAndTrim()`

**Symptom:** When a user runs `stagecoach --edit` and writes a commit message with a subject, a blank line, and a body, the blank line is **removed**. The resulting commit collapses subject and body into one paragraph — the body is folded into the subject line.

**Reproduction** (from `validate.sh` WF-14b):
```
editor writes:   "fix: real subject\n\nreal body line\n"   (subject + blank + body)
commit object:   "fix: real subject\nreal body line"        ← blank line GONE
git %s shows:    "fix: real subject real body line"         ← body swallowed into subject
```

**Root cause:** `stripCommentsAndTrim` drops *every* empty line, not just comment lines:
```go
if t := strings.TrimRight(line, " \t\r"); t != "" {
    kept = append(kept, t)   // ← any blank line is dropped entirely
}
```

**Spec violation:** FR-E1 (§9.22) specifies *"strip comment lines and trailing whitespace"* for the edited message. It does **not** authorize stripping blank lines. Git's own `commit.cleanup = strip` (the default) preserves a single blank line separating subject from body — it only strips leading/trailing blank lines and collapses *consecutive* blank lines. This implementation drops them all.

**Impact:** Any `--edit` user who writes a proper subject + body loses the body separation in the committed message. Data-integrity-of-message bug (the *commit* is fine; the *message structure* is corrupted). The non-`--edit` path is unaffected — multi-line generated messages preserve bodies correctly (verified).

**Severity rationale:** Medium. Only affects the `--edit` opt-in path, but within that path it corrupts message structure for every multi-line message — a user following the git convention of subject-blank-body gets a structurally wrong commit.

---

### BUG-2 (Low / Cosmetic) — Double `"stagecoach:"` prefix on every `upgrade` error

**Location:** `internal/cmd/upgrade.go` (3 sites) and `internal/cmd/upgrade_run.go` (6 sites) — all error returns of the form `fmt.Errorf("stagecoach: %w", err)`.

**Symptom:** Every error message from `stagecoach upgrade ...` is printed with the program name twice:
```
$ stagecoach upgrade --version v1.0.0 --prerelease
stagecoach: stagecoach: --version and --prerelease are mutually exclusive
```
The exit codes are correct (1); only the message text is cosmetically wrong.

**Root cause:** `cmd/stagecoach/main.go` already prepends `"stagecoach: "` when printing any returned error:
```go
fmt.Fprintf(os.Stderr, "stagecoach: %v\n", err)
```
Every **other** command in the codebase relies on this and therefore wraps errors *without* a prefix, e.g. `config.go`:
```go
return exitcode.New(exitcode.Error, fmt.Errorf("no config file at %s ...", path))   // → "stagecoach: no config file ..."
```
But the `upgrade` files add `"stagecoach: "` themselves (`fmt.Errorf("stagecoach: %w", err)`), producing the doubled prefix.

This directly contradicts the rule documented in `upgrade_run.go`'s own file comment:
> "confirmUpgrade's PLAIN error (no `stagecoach:` prefix) is wrapped via `exitcode.New(exitcode.Error, err)` WITHOUT re-adding the prefix (S2's rule; main.go adds it)."

The author followed that rule for `confirmUpgrade` but violated it for the 9 other error paths in the same files.

**Fix:** Drop the `stagecoach: ` prefix from the `fmt.Errorf("stagecoach: %w", err)` wrappers in `upgrade.go` / `upgrade_run.go` (pass `err` through unwrapped, or `fmt.Errorf("%w", err)`).

**Impact:** Cosmetic — affects only the displayed message for `upgrade` errors, not behavior or exit codes.

---

## Cosmetic observations (not bugs)

### OBS-1 — One gofmt-unformatted test file
`internal/config/inert_test.go` has struct-literal field alignment that `gofmt` would change. **Non-blocking**: the `.golangci.yml` config does not enable the `gofmt`/`gofumpt` linter (it enables only `errcheck, gosimple, govet, ineffassign, staticcheck, unused`), so CI lint passes. Still worth a `go fmt` pass for cleanliness.

### OBS-2 — `LAZYGIT_CONFIG_DIR` not honored by `integrate` when `lazygit` is on PATH
`internal/cmd/integrate_lazygit.go:resolveLazygitConfigPath()` prefers `lazygit --print-config-dir` (which is authoritative when `lazygit` is installed and itself honors the env var), then falls back to `XDG_CONFIG_HOME` / platform default. It never reads `LAZYGIT_CONFIG_DIR` directly. This is a pre-existing, documented design choice (the code comment explains it) and only matters in the narrow case where `lazygit` is **not** on PATH but the user sets `LAZYGIT_CONFIG_DIR`. Not a regression from the v3.0 work; noted for completeness.

---

## What was verified working (confidence areas)

The validation explicitly exercised every headline invariant and feature in the PRD:

**Core commit engine (§13)**
- Snapshot freeze / stage-while-generating — a file staged *during* generation is excluded from the in-flight commit and remains staged (WF-2). ✅
- Rescue path — a failed generation leaves HEAD and the index byte-for-byte unchanged, exit 3, recovery recipe printed (WF-6). ✅
- Atomic CAS `update-ref` semantics (HEAD-movement guard) — exercised by the in-process + e2e suites. ✅

**Provider system (§12)**
- FR-R5b multi-backend model enforcement — a bare model on a `provider_flag` provider (pi-style) is a hard error with a clear remediation; a prefixed model (`zai/glm-5.2`) is split into `--provider zai --model glm-5.2` (WF-7, WF-8). ✅
- `providers list` / `providers show` (WF-20). ✅

**Commit message quality (§9.3, §9.7)**
- Style learning + duplicate-rejection loop — a subject matching one of the last 50 is retried with an explicit rejection list until a new one is produced (WF-9). ✅

**Multi-commit decomposition (§13.6)**
- One-file short-circuit (FR-M2b) — a single changed file bypasses the planner entirely (verified via a planner canary; WF-21). ✅
- `--single` / `--no-decompose` escape hatch produces exactly one commit (WF-5). ✅
- Full multi-concept decompose + arbiter + `T_start` freeze invariants (FR-M1b/c/d) — covered by the `internal/decompose` in-process suite and the e2e S1/S3/S5 scenarios (all pass). ✅

**Commit-path hooks (§9.25, v2.4)**
- `pre-commit` runs by default around every plumbing-path commit (WF-10). ✅
- `--no-verify` skips `pre-commit`/`commit-msg` only (WF-11). ✅
- A failing `pre-commit` aborts the run before `update-ref`, leaving HEAD + index unchanged, exit 3 (WF-12). ✅

**`--edit` / `--push` conveniences (§9.22)**
- Empty edited message aborts with exit 1 ("empty commit message — aborted"), NOT a rescue (WF-13). ✅
- Edited message is committed (WF-14). ✅ *(subject/body blank-line bug aside — BUG-1)*

**Config model (§9.17, FR-B8)**
- `config init` writes a populated working config for a built-in provider (WF-16). ✅
- `config upgrade` preserves every active setting (round-trips 5/5 keys, bumps to schema v3, leaves a timestamped backup) — the never-clobber guarantee holds (WF-17). ✅
- v2→v3 in-memory migration + advisory notice fires correctly. ✅

**Self-update v3.0 (§9.29) — the newest feature**
- `stagecoach upgrade` is correctly **walled off** from the commit core: its no-op `PersistentPreRunE` overrides `config.Load`, it acquires no run lock, reads no repo, invokes no provider. Verified it runs outside a git repo. ✅
- `upgrade --rollback` with no backup is a no-op reported as such, exit 0 (FR-U8) (WF-18). ✅
- `upgrade --check` resolves current-vs-latest; a `dev` build prints the latest tag and exits 0 informatively without falsely claiming an update (FR-U6) (WF-25). ✅
- Flag validation mutex rules (`--version`/`--prerelease`, `--rollback`/`--check`) reject with exit 1 (WF-19). ✅ *(double-prefix bug aside — BUG-2)*
- Exit-code discipline: code 6 (`UpdateAvailable`) is upgrade-path-only and never returned by the commit path (asserted in `exitcode` tests). ✅
- The `[upgrade]` config table is additive — does not bump `CurrentConfigVersion`, emits no load-time advisory (FR-B4). ✅
- Semver `Compare` handles all standard cases (prerelease ordering, numeric not lexical, leading-`v`). ✅
- The direct-swap path's failure-safety (bad SHA256 / bad sanity-run aborts before any on-disk change) and install-method delegation (refuses a manager-owned binary unless `--force`) are covered by `internal/cmd/upgrade_*_test.go` + `internal/upgrade/*_test.go`. ✅

**Distribution surface (§21, v3.0)**
- npm wrapper: shim sets `STAGECOACH_INSTALL_METHOD=npm`, execs the cached binary, propagates exit code; `install.cjs` postinstall maps platform→asset, SHA256-verifies, extracts; smoke + install tests pass. ✅
- Nix flake (`flake.nix`) present with documented vendorHash workflow; not runnable locally (no `nix` installed) but structure is sound. ✅
- asdf/mise plugin: `bin/list-all` enumerates tags via `git ls-remote` (no API rate-limit); `bin/install` downloads + SHA256-verifies + extracts; smoke test passes against a fixture. ✅
- Homebrew/Scoop/AUR/go-install/curl|sh all documented and wired in `.goreleaser.yaml` / `release.yml`. ✅

**Docs & spec reconciliation**
- `FUTURE_SPEC.md` correctly marks self-update as **SUPERSEDED** by PRD §9.29 (v3.0) with the delegate-first rationale. ✅
- README install section lists all channels live; FAQ scopes §19's "no network calls" to the commit path with `upgrade` as the named exception. ✅
- `.goreleaser.yaml` annotated with the "Beyond goreleaser" channel pointers. ✅

---

## Risk assessment

The codebase is in strong shape. All shipped test layers pass, the coverage gate holds, the core snapshot/CAS/rescue invariants are verified end-to-end, and the v3.0 self-update feature is correctly walled off from the commit core.

The two bugs found are both **low blast radius**:
- **BUG-1** (`--edit` blank-line stripping) is a real message-structure corruption but confined to the opt-in `--edit` path; the default and decompose paths are unaffected.
- **BUG-2** (double prefix) is purely cosmetic and confined to the `upgrade` command's error messages.

Neither bug affects repository integrity, commit atomicity, or the core snapshot/CAS guarantees — the product's headline safety properties all hold.

---

## Recommended next steps (for the maintainer — not part of this validation)

1. **BUG-1:** Rework `stripCommentsAndTrim` to preserve a single inter-paragraph blank line (mirror git's `strip` cleanup: drop *leading/trailing* blank lines, collapse *consecutive* blanks to one, but keep the subject/body separator). Add a regression test that commits an edited subject+body and asserts `git cat-file -p HEAD` contains the blank line.
2. **BUG-2:** Remove the redundant `"stagecoach: "` prefix from the 9 `fmt.Errorf("stagecoach: %w", err)` sites in `internal/cmd/upgrade.go` and `internal/cmd/upgrade_run.go`, matching the convention in all other command files.
3. **OBS-1:** Run `go fmt ./internal/config/...` to normalize `inert_test.go` (optional — not gated by CI).