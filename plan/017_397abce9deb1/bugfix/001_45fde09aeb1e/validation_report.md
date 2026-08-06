# Stagecoach v3.0 Self-Update — Validation Report

**Scope:** `stagecoach upgrade` (FR-U1–U12) — the v3.0 delegate-first self-update feature.
**Validator approach:** end-to-end replication of every PRD bug report against the **REAL** source
(no reliance on the project's own test stubs), plus full codebase health checks (build / vet / lint /
unit / race) and live CLI exit-code verification against the real GitHub API.
**Validator:** `./validate.sh` (7 phases, 11 assertions). Run result on this codebase: **all green.**

---

## TL;DR

All three PRD bugs (**BUG-001** critical, **BUG-002** major, **BUG-003** minor) are **FIXED and
verified end-to-end**. The fixes are correct for the scenarios the PRD reproduces. The broader
codebase is healthy (all tests pass, `go vet` + `golangci-lint` clean, no data races, clean build,
exit-code contract `0/1/6` correct against live GitHub).

Validation also surfaced **three MINOR residual gaps** (routing-completeness, not breakages) in the
go-install detection and sanity gate — listed below. None of them block a real upgrade, and because
BUG-001 is fixed, none of them strand the user the way the original bugs did.

| ID | Severity | Status | One-line |
|---|---|---|---|
| BUG-001 | Critical | ✅ **Fixed & verified** | Sanity-run v/no-v version mismatch — fixed; real goreleaser binary now swaps |
| BUG-002 | Major | ✅ **Fixed & verified** (Unix) | go-install misdetection when GOPATH unset — fixed via ~/go fallback |
| BUG-003 | Minor | ✅ **Fixed & verified** | Prerelease tag selection over non-semver tags — fixed; order-independent |
| RES-1 | Minor (residual) | 🟡 New finding | Sanity substring check accepts a prerelease binary for a stable tag |
| RES-2 | Minor (residual) | 🟡 New finding | go-install detection gap on Windows (USERPROFILE\go, HOME-based fallback) |
| RES-3 | Minor (residual) | 🟡 New finding | `$GOBIN` not honored — go-install user with GOBIN set misroutes to direct |

---

## How each PRD bug was verified (not just unit-tested)

The PRD's central critique is that the *existing tests pass only because their stubs align with the
buggy code*. To rule that out, validation replicates each reproduction against the **actual
production logic**, isolated from the repo's test stubs:

* `internal/upgrade` is **stdlib-only with zero project imports** (verified: no
  `github.com/dabstractor/...` import in any non-test source file — FR-U12 holds). `validate.sh`
  **copies the real source** into a throwaway Go module and imports it, so the assertions exercise
  the genuine `sanityCheck` / `detectPath` / `LatestAdmittingPrereleases` code.
* BUG-001 is reproduced with the **real `cmd/stagecoach` binary built exactly as goreleaser does**
  (`go build -ldflags "-X main.version=1.2.0"`, no leading `v`), fed through a real
  `ResolveTarget` → `StageNewBinary` flow against an `httptest` GitHub fake whose release tag is
  `v1.2.0` — the precise PRD reproduction.

---

## BUG-001 — Critical — Sanity-run v/no-v mismatch — ✅ FIXED & VERIFIED

**Claim:** the sanity-run's `bytes.Contains(stdout, release.Tag)` always failed because
`release.Tag` carries the leading `v` (`v1.2.0`) while a goreleaser-built binary reports its version
**without** the `v` (`stagecoach version 1.2.0`, via `-X main.version={{.Version}}`), aborting
`stagecoach upgrade` for every real release on the only self-swap-eligible channel.

**Fix in place** (`internal/upgrade/stage.go`, `sanityCheck`): the check now accepts the tag **or**
its v-stripped form:
```go
if !bytes.Contains(out, []byte(wantTag)) && !bytes.Contains(out, []byte(strings.TrimPrefix(wantTag, "v"))) {
    return fmt.Errorf("sanity-run %s: output %q lacks tag %q: %w", ...)
}
```

**Verification (end-to-end, real binary):**
* Built `cmd/stagecoach` goreleaser-style → `--version` prints `stagecoach version 1.2.0` (no `v`).
* Ran it through `StageNewBinary` against a release tagged `v1.2.0` → **SUCCEEDED** (pre-fix this is
  the exact always-fail path). ✅
* **Negative control (safety gate intact):** a binary reporting `1.2.0` against tag `v9.9.9` is still
  correctly **REJECTED** with `ErrSanityVersionMismatch`. ✅ (The fix did not weaken the gate.)
* **v-prefixed binary control:** a binary reporting `v1.2.0` against tag `v1.2.0` also succeeds. ✅

**Regression test present:** `internal/upgrade/stage_test.go` `TestStageNewBinary_NoVGoreleaserOutput`.

---

## BUG-002 — Major — go-install misdetection when GOPATH unset — ✅ FIXED & VERIFIED (Unix)

**Claim:** a binary at `~/go/bin/stagecoach` was misrouted to `direct` when `GOPATH` was unset (the
common modern-Go case), because the heuristic only ran when `GOPATH` was explicitly set, with no
`~/go` fallback — contradicting FR-U2/FR-U3.

**Fix in place** (`internal/upgrade/detect.go`, `detectPath`): when `GOPATH` is empty, it resolves
the default to `$HOME/go` (matching `go env GOPATH`):
```go
gopath := d.envOr("GOPATH", "")
if gopath == "" {
    if home := d.envOr("HOME", ""); home != "" {
        gopath = filepath.Join(home, "go")
    }
}
```

**Verification (real source):**
* `ExePath=$HOME/go/bin/stagecoach`, `GOPATH` unset, `HOME` set → **`go-install`**
  (evidence `path: ~/go/bin (default GOPATH)`). ✅
* **Control:** `GOPATH` **and** `HOME` both unset → **`direct`** (no false-positive go-install). ✅

**Regression test present:** `internal/upgrade/detect_test.go` `TestDetect_Path_GoInstallDefaultGOPATH`.

---

## BUG-003 — Minor — Prerelease tag selection over non-semver tags — ✅ FIXED & VERIFIED

**Claim:** `LatestAdmittingPrereleases` could select a leading non-semver tag (`nightly`) over a
valid semver tag (`v1.5.0`) because `Compare` returns `0` for an unparseable operand.

**Fix in place** (`internal/upgrade/releases.go`, `LatestAdmittingPrereleases`): the selection loop
gates each candidate through `ParseAndClean` and advances `best` only when a parseable candidate
beats the current best — deprioritizing unparseable tags regardless of array order:
```go
_, okCandidate := ParseAndClean(r.TagName)
if best < 0 || (okCandidate && !okBest) || (okCandidate && okBest && Compare(r.TagName, rs[best].TagName) > 0) {
    best = i; okBest = okCandidate
}
```

**Verification (real source):**
* `[nightly, v1.5.0]` → `v1.5.0` (the original bug ordering). ✅
* `[v1.5.0, nightly]` → `v1.5.0` (order-independent). ✅
* `[v1.9.0, v2.0.0-rc1]` → `v2.0.0-rc1` (prerelease-aware precedence preserved — rc outranks older
  stable per semver §11.4). ✅

**Regression test present:** `internal/upgrade/releases_test.go` `TestClient_LatestAdmittingPrereleases`.

---

## Residual / New Findings (all MINOR)

These are completeness gaps in routing/detection, **not** functional breakages — and because
BUG-001 is fixed, none of them strand the user the way the original bugs did.

### RES-1 — Minor — Sanity substring check accepts a prerelease binary for a stable tag
**Location:** `internal/upgrade/stage.go` `sanityCheck` (substring, not semver-equality).
**Symptom:** a binary reporting `1.2.0-rc1` is **accepted** for a stable release tag `v1.2.0`,
because the v-stripped `1.2.0` is a substring of `1.2.0-rc1`. Verified end-to-end.
**Why it's minor:** it requires a release's archive to actually contain a mislabeled rc binary (a
packaging mistake or tampered-but-checksum-valid archive). The sanity gate is the last line of
defense, so this is a real but low-probability gap. It is **pre-existing** (the substring approach
predates the BUG-001 fix) and is **documented as intentional** in the source ("a plain substring
check — NOT a semver compare; that is the command layer's job"). A more robust gate would compare
`ParseAndClean(output)` == `ParseAndClean(tag)` via `Compare == 0`.
**Direction:** keep as-is, or harden to a semver-equality compare if the team wants the gate to
distinguish rc from stable.

### RES-2 — Minor — go-install detection gap on Windows (USERPROFILE\go)
**Location:** `internal/upgrade/detect.go` `detectPath` — the BUG-002 fallback reads `HOME`.
**Symptom:** on Windows, `go env GOPATH` defaults to `%USERPROFILE%\go`, but Windows typically does
**not** set `HOME` (it sets `USERPROFILE`). So a Windows user who `go install`ed stagecoach with
`GOPATH` unset is **not** detected as `go-install` → misroutes to `direct` (self-swap) instead of
`go install …@latest` delegation. Verified against the real source.
**Why it's minor:** contradicts FR-U3 routing, but the upgrade still **succeeds** (BUG-001 is fixed,
so the self-swap completes); the user just doesn't get delegation. The PRD's BUG-002 repro was
explicitly Unix, so this is a residual the fix didn't reach.
**Direction:** when `GOPATH` is empty, resolve the default from `USERPROFILE` on Windows
(`GOOS == "windows"`) in addition to `HOME`.

### RES-3 — Minor — `$GOBIN` not honored by go-install detection
**Location:** `internal/upgrade/detect.go` `detectPath` — references `GOPATH`/`HOME` only, never `GOBIN`.
**Symptom:** a user who sets `GOBIN=/custom/bin` and installs via `go install` has the binary at
`$GOBIN/stagecoach`, which the `$GOPATH/bin` / `~/go/bin` heuristic never matches → misroutes to
`direct`. Verified against the real source (`channel="direct"`).
**Why it's minor:** same class as BUG-002; same low impact (self-swap succeeds). `GOBIN` is a less
common setup than the default `~/go/bin`.
**Direction:** add a `GOBIN` path check alongside the `GOPATH/bin` heuristic.

---

## Broader Codebase Health (all green)

| Check | Command | Result |
|---|---|---|
| Build (all packages) | `go build ./...` | ✅ clean |
| Build main binary | `go build ./cmd/stagecoach` | ✅ clean |
| Static analysis | `go vet ./...` | ✅ clean |
| Lint | `golangci-lint v1.61 run` (CI pin) | ✅ clean on upgrade surface |
| Unit tests | `go test ./...` (21 packages) | ✅ all pass |
| Race detector | `go test -race ./internal/upgrade/...` | ✅ no data races |
| CLI exit codes (live) | `upgrade --check` behind / current / dev | ✅ `6` / `0` / `0` against real GitHub |
| Delegate targets (FR-U3) | brew/scoop/winget/mise/go-install/npm/asdf/aur/nix | ✅ all argv correct |
| Error propagation | failed delegate + failed resolve → exit | ✅ correctly `1` (not masked) |

**Exit-code contract (PRD §15.4: 0/1/6) verified live:** a behind build exits `6`, an up-to-date
build exits `0`, a dev build is informational (`0`), a release-resolution error exits `1`, and a
failing delegated updater (e.g. npm 404) exits `1`. (During validation, an initial false reading of
"exit 0 on failure" was traced to piping CLI output through `head` — the real exit codes are
correct. This is documented here so it isn't re-reported.)

---

## Validator Usage

```bash
./validate.sh            # run all 7 phases (Phase 7 needs network for live GitHub --check)
./validate.sh --quick    # skip the network-dependent Phase 7
```

Phases: (1) build, (2) `go vet`, (3) `golangci-lint`, (4) unit tests, (5) race detector on
`internal/upgrade`, (6) **E2E bug regression against the real source** (BUG-001/002/003 + the
goreleaser-style binary), (7) live `--check` exit-code contract. The script writes **nothing** to
the repo — all temporary artifacts go to a `mktemp` directory that is cleaned up on exit. Exit code
is `0` only if every assertion passes.

> **Note:** `validate.sh` and `validation_report.md` are temporary validation artifacts and may be
> deleted after validation completes.