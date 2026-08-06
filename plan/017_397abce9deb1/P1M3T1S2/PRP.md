name: "P1.M3.T1.S2 — StageNewBinary: download+verify+extract+sanity-run (FR-U5 steps 4-6, FR-U11 abort-before-swap)"
description: >
  The download→verify→extract→sanity-run step of the direct-binary self-swap (FR-U5 steps 4-6 + FR-U11). A NEW file
  internal/upgrade/stage.go (NOT swap.go — S1's PRP reserved swap.go for P1.M3.T2.S1 [Backup + atomic swap]; StageNewBinary
  "stages" the new binary in temp, the on-disk "swap" is P1.M3.T2's distinct step; mirrors S1's own resolve.go-vs-swap.go
  collision-avoidance). Exports `StageNewBinary(ctx, c *Client, release Release, asset Asset, tempDir string) (newBinPath
  string, err error)` — the MISSING middle of the swap pipeline: S1's ResolveTarget returns (Release, Asset); StageNewBinary
  turns that into a verified, sanity-checked new-binary path in tempDir; P1.M3.T2 does the backup+atomic-rename. Three steps:
  (1) DOWNLOAD+VERIFY using the PRE-SELECTED asset: `sums := c.FetchChecksums(ctx, release)` → `want := sums[asset.Name]`
  (ErrChecksumMissing if absent) → `archivePath := filepath.Join(tempDir, asset.Name)` → `c.DownloadFile(ctx,
  asset.DownloadURL, archivePath)` → `VerifySHA256(archivePath, want)` — composing the SAME primitives the LANDED
  DownloadAndVerifyArchive composes, but via the asset (DownloadAndVerifyArchive takes goos/goarch and RE-SELECTS the asset,
  which is redundant given S1 already selected it and conflicts with this task's "leave tempDir for inspection" contract; see
  Anti-Patterns). (2) EXTRACT the single binary (FR-U5 step 5) via a private `extractBinary(archivePath, destDir, assetName)`:
  format from the asset-name suffix (.zip ⇒ archive/zip; else .tar.gz ⇒ archive/tar+compress/gzip — NO runtime import,
  platform-agnostic); entry base name `stagecoach`(+`.exe` if .zip); dest `tempDir/new-stagecoach`(+`.exe`); extract ONLY that
  one entry (not the whole archive); best-effort os.Chmod 0o755. (3) SANITY-RUN (FR-U5 step 6 + FR-U11) via a private
  `sanityCheck(ctx, newBinPath, release.Tag)`: exec `<newBinPath> --version` through a package-level `var execVersion`
  seam (default real os/exec; tests override), assert exit 0 AND output contains release.Tag. On ANY failure (verify/extract/
  sanity) return a typed error (ErrChecksumMissing/ErrChecksumMismatch propagate from download.go; NEW ErrArchiveNoBinary /
  ErrSanityVersionMismatch / ErrSanityRunFailed) and LEAVE tempDir for inspection (no os.RemoveAll / no os.Remove — the PRD's
  "leave the temp for inspection"; success-path cleanup is P1.M3.T2's job after the swap). On success return newBinPath; do
  NOT clean tempDir. NEVER touches the running binary (no rename/move/chmod of stagecoach — that is P1.M3.T2). Plus
  stage_test.go (package upgrade, internal tests): httptest server serving a REAL archive fixture (a compiled cmd/stubcli
  packed as "stagecoach") + checksums.txt; 4 cases — happy path (stubcli whose --version prints the tag → success), tampered
  archive (wrong SHA in checksums.txt → VerifySHA256 fails → ErrChecksumMismatch, NO extract), wrong-tag binary (sanity
  fails → ErrSanityVersionMismatch), non-zero-exit binary (sanity fails → ErrSanityRunFailed). Uses the cmd/stubcli PATTERN
  (env-driven cross-platform fake binary; clone stubtest.Build's `go build` to compile cmd/stubcli once per process) + the
  package-level execVersion seam (tests override; stage_test.go NOT parallel — it mutates the seam). Stdlib-only (archive/tar,
  archive/zip, compress/gzip, os/exec newly imported; NO internal/* — FR-U12); go.mod unchanged. ZERO production callers after
  this subtask (consumer is P1.M4.T2 runUpgrade) — expected.

---

## Goal

**Feature Goal**: Provide the download→verify→extract→sanity-run step of the FR-U5 direct-binary self-swap: given S1's
`(Release, Asset)` and a temp dir, produce a verified, sanity-checked new-binary path in that temp dir — WITHOUT ever
touching the running binary. A binary that fails to download, fails SHA256 verification, fails to extract, fails to run,
or misreports its version is NEVER swapped (FR-U11 abort-before-swap): StageNewBinary returns a typed error and leaves
tempDir for inspection. This is steps 4-6 of FR-U5 (download+verify / extract / sanity-run); the on-disk backup+rename
is P1.M3.T2's separate job.

**Deliverable** (2 new files in package upgrade; no edits to existing files):
1. **internal/upgrade/stage.go** — `StageNewBinary` (exported) + `extractBinary` + `sanityCheck` (private) + the
   `execVersion` package-level seam + NEW typed errors. Stdlib-only; no internal/*.
2. **internal/upgrade/stage_test.go** — httptest-backed tests: a `buildStubCLI(t)` helper (compiles cmd/stubcli), a
   `packArchive` helper (tar.gz/zip + checksums), the 4 contract cases (happy / tampered / wrong-tag / non-zero-exit).

**Success Definition**:
- `StageNewBinary(ctx, c, release, asset, tempDir)` downloads `asset.DownloadURL` to `tempDir/<asset.Name>`, verifies its
  SHA256 against `c.FetchChecksums(release)[asset.Name]`, extracts the `stagecoach`(.exe) entry to
  `tempDir/new-stagecoach`(.exe), sanity-runs it (`--version` output contains `release.Tag`, exit 0), and returns the
  newBinPath on success.
- A tampered archive (checksum mismatch) ⇒ returns an error wrapping `ErrChecksumMismatch` and does NOT extract (no
  `new-stagecoach` file appears in tempDir).
- A binary that prints the WRONG tag ⇒ returns an error wrapping `ErrSanityVersionMismatch`; a binary that exits non-zero
  ⇒ returns an error wrapping `ErrSanityRunFailed`. Both leave tempDir intact.
- StageNewBinary NEVER removes tempDir, NEVER removes the downloaded archive on failure (leaves it for inspection), and
  NEVER renames/moves/chmods the running `stagecoach` (the swap is P1.M3.T2).
- The `execVersion` seam is injectable; the 4 tests cover success + every failure branch.
- `go build ./...` + cross-build clean; `go vet ./internal/upgrade/...` clean; `gofmt -l` empty; `go test -race
  ./internal/upgrade/...` green; `make test`/`make lint` green; `go.mod`/`go.sum` unchanged; NO `internal/*` import (FR-U12).
- Scope: `git status` == only `internal/upgrade/stage.go` + `internal/upgrade/stage_test.go` (new). ZERO production callers
  (consumer is P1.M4.T2).

## User Persona (if applicable)

**Target User**: The `stagecoach upgrade` command layer (P1.M4.T2 `runUpgrade`), which calls ResolveTarget (S1) then
StageNewBinary (this task) to prepare the new binary, then the swap step (P1.M3.T2) to atomically replace the running one.
End users never call StageNewBinary directly.

**Use Case**: `stagecoach upgrade` (direct-binary channel) → ResolveTarget picks v1.2.3 linux_amd64 → StageNewBinary
downloads the tar.gz, verifies its SHA256 against checksums.txt, extracts `stagecoach` to `tempDir/new-stagecoach`,
runs `<tempDir>/new-stagecoach --version` (confirms "v1.2.3" + exit 0) → returns the path → P1.M3.T2 backs up the old
binary and atomically renames new-stagecoach→stagecoach. If the download was tampered, or the binary is corrupt/wrong,
StageNewBinary aborts with a clear error and the running binary is untouched.

**User Journey**: user runs `stagecoach upgrade` → runUpgrade (P1.M4.T2) → ResolveTarget → StageNewBinary (downloads to a
fresh tempDir) → [verify ok → extract → sanity ok → newBinPath] OR [verify/sanity FAIL → typed error, exit non-zero,
running binary unchanged]. On success, P1.M3.T2 swaps. The tempDir is cleaned by P1.M3.T2 on the success path; on failure
it is left for the user/dev to inspect (PRD).

**Pain Points Addressed**: FR-U5 steps 4-6 + FR-U11 — the self-swap must NEVER replace the running binary with an
unverified or broken one. StageNewBinary is the gate: download+verify (cryptographic integrity), extract (the right
binary), sanity-run (it actually runs and reports the right version) — all BEFORE any on-disk swap. A failure at any step
aborts cleanly.

## Why

- **FR-U5 steps 4-6 / §9.29**: the self-swap sequence is download+verify → extract → sanity-run → backup → swap.
  StageNewBinary owns the first three (steps 4-6); P1.M3.T2 owns backup+swap. Splitting here gives a clean, independently
  testable "prepare the new binary" boundary that the command layer composes.
- **FR-U11 abort-before-swap**: a binary that fails to download, fails SHA256, fails to extract, fails to run, or
  misreports its version must NEVER reach the swap. StageNewBinary enforces this: every failure returns an error BEFORE
  the running binary is touched. This is the load-bearing safety property of self-update.
- **Thin composition of LANDED primitives**: the download/verify primitives (FetchChecksums, DownloadFile, VerifySHA256)
  are LANDED (P1.M1.T3.S2). StageNewBinary COMPOSES them (via the pre-selected asset) + adds extraction (stdlib) + the
  sanity-run (os/exec via a seam). No new network or crypto code.
- **Bounded, no-conflict scope**: one new production file (stage.go) + tests. stage.go (NOT swap.go — P1.M3.T2 owns it).
  No edit to releases.go/download.go/version.go/detect.go/delegate.go/resolve.go. Lands independently; needs only the
  Client + checksum primitives + S1's (Release, Asset) — all landed/landing.

## What

**User-visible behavior**: None directly (no caller yet — the command is P1.M4). Internally, StageNewBinary becomes the
authoritative "prepare a verified new binary in temp" entry point for the direct-swap path.

**Technical change** (one new file + tests; verbatim API in the Blueprint): download+verify via the asset → extract the
single binary → sanity-run through an injectable seam → return the path or a typed error (leaving tempDir on failure).

### Success Criteria
- [ ] `internal/upgrade/stage.go` exports `StageNewBinary(ctx context.Context, c *Client, release Release, asset Asset,
      tempDir string) (string, error)` + the NEW typed errors `ErrArchiveNoBinary`, `ErrSanityVersionMismatch`,
      `ErrSanityRunFailed` (each wrapped with %w at its use site so `errors.Is` reaches them).
- [ ] StageNewBinary: FetchChecksums → DownloadFile(asset.DownloadURL) → VerifySHA256 → extractBinary → sanityCheck →
      return newBinPath. Errors propagate the download.go sentinels (ErrChecksumMissing/ErrChecksumMismatch/ErrHTTP) +
      the new stage sentinels, UNWRAPPED-relative-to-the-sentinel (errors.Is-reachable).
- [ ] `extractBinary(archivePath, destDir, assetName) (string, error)`: .zip ⇒ archive/zip; else .tar.gz ⇒ archive/tar +
      compress/gzip; extracts ONLY the entry whose `filepath.Base == "stagecoach"`(+`.exe` if .zip); dest
      `destDir/new-stagecoach`(+`.exe`); best-effort `os.Chmod(dest, 0o755)`; missing entry ⇒ `ErrArchiveNoBinary`.
- [ ] `sanityCheck(ctx, path, wantTag) error` via the package-level `var execVersion` seam (default
      `exec.CommandContext(ctx, path, "--version").Output()`): exec error/non-zero exit ⇒ `ErrSanityRunFailed`;
      output lacks `wantTag` ⇒ `ErrSanityVersionMismatch`.
- [ ] On ANY failure, StageNewBinary returns a typed error and LEAVES tempDir (no os.RemoveAll, no os.Remove of the
      archive). On success, returns newBinPath and does NOT clean tempDir (P1.M3.T2 owns success-path cleanup).
- [ ] StageNewBinary NEVER renames/moves/chmods any path outside tempDir (the running binary is untouched — P1.M3.T2's job).
- [ ] stage.go imports stdlib only (context, os, os/exec, path/filepath, bytes, archive/tar, archive/zip, compress/gzip,
      fmt, io, strings); NO internal/* (FR-U12); NO runtime (format from the asset-name suffix).
- [ ] stage.go is a FILE comment (not `// Package upgrade` — releases.go owns it).
- [ ] stage_test.go covers: happy path (real stubcli archive → success), tampered archive (wrong SHA → ErrChecksumMismatch,
      no extract), wrong-tag binary (→ ErrSanityVersionMismatch), non-zero-exit binary (→ ErrSanityRunFailed). Uses httptest
      + a compiled cmd/stubcli (the `buildStubCLI` helper) + the execVersion seam. stage_test.go does NOT call t.Parallel().
- [ ] `go build ./...` + cross-build (windows/linux/darwin) clean; `go vet ./internal/upgrade/...` clean; `gofmt -l` empty;
      `go test -race ./internal/upgrade/...` + `make test`/`make lint` green; `go.mod`/`go.sum` unchanged.
- [ ] Scope: `git status` == only `internal/upgrade/stage.go` + `internal/upgrade/stage_test.go` (new). NO swap.go.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim StageNewBinary/extractBinary/sanityCheck API, the exact download/verify primitives to compose
(FetchChecksums/DownloadFile/VerifySHA256 — all LANDED, with signatures + sentinels), the DownloadAndVerifyArchive-vs-asset
reconciliation (WHY compose the primitives directly instead of calling it — §2/Anti-Patterns), the file-name decision (stage.go
NOT swap.go — S1 reserved swap.go for P1.M3.T2), the extract format-from-asset-suffix rule (platform-agnostic, no runtime
import) + the entry/dest naming + chmod, the sanity-run semantics + the package-level execVersion seam (+ why stage_test.go
must not be parallel), the "leave tempDir on failure / don't clean on success" cleanup contract, the NEW typed errors + the
download sentinels to propagate, the test harness (cmd/stubcli pattern + buildStubCLI cloning stubtest.Build + packArchive +
httptest + the fake Release with absolute DownloadURLs + t.Setenv to drive stubcli), the 4 contract test cases, and the grep guards.

### Documentation & References

```yaml
# MUST READ — the authoritative findings (the DownloadAndVerifyArchive reconciliation, the file-name decision, the harness, the cleanup contract)
- docfile: plan/017_397abce9deb1/P1M3T1S2/research/findings.md
  why: "§0 the FILE-NAME decision (stage.go NOT swap.go — S1 reserved swap.go for P1.M3.T2; mirrors S1's resolve.go choice);
        §1 the LANDED primitives (FetchChecksums/DownloadFile/VerifySHA256 signatures + sentinels; Release/Asset/Client);
        §2 THE DownloadAndVerifyArchive-vs-asset RECONCILIATION (compose the primitives via the asset — do NOT call
        DownloadAndVerifyArchive: it takes goos/goarch + re-selects + removes-on-failure); §3 extraction (format from asset
        suffix, entry/dest naming, chmod, no runtime import); §4 the sanity-run + the execVersion seam + the no-parallel caveat;
        §5 the cleanup contract (leave tempDir on failure, don't clean on success); §6 the test harness (cmd/stubcli +
        buildStubCLI + packArchive + httptest + t.Setenv); §7 scope fences; §8 validation."
  critical: "§0: file is stage.go (NOT swap.go). §2: do NOT call DownloadAndVerifyArchive — compose FetchChecksums+DownloadFile+
             VerifySHA256 via the asset. §4: execVersion is a package-level var; stage_test.go must NOT be parallel. §5: LEAVE
             tempDir on failure (no os.Remove/os.RemoveAll); don't clean on success (P1.M3.T2's job)."

# MUST READ — the previous PRP (S1 ResolveTarget — the (Release, Asset) source + the swap.go reservation that drives the file-name decision)
- docfile: plan/017_397abce9deb1/P1M3T1S1/PRP.md
  why: "Defines ResolveTarget → (Release, Asset) (StageNewBinary's input). Its Anti-Patterns EXPLICITLY reserve swap.go for
        P1.M3.T2.S1 ('swap.go is P1.M3.T2.S1's; resolve.go avoids the collision') — that reservation is WHY this task uses
        stage.go. Its package conventions (file-comment-not-package-doc, injectable seam, exported sentinels propagated
        unwrapped, stdlib-only walled-off FR-U12, ZERO production callers) are the conventions stage.go must match."
  critical: "Do NOT create swap.go (P1.M3.T2 collision). Do NOT re-resolve the target (S1 owns that — StageNewBinary takes the
             asset as input). Match S1's conventions exactly (file comment, sentinels, stdlib-only)."

# MUST READ — the download/verify primitives StageNewBinary composes (TREAT AS LANDED; consume, don't rebuild)
- file: internal/upgrade/download.go
  why: "FetchChecksums (finds *_checksums.txt asset by checksumsName(tag)/suffix, downloads DownloadURL, parses '<64hex>  <fn>'
        → map[fn]hexsum; sentinels ErrNoChecksumsFile/ErrChecksumParse/ErrHTTP). DownloadFile(ctx, url, dest) (streams to dest,
        non-2xx/transport ⇒ ErrHTTP, removes partial on copy/close). VerifySHA256(path, want) (constant-time compare; mismatch
        ⇒ ErrChecksumMismatch). DownloadAndVerifyArchive (the function this task does NOT call — see §2). assetName/checksumsName
        (in-package naming helpers). Sentinels all EXPORTED + wrapped with %w ⇒ errors.Is-reachable."
  pattern: "download.go composes FetchChecksums→DownloadFile→VerifySHA256 in DownloadAndVerifyArchive; StageNewBinary composes the
            SAME three, but via the asset (no SelectAsset) and without the os.Remove-on-failure (leave-for-inspection)."
  critical: "DownloadFile uses the ABSOLUTE DownloadURL (BaseURL is metadata-only). FetchChecksums finds the checksums asset BY
             NAME in release.Assets — so the test's fake Release MUST include a checksums Asset whose DownloadURL serves the body.
             Do NOT call DownloadAndVerifyArchive (it takes goos/goarch + re-selects + removes-on-failure — wrong for this caller)."

# MUST READ — Release/Asset/Client types (the input shape)
- file: internal/upgrade/releases.go   # Asset (:44) {Name, DownloadURL, Size}; Release (:53) {Tag, Assets}; Client (:61)
  why: "StageNewBinary's (release, asset) params + the Client receiver for FetchChecksums/DownloadFile. httpClient() ⇒
        http.DefaultClient when HTTP==nil (so a bare &Client{} works in tests)."
  critical: "releases.go OWNS the package doc (line 1) — stage.go must start with a FILE comment, not '// Package upgrade'."

# MUST READ — the compile-a-stub-in-tests pattern to clone (buildStubCLI)
- file: internal/stubtest/stubtest.go   # Build (:44) compiles cmd/stubagent via `go build -buildvcs=false -o <tmp> <importpath>` with build.Dir=moduleRoot()
  why: "The proven pattern for compiling a test binary once per process (sync.Once cache, CWD-independent via moduleRoot()).
        Clone it in stage_test.go as buildStubCLI(t) targeting github.com/dabstractor/stagecoach/cmd/stubcli."
  pattern: "stubOnce.Do(func(){ ... exec.Command(goPath,'build','-buildvcs=false','-o',stubPath,IMPORTPATH); build.Dir=moduleRoot() ...});
            return stubPath. Reuse stubtest's moduleRoot() logic (it is unexported — re-derive via runtime.Caller OR a simpler
            `go env GOMOD` OR set build.Dir to the repo root found from the test file path)."
  gotcha: "stubtest.Build hardcodes cmd/stubagent and its stubPath — do NOT reuse its cache; write a SEPARATE buildStubCLI with its
           own sync.Once + path. The IMPORTPATH is github.com/dabstractor/stagecoach/cmd/stubcli."

# MUST READ — the fake binary itself (cmd/stubcli — env-driven, cross-platform)
- file: cmd/stubcli/main.go   # prints STAGECOACH_STUBCLI_OUT, exits STAGECOACH_STUBCLI_EXIT, ignores args
  why: "The 'cmd/stubcli pattern' the contract cites. It is a tiny cross-platform compiled binary driven by env: prints
        STUBCLI_OUT to stdout, exits STUBCLI_EXIT, ignores argv (so `<stubcli> --version` works). The sanity-run exec inherits
        os.Environ() (cmd.Env unset), so the test drives the stub via t.Setenv(STUBCLI_OUT/EXIT)."
  pattern: "Happy path: t.Setenv('STAGECOACH_STUBCLI_OUT', release.Tag) (+ EXIT unset ⇒ 0) → stub prints the tag → sanity passes.
            Wrong tag: t.Setenv OUT='v9.9.9-wrong' → sanity fails (ErrSanityVersionMismatch). Non-zero: t.Setenv EXIT='1' → exec
            fails → sanity fails (ErrSanityRunFailed). The stubcli is packed into the archive AS 'stagecoach' (extraction is real)."
  critical: "The sanity-run's exec.CommandContext must NOT set cmd.Env (so the child inherits os.Environ() + the t.Setenv values).
             If the override sets cmd.Env explicitly, it must append os.Environ() FIRST so the stub's env vars survive."

# CONTEXT — the exec.CommandContext precedent (how the repo runs+captures a binary)
- file: internal/provider/executor.go   # :51 exec.CommandContext(ctx, spec.Command, spec.Args...); CombinedOutput/Output capture
  why: "Confirms the repo's exec idiom (CommandContext + context for cancellation; Output() for stdout). StageNewBinary's default
        execVersion uses exec.CommandContext(ctx, path, '--version').Output() — same idiom."

# CONTEXT — version.go (NOT imported by stage.go — currentVersion is download.go's concern)
- file: internal/upgrade/version.go   # currentVersion (: used by download.go's User-Agent header)
  why: "Confirms stage.go does NOT need version logic (the tag-match in sanityCheck is a plain bytes.Contains on release.Tag — no
        semver compare). Compare/CurrentSemver are the COMMAND LAYER's (P1.M4.T2) concern, not StageNewBinary's."
  critical: "Do NOT import version.go into stage.go (the tag-match is a substring check, not a semver compare)."

# CONTEXT — the downstream consumer (LANDS LATER; not this task)
- docfile: (P1.M3.T2.S1 PRP, when written)   # backup + atomic swap — consumes StageNewBinary's newBinPath
  why: "P1.M3.T2.S1 takes StageNewBinary's returned newBinPath, backs up the running binary, and atomically renames
        new-stagecoach→stagecoach (Unix os.Rename / Windows .old dance). It also owns the success-path tempDir cleanup. StageNewBinary
        STOPS at the verified newBinPath; it must NOT do any of the swap/cleanup."
  critical: "ZERO production callers of StageNewBinary after this subtask (only stage_test.go). The command layer (P1.M4.T2) composes
             ResolveTarget → StageNewBinary → [swap P1.M3.T2]. Do NOT add the command/caller here."
```

### Current Codebase tree (relevant slice)

```bash
internal/upgrade/
  releases.go        # READ-ONLY — Client/Release/Asset types; OWNS the package doc (line 1)
  releases_test.go   # READ-ONLY — newFakeClient/cannedLatest/cannedReleases (NOT reused by stage_test — it builds its own fake Release)
  download.go        # READ-ONLY — FetchChecksums/DownloadFile/VerifySHA256 (the primitives StageNewBinary composes) + sentinels
  version.go         # READ-ONLY — NOT imported by stage.go (tag-match is a substring, not semver)
  detect.go          # READ-ONLY (P1.M2.T1.S1) — no overlap
  delegate.go        # READ-ONLY (P1.M2.T2.S1) — the delegation path; no overlap
  resolve.go         # READ-ONLY (P1.M3.T1.S1, landing) — ResolveTarget → (Release, Asset) [StageNewBinary's input]
  resolve_test.go    # READ-ONLY (P1.M3.T1.S1) — no overlap
  stage.go           # CREATE — StageNewBinary + extractBinary + sanityCheck + execVersion seam + typed errors (THIS TASK)
  stage_test.go      # CREATE — httptest + cmd/stubcli + 4 contract cases (THIS TASK)
  # (swap.go is P1.M3.T2.S1's — do NOT create it here)
internal/stubtest/
  stubtest.go        # READ-ONLY — Build (the compile-a-stub pattern to clone for buildStubCLI)
cmd/stubcli/
  main.go            # READ-ONLY — the env-driven fake binary (the archive payload)
go.mod               # READ-ONLY — module github.com/dabstractor/stagecoach; stdlib only; NO new require
```

### Desired Codebase tree with files to be added

```bash
internal/upgrade/stage.go        # NEW — StageNewBinary + extractBinary + sanityCheck + execVersion + typed errors (stdlib only; file comment)
internal/upgrade/stage_test.go   # NEW — buildStubCLI + packArchive + httptest + 4 cases (happy/tampered/wrong-tag/non-zero-exit)
# (NO swap.go — P1.M3.T2.S1 owns it. NO edit to any existing file.)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (file name = stage.go, NOT swap.go): the contract says "In internal/upgrade/swap.go" but S1's PRP EXPLICITLY
// reserved swap.go for P1.M3.T2.S1 (Backup + atomic swap) and chose resolve.go to avoid the collision. Creating swap.go
// here would collide with the planned P1.M3.T2 swap.go. stage.go is semantically correct (StageNewBinary "stages" the
// new binary in temp; the on-disk "swap" is P1.M3.T2's distinct step) and mirrors S1's own deviation. Name it stage.go.

// CRITICAL (do NOT call DownloadAndVerifyArchive — compose the primitives via the asset): DownloadAndVerifyArchive(ctx,
// release, goos, goarch, destDir) takes goos/goarch and RE-SELECTS the asset (redundant — S1 already selected it) and
// REMOVES the partial archive on failure (conflicts with this task's "leave tempDir for inspection" contract). StageNewBinary
// composes FetchChecksums + DownloadFile + VerifySHA256 directly using the asset — the SAME primitives, via the asset, no
// re-selection, no removal-on-failure. This honors the contract's "download+verify" intent + the (release, asset) input.
// (See Anti-Patterns + findings §2.)

// CRITICAL (format + .exe from the ASSET-NAME SUFFIX, NOT runtime): the archive is .zip (windows) or .tar.gz (else) per
// assetName(); the binary entry is "stagecoach"(.exe for .zip). Tie BOTH to `strings.HasSuffix(assetName, ".zip")` so
// extraction is platform-agnostic and testable off-host (a linux CI host can extract a .tar.gz; the host-native format is
// what the test packs). Do NOT import "runtime" — it would couple extraction to the host and break cross-format tests.

// CRITICAL (the entry/dest NAMES differ on purpose): the archive ENTRY base name is "stagecoach"(.exe) — what goreleaser
// ships. The DEST path is "new-stagecoach"(.exe) — the rename avoids colliding with the running "stagecoach" in the same
// dir; P1.M3.T2 renames new-stagecoach→stagecoach at swap time. extractBinary maps entry "stagecoach" → dest "new-stagecoach".

// CRITICAL (extract ONLY the one binary entry — not the whole archive): the contract: "extract ONLY the single binary".
// Iterate the archive; on the entry whose filepath.Base == "stagecoach"(.exe), copy its bytes to dest; SKIP everything else.
// A tar.gz with extra files (README, LICENSE) must NOT extract them. Match by filepath.Base (robust to a "./" tar prefix).

// CRITICAL (LEAVE tempDir on failure — do NOT os.Remove/os.RemoveAll): the PRD contract: "on failure leave the temp for
// inspection". On verify/extract/sanity failure, return a typed error and leave tempDir entirely (the downloaded archive +
// any extracted binary stay for the dev to inspect). Do NOT mirror download.go's "remove partial archive" here — that
// protects callers who might re-extract a lingering tampered archive; StageNewBinary never extracts after a verify failure
// (it errors first), so leaving it is SAFE and matches the PRD. On SUCCESS, do NOT clean tempDir either (P1.M3.T2 owns the
// success-path cleanup after the swap). StageNewBinary does NO filesystem cleanup at all.

// CRITICAL (NEVER touch the running binary): StageNewBinary writes ONLY inside tempDir. It must NOT rename/move/chmod any
// path outside tempDir (the running stagecoach is P1.M3.T2's to swap). This is the FR-U11 invariant: a failure here leaves
// the running binary byte-for-byte unchanged.

// GOTCHA (the execVersion seam is a package-level var — stage_test.go must NOT be parallel): `var execVersion = func(...)`
// is mutated by tests; if stage_test.go called t.Parallel() two tests could race on it. Do NOT call t.Parallel() in
// stage_test.go. The sibling releases/detect/delegate tests are unaffected (they never touch execVersion) and stay parallel.

// GOTCHA (the sanity-run exec must inherit os.Environ so the stubcli's env vars survive): exec.CommandContext(ctx, path,
// "--version") with cmd.Env UNSET ⇒ the child inherits os.Environ() (incl. t.Setenv values). If a test override sets cmd.Env
// explicitly, it MUST prepend os.Environ() so STAGECOACH_STUBCLI_OUT/EXIT reach the stubcli. The DEFAULT execVersion leaves
// cmd.Env unset (correct); overrides that add env must append, not replace.

// GOTCHA (file comment, NOT package doc): releases.go line 1 owns "// Package upgrade". Start stage.go with a FILE comment
// ("// stage.go implements the FR-U5 download→verify→extract→sanity step…") — exactly as resolve.go/detect.go/download.go/
// delegate.go do. A second "// Package upgrade" splits the package overview.

// GOTCHA (best-effort chmod 0o755 after extraction): the extracted binary must be executable for the sanity-run (unix).
// archive/tar preserves mode bits from the header; archive/zip does NOT preserve unix mode. To be robust across both,
// always `_ = os.Chmod(dest, 0o755)` after writing (best-effort; near-no-op on windows). Do NOT fail extraction if chmod
// errors (it won't on a tempDir the test owns).

// GOTCHA (checksums.txt asset must be in the fake Release): FetchChecksums finds the checksums asset BY NAME in
// release.Assets (checksumsName(tag) or *_checksums.txt suffix) and downloads its DownloadURL. The test's fake Release MUST
// include BOTH the archive Asset AND a checksums Asset (with the right DownloadURL serving the "<sha>  <name>\n" body), or
// FetchChecksums returns ErrNoChecksumsFile.

// GOTCHA (stdlib-only — no internal/*): stage.go imports context/os/os/exec/path/filepath/bytes/archive/tar/archive/zip/
// compress/gzip/fmt/io/strings. NO internal/* (FR-U12 walled-off). NO runtime (format from asset suffix). NO new go.mod
// require (all stdlib). Adding internal/* or a new require is a walled-off violation / scope creep.
```

## Implementation Blueprint

### Data models and structure

```go
// NEW typed errors (stage.go), each wrapped with %w at its use site so errors.Is reaches them — mirrors download.go's convention.
var (
	ErrArchiveNoBinary      = errors.New("upgrade: archive has no stagecoach binary entry")
	ErrSanityVersionMismatch = errors.New("upgrade: staged binary --version does not report the target tag")
	ErrSanityRunFailed      = errors.New("upgrade: staged binary failed to run")
)

// execVersion is the sanity-run seam (the contract's "injectable exec runner"). Default: real os/exec. Tests override it
// to drive the cmd/stubcli fake binary (happy/wrong-tag/non-zero-exit). PACKAGE-LEVEL ⇒ stage_test.go must NOT be parallel.
var execVersion = func(ctx context.Context, path string) ([]byte, error) {
	return exec.CommandContext(ctx, path, "--version").Output() // cmd.Env unset ⇒ child inherits os.Environ()
}

// StageNewBinary signature (per the contract): takes the pre-selected asset (S1's output) + a tempDir.
// No new public structs. extractBinary + sanityCheck are private helpers.
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/upgrade/stage.go — file comment + imports + typed errors + execVersion seam + sanityCheck + extractBinary + StageNewBinary
  - FILE COMMENT header (NOT a package doc): "// stage.go implements the FR-U5 download→verify→extract→sanity-run step of the
    direct-binary self-swap (§9.29 FR-U5 steps 4-6 + FR-U11). It composes the GitHub Releases Client's download/verify
    primitives (download.go) using the pre-selected asset, extracts the single stagecoach binary from the archive (stdlib
    archive/tar+gzip / archive/zip), and sanity-runs it (--version reports the target tag) BEFORE any on-disk swap (P1.M3.T2).
    On any failure it leaves the staging tempDir for inspection and returns a typed error (FR-U11 abort-before-swap). It is
    walled off (FR-U12: stdlib-only, no internal/* imports). File comment only — releases.go owns the package doc."
  - IMPORTS: archive/tar, archive/zip, bytes, compress/gzip, context, fmt, io, os, os/exec, path/filepath, strings.
    (NO runtime — format from the asset-name suffix. NO internal/*.)
  - Typed errors: ErrArchiveNoBinary, ErrSanityVersionMismatch, ErrSanityRunFailed (var block, exported, godoc each citing FR-U5/FR-U11).
  - execVersion seam (the package-level var above).
  - sanityCheck(ctx context.Context, path, wantTag string) error:
        out, err := execVersion(ctx, path)
        if err != nil { return fmt.Errorf("sanity-run %s: %w", path, ErrSanityRunFailed) }
        if !bytes.Contains(out, []byte(wantTag)) {
            return fmt.Errorf("sanity-run %s: output %q lacks tag %q: %w", path, out, wantTag, ErrSanityVersionMismatch)
        }
        return nil
  - extractBinary(archivePath, destDir, assetName string) (string, error):
        exe := strings.HasSuffix(assetName, ".zip")         // .zip ⇒ windows ⇒ .exe; .tar.gz ⇒ no suffix
        entryBase := "stagecoach"; if exe { entryBase += ".exe" }
        destName := "new-stagecoach"; if exe { destName += ".exe" }
        dest := filepath.Join(destDir, destName)
        if exe { /* archive/zip path */ } else { /* archive/tar+compress/gzip path */ }
        // BOTH paths: open the archive, iterate entries, find the one whose filepath.Base == entryBase, copy its bytes to dest.
        // On a missing entry: return fmt.Errorf("extract %s: want %s: %w", archivePath, entryBase, ErrArchiveNoBinary).
        // After writing: _ = os.Chmod(dest, 0o755)  (best-effort; covers zip-on-unix + any mode loss).
        return dest, nil
    - ZIP path: `zr, err := zip.OpenReader(archivePath)` (defer Close); for _, f := range zr.File { if filepath.Base(f.Name)==entryBase {
        rc, err := f.Open(); defer rc.Close(); f2, err := os.Create(dest); io.Copy(f2, rc); f2.Close() } }.
    - TAR.GZ path: `f, err := os.Open(archivePath)`; `gz, err := gzip.NewReader(f)` (defer gz.Close); `tr := tar.NewReader(gz)`;
        for { hdr, err := tr.Next(); if err == io.EOF { break }; if err != nil { return ..., err };
        if filepath.Base(hdr.Name) == entryBase { out, err := os.Create(dest); io.Copy(out, tr); out.Close() } }.
    - GOTCHA: extract ONLY the matching entry; skip all others. If the loop completes without finding entryBase ⇒ ErrArchiveNoBinary.
  - StageNewBinary(ctx context.Context, c *Client, release Release, asset Asset, tempDir string) (string, error):
        // (1) Download + verify via the pre-selected asset (FR-U5 step 4 + FR-U11).
        sums, err := c.FetchChecksums(ctx, release)
        if err != nil { return "", err }                                   // propagates ErrNoChecksumsFile/ErrChecksumParse/ErrHTTP
        want, ok := sums[asset.Name]
        if !ok { return "", fmt.Errorf("asset %q not in checksums.txt: %w", asset.Name, ErrChecksumMissing) }
        archivePath := filepath.Join(tempDir, asset.Name)
        if err := c.DownloadFile(ctx, asset.DownloadURL, archivePath); err != nil { return "", err }   // ErrHTTP
        if err := VerifySHA256(archivePath, want); err != nil { return "", err }                        // ErrChecksumMismatch
        // (2) Extract the single binary (FR-U5 step 5).
        newBinPath, err := extractBinary(archivePath, tempDir, asset.Name)
        if err != nil { return "", err }                                   // ErrArchiveNoBinary / I/O
        // (3) Sanity-run (FR-U5 step 6 + FR-U11 abort-before-swap).
        if err := sanityCheck(ctx, newBinPath, release.Tag); err != nil { return "", err }  // ErrSanityRunFailed / ErrSanityVersionMismatch
        // (4) Success: return the staged path. Do NOT clean tempDir (P1.M3.T2 owns success-path cleanup after the swap).
        return newBinPath, nil
    - GODOC on StageNewBinary: cite FR-U5 steps 4-6 + FR-U11; state it composes FetchChecksums/DownloadFile/VerifySHA256 via
      the asset (NOT DownloadAndVerifyArchive — that re-selects); state extraction is format-from-asset-suffix; state the
      sanity-run (--version contains release.Tag, exit 0) via the execVersion seam; state on failure it LEAVES tempDir for
      inspection and returns a typed error; state it NEVER touches the running binary (P1.M3.T2 owns the swap); state ZERO
      production callers (P1.M4.T2 is the consumer).
  - NAMING: StageNewBinary (exported); extractBinary/sanityCheck (private); execVersion (package-level seam); ErrArchiveNoBinary/
    ErrSanityVersionMismatch/ErrSanityRunFailed (exported sentinels).
  - GOTCHA: NO os.Remove / os.RemoveAll anywhere (leave tempDir on failure AND success). NO rename/move/chmod outside tempDir.

Task 2: CREATE internal/upgrade/stage_test.go — buildStubCLI + packArchive + httptest + the 4 contract cases
  - IMPORTS: archive/tar, archive/zip, bytes, compress/gzip, context, crypto/sha256, encoding/hex, errors, fmt, io, net/http,
    net/http/httptest, os, os/exec, path/filepath, runtime, strings, sync, testing.
  - Do NOT call t.Parallel() in ANY stage_test.go test (they mutate the package-level execVersion seam).
  - HELPERS:
    - `var (stubCLIONCE sync.Once; stubCLIPath string)` + `buildStubCLI(t) string` — clones stubtest.Build: once-per-process
      `go build -buildvcs=false -o <tmpdir>/stagecoach-stubcli(.exe) github.com/dabstractor/stagecoach/cmd/stubcli` with
      build.Dir = the repo root (derive via the test file's path: filepath.Dir of the upgrade pkg → up to module root; OR run
      `go env GOMOD`). t.Skipf if `go` not on PATH. t.Helper().
    - `hostAssetName(tag string) string` → assetName(tag, runtime.GOOS, runtime.GOOS) via the in-package helper (stage_test is
      package upgrade ⇒ can call assetName directly): "stagecoach_<v>_<os>_<arch>.zip" (windows) / ".tar.gz" (else).
    - `hostEntryName() string` → "stagecoach" + (".exe" if windows else "").
    - `packArchive(t, stubPath, entryName, assetName string) ([]byte, string)` → packs stubPath's bytes under entryName into
      the host-native format (zip if windows else tar.gz), returns (archiveBytes, sha256hex). Sets tar header Mode 0o755.
    - `setupRelease(t, archiveBytes, sha, tag string) (Release, *Client, *httptest.Server, func())` → httptest server serving
      archiveBytes at /archive and "<sha>  <assetName>\n" at /checksums; builds Release{Tag:tag, Assets:[{Name:assetName,
      DownloadURL: ts.URL+"/archive"}, {Name: checksumsName(tag), DownloadURL: ts.URL+"/checksums"}]} + &Client{Repo:"o/r"};
      returns (release, c, ts, cleanup). The cleanup restores execVersion (saved before the test overrides it).
  - CASES:
    - TestStageNewBinary_HappyPath: stub := buildStubCLI(t); archive, sha := packArchive(t, stub, hostEntryName(), hostAssetName("v1.2.3"));
      release, c, ts, cleanup := setupRelease(t, archive, sha, "v1.2.3"); defer cleanup(); defer ts.Close();
      tempDir := t.TempDir(); t.Setenv("STAGECOACH_STUBCLI_OUT", "v1.2.3")  // stubcli prints the tag (EXIT unset ⇒ 0)
      (execVersion left as the DEFAULT real exec — the extracted stubcli inherits os.Environ and prints the tag).
      newBinPath, err := StageNewBinary(context.Background(), c, release, release.Assets[0], tempDir)
      assert err==nil; newBinPath == filepath.Join(tempDir, "new-stagecoach"+exeSuffix); the file exists + is executable;
      (proves download+verify+extract+sanity all succeed end-to-end with a REAL compiled binary).
    - TestStageNewBinary_TamperedArchive: like happy but setupRelease serves a DIFFERENT archive than the checksums expect
      (e.g. checksums body lists a bogus sha "000...000", or serve archive2 while checksums hashes archive1). Leave execVersion
      default. newBinPath, err := StageNewBinary(...); assert err != nil && errors.Is(err, ErrChecksumMismatch); AND assert
      NO extract happened: `_, statErr := os.Stat(filepath.Join(tempDir, "new-stagecoach"+exeSuffix)); errors.Is(statErr,
      os.ErrNotExist)` (the tampered archive is NEVER extracted). (tempDir left for inspection — do not assert it's gone.)
    - TestStageNewBinary_WrongTag: like happy (real archive, real extract), but override execVersion to run the real extracted
      stubcli with STUBCLI_OUT="v9.9.9-wrong" (so it prints the wrong tag). Use a savedDefault := execVersion; execVersion = func(...)
      { cmd := exec.CommandContext(ctx, path, "--version"); cmd.Env = append(os.Environ(), "STAGECOACH_STUBCLI_OUT=v9.9.9-wrong");
      return cmd.Output() }; defer { execVersion = savedDefault }. assert errors.Is(err, ErrSanityVersionMismatch).
      (The binary was extracted + ran, but misreported → NEVER swapped.)
    - TestStageNewBinary_NonZeroExit: like WrongTag but STUBCLI_EXIT=1 (override sets cmd.Env append "STAGECOACH_STUBCLI_EXIT=1").
      The stubcli exits 1 ⇒ exec.Output() returns *exec.ExitError ⇒ sanityCheck wraps ErrSanityRunFailed. assert errors.Is(err,
      ErrSanityRunFailed). (The binary ran but exited non-zero → NEVER swapped.)
  - COVERAGE: success + verify-fail (no extract) + sanity-version-mismatch + sanity-run-failed. Every StageNewBinary branch.
  - GOTCHA: each test that overrides execVersion MUST save+restore it (defer) — or use a setupRelease-style cleanup. The
    happy/tampered tests leave execVersion as the DEFAULT real exec (the stubcli inherits t.Setenv values). The wrong-tag/
    non-zero tests OVERRIDE execVersion to inject the stub env into the subprocess (because t.Setenv alone would affect the
    stubcli run by the DEFAULT exec too — but overriding makes the env-injection explicit + test-local). Either approach works;
    prefer the override for wrong-tag/non-zero so the env injection is visible in the test.

Task 3: VERIFY — build, vet, format, focused + full tests, lint, grep guards
  - go build ./... ; GOOS=windows go build ./... ; GOOS=linux go build ./... ; GOOS=darwin go build ./...
  - go vet ./internal/upgrade/...
  - gofmt -l internal/upgrade/stage.go internal/upgrade/stage_test.go   # empty
  - go test ./internal/upgrade/ -run 'StageNewBinary|SanityCheck|ExtractBinary' -v
  - go test -race ./internal/upgrade/...   # stage_test NOT parallel (mutates execVersion); siblings stay parallel + green
  - make test ; make lint
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: download+verify via the asset (NOT DownloadAndVerifyArchive — that re-selects + removes-on-failure):
sums, err := c.FetchChecksums(ctx, release)                 // finds *_checksums.txt asset, downloads+parses → map[fn]hexsum
if err != nil { return "", err }
want, ok := sums[asset.Name]
if !ok { return "", fmt.Errorf("asset %q not in checksums.txt: %w", asset.Name, ErrChecksumMissing) }
archivePath := filepath.Join(tempDir, asset.Name)
if err := c.DownloadFile(ctx, asset.DownloadURL, archivePath); err != nil { return "", err }  // ABSOLUTE url
if err := VerifySHA256(archivePath, want); err != nil { return "", err }                      // ErrChecksumMismatch

// PATTERN: extract ONLY the matching entry (tar.gz shown; zip analogous via zip.OpenReader + f.Open):
f, _ := os.Open(archivePath); defer f.Close()
gz, _ := gzip.NewReader(f); defer gz.Close()
tr := tar.NewReader(gz)
found := false
for {
	hdr, err := tr.Next()
	if err == io.EOF { break }
	if err != nil { return "", err }
	if filepath.Base(hdr.Name) == entryBase {   // "stagecoach" / "stagecoach.exe"
		out, err := os.Create(dest)             // dest = tempDir/new-stagecoach(.exe)
		if err != nil { return "", err }
		if _, err := io.Copy(out, tr); err != nil { out.Close(); return "", err }
		out.Close()
		found = true
		break
	}
}
if !found { return "", fmt.Errorf("extract %s: want %s: %w", archivePath, entryBase, ErrArchiveNoBinary) }
_ = os.Chmod(dest, 0o755)   // best-effort (unix exec; near-no-op on windows)

// PATTERN: the sanity-run seam + the tag-match (errors.Is-reachable sentinels):
var execVersion = func(ctx context.Context, path string) ([]byte, error) {
	return exec.CommandContext(ctx, path, "--version").Output()   // cmd.Env unset ⇒ inherits os.Environ()
}
func sanityCheck(ctx context.Context, path, wantTag string) error {
	out, err := execVersion(ctx, path)
	if err != nil { return fmt.Errorf("sanity-run %s: %w", path, ErrSanityRunFailed) }
	if !bytes.Contains(out, []byte(wantTag)) {
		return fmt.Errorf("sanity-run %s: output %q lacks tag %q: %w", path, out, wantTag, ErrSanityVersionMismatch)
	}
	return nil
}

// PATTERN: the happy-path test — REAL compiled stubcli, default exec, t.Setenv drives the stub:
stub := buildStubCLI(t)
archive, sha := packArchive(t, stub, hostEntryName(), hostAssetName("v1.2.3"))
release, c, ts, cleanup := setupRelease(t, archive, sha, "v1.2.3"); defer cleanup(); defer ts.Close()
t.Setenv("STAGECOACH_STUBCLI_OUT", "v1.2.3")  // stubcli prints the tag; EXIT unset ⇒ 0
newBinPath, err := StageNewBinary(ctx, c, release, release.Assets[0], t.TempDir())  // default execVersion runs the real stub
```

### Integration Points

```yaml
UPGRADE PACKAGE (internal/upgrade/stage.go):
  - +StageNewBinary(ctx, c *Client, release, asset, tempDir) (string, error); +extractBinary; +sanityCheck; +execVersion seam;
    +ErrArchiveNoBinary/ErrSanityVersionMismatch/ErrSanityRunFailed.

DOWNSTREAM (this subtask ENABLES but does NOT build):
  - P1.M3.T2.S1 (backup + atomic swap): takes StageNewBinary's newBinPath, backs up the running binary, atomically renames
    new-stagecoach→stagecoach (Unix os.Rename / Windows .old dance), and cleans tempDir on the success path. StageNewBinary
    STOPS at the verified newBinPath; it must NOT do the swap/cleanup.
  - P1.M3.T3.S1 (--rollback): reuses the swap primitives (not StageNewBinary directly).
  - P1.M4.T2 (runUpgrade dispatcher): builds the Client, calls ResolveTarget → StageNewBinary → [swap P1.M3.T2]. The command
    layer does the newer-than-current check (Compare + CurrentSemver) — NOT StageNewBinary (it just stages whatever asset it's given).
  - ZERO production callers of StageNewBinary after this subtask (only stage_test.go) — expected.

SCOPE FENCES: NO swap.go file (P1.M3.T2 owns it); NO backup/rename/rollback (P1.M3.T2/T3); NO cobra/command/runUpgrade/confirmation
  prompt/--check (P1.M4); NO edit to releases.go/download.go/version.go/detect.go/delegate.go/resolve.go; NO DownloadAndVerifyArchive
  call (compose the primitives via the asset — see Anti-Patterns); NO internal/* import (FR-U12); NO package doc (releases.go owns
  it); NO new go.mod require; NO filesystem cleanup in StageNewBinary (leave tempDir on failure; P1.M3.T2 cleans on success); NO
  rename/move/chmod outside tempDir (the running binary is untouched).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build + vet (stage.go must compile alongside the parallel resolve.go/delegate.go/detect.go — no symbol clash; the new
# archive/* + os/exec imports are stdlib).
go build ./...
GOOS=windows go build ./...
GOOS=linux   go build ./...
GOOS=darwin  go build ./...
# Expected: clean. A failure likely means a symbol clash (StageNewBinary/execVersion already exist — they don't), an unwanted
#           import (internal/* or runtime), or a missing/error in the tar/zip extraction logic. Cross-build proves the
#           format-from-asset-suffix extraction is platform-agnostic (no runtime.GOOS in stage.go).

# Vet.
go vet ./internal/upgrade/...
# Expected: clean. (archive/tar + archive/zip extraction is a common vet target — ensure io.Copy errors are checked, defers close.)

# Format.
gofmt -l internal/upgrade/stage.go internal/upgrade/stage_test.go
# Expected: empty. If listed: gofmt -w the file(s).

# Lint.
make lint      # golangci-lint (staticcheck/gosimple/govet/errcheck/ineffassign/unused)
# Expected: zero errors. `unused` stays clean (StageNewBinary is EXPORTED ⇒ not unused even before a caller; the typed errors +
#           execVersion are used by stage.go/stage_test.go; extractBinary/sanityCheck are used by StageNewBinary). errcheck
#           satisfied (io.Copy/os.Create/os.Chmod errors checked or explicitly _ = where best-effort).

# Scope guard: only stage.go + stage_test.go added; no existing file edited; NO swap.go.
git status --short
# Expected: ?? internal/upgrade/stage.go  ?? internal/upgrade/stage_test.go  (only).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new StageNewBinary tests (focused). NOTE: stage_test.go is NOT parallel (mutates execVersion).
go test ./internal/upgrade/ -run 'StageNewBinary|SanityCheck|ExtractBinary' -v
# Expected: PASS — HappyPath (real stubcli → newBinPath); TamperedArchive (ErrChecksumMismatch, no extract); WrongTag
#           (ErrSanityVersionMismatch); NonZeroExit (ErrSanityRunFailed).

# The full upgrade package incl. the parallel detect/download/delegate/releases/resolve tests.
go test ./internal/upgrade/... -v
go test -race ./internal/upgrade/...
# Expected: green (race detector). stage_test.go does NOT call t.Parallel() (mutates execVersion) — the race detector is still
#           happy because the sibling tests never touch execVersion. resolve_test.go (S1) + the others pass unaffected —
#           stage.go is additive and imports none of them.

# Full repo suite.
make test
# Expected: green. stage.go is additive, stdlib-only, imports no internal/*.

# NOTE: there is NO coverage-gate on internal/upgrade (the §20.3 gate is internal/{git,provider,generate,config} only). The
# 4-case suite gives strong coverage of StageNewBinary's branches regardless.
```

### Level 3: Integration Testing (System Validation)

```bash
# There is no integration/e2e surface for this task — StageNewBinary has no production caller yet (the command is P1.M4; the
# runUpgrade dispatcher is P1.M4.T2; the swap is P1.M3.T2). The unit tests (Level 2) ARE the contract: they spin a REAL
# httptest server, download+verify+extract a REAL compiled stubcli, and sanity-run it (the happy path is a genuine end-to-end
# download→extract→exec with a real binary — not a mock). A full e2e (real GitHub release → staged → swapped) is P1.M4.T3.S2/S3.

# Sanity: the package still builds into the binary (no downstream compile break from the new symbols).
go build ./...
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: stage.go is a FILE comment, not a package doc.
head -3 internal/upgrade/stage.go | grep -q '^// stage.go' && echo "OK: file comment"
grep -c '^// Package upgrade' internal/upgrade/stage.go
# Expected: 0 (releases.go owns the package doc).

# Guard 2: NO internal/* imports (walled off, FR-U12); NO runtime import (format from asset suffix).
grep -nE 'stagecoach/internal|"runtime"' internal/upgrade/stage.go
# Expected: empty. stage.go imports only stdlib (archive/tar, archive/zip, compress/gzip, os/exec, etc.).

# Guard 3: stdlib-only — no new go.mod require.
git diff go.mod go.sum
# Expected: empty.

# Guard 4: StageNewBinary exists + composes the primitives (NOT DownloadAndVerifyArchive).
grep -n 'func StageNewBinary' internal/upgrade/stage.go
grep -cE 'c\.FetchChecksums|c\.DownloadFile|VerifySHA256\(' internal/upgrade/stage.go
# Expected: 1 StageNewBinary; 3 hits (FetchChecksums + DownloadFile + VerifySHA256 — composed via the asset).
grep -n 'DownloadAndVerifyArchive' internal/upgrade/stage.go
# Expected: empty (StageNewBinary does NOT call it — see Anti-Patterns).

# Guard 5: NO filesystem cleanup in StageNewBinary (leave tempDir on failure + success).
grep -nE 'os\.Remove|os\.RemoveAll' internal/upgrade/stage.go
# Expected: empty (StageNewBinary does no cleanup — the PRD "leave for inspection" + P1.M3.T2 success-path cleanup).

# Guard 6: StageNewBinary NEVER renames/moves the running binary (writes ONLY inside tempDir).
grep -nE 'os\.Rename|os\.Mkdir|os\.WriteFile' internal/upgrade/stage.go
# Expected: only os.Create inside extractBinary (within tempDir); NO os.Rename (the swap is P1.M3.T2's). (os.Create is used to
#           write the extracted binary to dest = tempDir/new-stagecoach — that's inside tempDir, allowed.)

# Guard 7: the execVersion seam + the 3 new sentinels exist.
grep -n 'var execVersion' internal/upgrade/stage.go
grep -nE 'ErrArchiveNoBinary|ErrSanityVersionMismatch|ErrSanityRunFailed' internal/upgrade/stage.go
# Expected: 1 execVersion var; 3 sentinel declarations (var block) + their use sites.

# Guard 8: format/.exe derived from the asset-name suffix (NOT runtime.GOOS).
grep -nE 'HasSuffix\(assetName, "\.zip"\)' internal/upgrade/stage.go
# Expected: ≥1 hit (the format + .exe decision keyed on the asset suffix).

# Guard 9: NO swap.go created (P1.M3.T2 owns it).
ls internal/upgrade/swap.go 2>/dev/null && echo "FAIL: swap.go exists (P1.M3.T2 conflict)" || echo "OK: no swap.go"
git status --short | grep swap.go
# Expected: "OK: no swap.go" + empty grep.

# Guard 10: ZERO production callers of StageNewBinary (consumer is P1.M4.T2).
grep -rn 'upgrade.StageNewBinary(\|\.StageNewBinary(' --include='*.go' internal/ cmd/ pkg/ | grep -v '_test.go' | grep -v 'func StageNewBinary'
# Expected: empty (no caller outside stage.go + tests).

# Guard 11: scope — only 2 files added.
git status --porcelain
# Expected: ?? internal/upgrade/stage.go  ?? internal/upgrade/stage_test.go  (only).

# Guard 12: stage_test.go does NOT call t.Parallel (mutates the execVersion seam).
grep -n 't\.Parallel()' internal/upgrade/stage_test.go
# Expected: empty.

# Regression: the sibling resolve/detect/download/delegate/releases tests still pass (no shared state except execVersion, which
# only stage_test mutates — and not in parallel).
go test ./internal/upgrade/ -run 'ResolveTarget|Detect|Download|SelectAsset|Delegate|Client_|Release' -v
# Expected: all PASS (the sibling tests, unaffected).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + cross-build (windows/linux/darwin) clean
- [ ] `go vet ./internal/upgrade/...` clean
- [ ] `gofmt -l internal/upgrade/stage.go internal/upgrade/stage_test.go` empty
- [ ] `go test -race ./internal/upgrade/...` green (stage_test NOT parallel; siblings parallel + green)
- [ ] `make test` + `make lint` pass; `go.mod`/`go.sum` unchanged

### Feature Validation
- [ ] `StageNewBinary(ctx, c, release, asset, tempDir) (string, error)` exists + the 3 new sentinels + the execVersion seam
- [ ] Download+verify via the asset (FetchChecksums + DownloadFile + VerifySHA256); ErrChecksumMissing/ErrChecksumMismatch propagate
- [ ] extractBinary: .zip ⇒ archive/zip; .tar.gz ⇒ archive/tar+gzip; extracts ONLY the stagecoach(.exe) entry → new-stagecoach(.exe);
      chmod 0o755; missing entry ⇒ ErrArchiveNoBinary
- [ ] sanityCheck: exec `<path> --version` via execVersion; non-zero/exec-error ⇒ ErrSanityRunFailed; output lacks tag ⇒ ErrSanityVersionMismatch
- [ ] Happy-path test: real stubcli archive → newBinPath returned; TamperedArchive → ErrChecksumMismatch + no extract;
      WrongTag → ErrSanityVersionMismatch; NonZeroExit → ErrSanityRunFailed
- [ ] StageNewBinary leaves tempDir on failure (no os.Remove/RemoveAll) + does not clean on success; NEVER touches the running binary

### Scope-Boundary Validation
- [ ] `git status` == only `internal/upgrade/stage.go` + `internal/upgrade/stage_test.go` (new)
- [ ] NO swap.go file (P1.M3.T2 owns it); NO edit to releases.go/download.go/version.go/detect.go/delegate.go/resolve.go
- [ ] NO `internal/*` import (FR-U12); NO `runtime` import (format from asset suffix); stdlib only
- [ ] NO DownloadAndVerifyArchive call (compose the primitives via the asset); NO filesystem cleanup; NO rename outside tempDir
- [ ] NO package doc in stage.go (releases.go owns it); file comment only
- [ ] NO backup/rename/rollback (P1.M3.T2/T3); NO cobra/command/runUpgrade (P1.M4)
- [ ] Grep guards 1–12 (Level 4) all pass

### Code Quality & Docs
- [ ] Mirrors the package conventions (injectable seam, exported sentinels propagated unwrapped, file-comment-not-package-doc,
      stdlib-only walled-off FR-U12, ZERO production callers)
- [ ] StageNewBinary godoc cites FR-U5 steps 4-6 + FR-U11 + the leave-tempDir-on-failure contract + the no-swap boundary
- [ ] Tests use a REAL compiled cmd/stubcli (the "cmd/stubcli pattern") + httptest (hermetic, no real GitHub, no rate-limit)
- [ ] stage_test.go does NOT call t.Parallel() (mutates the package-level execVersion seam)

---

## Anti-Patterns to Avoid

- ❌ Don't name the file swap.go. The contract says "In internal/upgrade/swap.go" but S1's PRP EXPLICITLY reserved swap.go for
  P1.M3.T2.S1 (Backup + atomic swap) and chose resolve.go to avoid the collision — the parallel_execution_context tells S2 to
  "not conflict with the previous PRP". Creating swap.go here would collide with the planned P1.M3.T2 swap.go. stage.go is
  semantically correct (StageNewBinary "stages" the new binary in temp; the on-disk "swap" is P1.M3.T2's distinct step) and
  mirrors S1's own deviation. Grep guard 9 enforces this.
- ❌ Don't call DownloadAndVerifyArchive. The contract says "(DownloadAndVerifyArchive)" but DownloadAndVerifyArchive(ctx,
  release, goos, goarch, destDir) takes **goos/goarch and RE-SELECTS the asset** (redundant — S1 already selected it), needs
  goos/goarch the (release, asset) input doesn't carry, and **REMOVES the partial archive on failure** (conflicts with this
  task's "leave tempDir for inspection" contract). StageNewBinary composes the SAME primitives (FetchChecksums + DownloadFile +
  VerifySHA256) directly via the asset — identical verify behavior, no redundant re-selection, no removal-on-failure. This honors
  the contract's "download+verify" intent + the (release, asset) input. Grep guard 4 enforces it (DownloadAndVerifyArchive absent).
- ❌ Don't import "runtime" into stage.go. The archive format (.zip vs .tar.gz) and the binary suffix (.exe) are derived from the
  ASSET-NAME SUFFIX (`strings.HasSuffix(assetName, ".zip")`), not runtime.GOOS. This keeps extraction platform-agnostic and
  testable off-host (a linux CI host extracts a .tar.gz; a windows-specific test could extract a .zip). Tying it to runtime.GOOS
  would couple extraction to the host and break cross-format reasoning. (The TEST uses runtime.GOOS/GOARCH to pick the host-native
  format to pack — that's test-only, fine.)
- ❌ Don't extract the whole archive. The contract: "extract ONLY the single 'stagecoach'/'stagecoach.exe' binary, not the whole
  archive." Iterate the archive; copy ONLY the entry whose `filepath.Base == "stagecoach"`(.exe); SKIP every other entry (README,
  LICENSE, checksums). A tarball with extra files must NOT materialize them in tempDir.
- ❌ Don't clean up tempDir on failure (or success). The PRD contract: "on failure leave the temp for inspection"; "Cleanup the
  temp on the success path is the swap step's job (P1.M3.T2)". StageNewBinary does NO os.Remove / os.RemoveAll — on failure the
  downloaded archive + any extracted binary stay for inspection; on success tempDir is left for P1.M3.T2 to clean after the swap.
  Do NOT mirror download.go's "remove partial archive" — that protects callers who might re-extract a lingering tampered archive;
  StageNewBinary never extracts after a verify failure (it errors first), so leaving it is SAFE and matches the PRD. Grep guard 5.
- ❌ Don't touch the running binary. StageNewBinary writes ONLY inside tempDir (the archive + new-stagecoach). It must NOT
  rename/move/chmod any path outside tempDir — the running stagecoach is P1.M3.T2's to swap. This is the FR-U11 invariant: a
  failure here leaves the running binary byte-for-byte unchanged. Grep guard 6.
- ❌ Don't make stage_test.go parallel. The execVersion seam is a PACKAGE-LEVEL var; tests mutate it. If two stage tests ran
  concurrently (t.Parallel) they'd race on execVersion (the race detector would catch it). The sibling releases/detect/delegate/
  resolve tests are unaffected (they never touch execVersion) and may stay parallel — but stage_test.go itself must NOT call
  t.Parallel(). Grep guard 12.
- ❌ Don't break the stubcli env inheritance. The sanity-run's exec.CommandContext(ctx, path, "--version") with cmd.Env UNSET
  inherits os.Environ() (so t.Setenv values reach the stubcli). If a test override sets cmd.Env explicitly (e.g. to inject
  STUBCLI_OUT/EXIT for the wrong-tag/non-zero cases), it MUST prepend os.Environ() — `cmd.Env = append(os.Environ(), "K=V")` —
  or the stubcli gets an empty env and behaves wrong. The DEFAULT execVersion leaves cmd.Env unset (correct).
- ❌ Don't re-wrap the download sentinels. FetchChecksums/DownloadFile/VerifySHA256 already wrap their sentinels (ErrNoChecksumsFile/
  ErrChecksumParse/ErrHTTP/ErrChecksumMissing/ErrChecksumMismatch) with %w and carry diagnostics. StageNewBinary returns them AS-IS
  (just `return "", err`) so the command layer's `errors.Is(err, upgrade.ErrChecksumMismatch)` works. The NEW stage sentinels
  (ErrArchiveNoBinary/ErrSanityVersionMismatch/ErrSanityRunFailed) ARE wrapped at their use site (fmt.Errorf("…: %w", sentinel)).
  Re-wrapping the download sentinels adds nothing + diverges from the package style.
- ❌ Don't write a "// Package upgrade" doc in stage.go. releases.go line 1 owns it; a second one splits the package overview.
  Start stage.go with a FILE comment ("// stage.go implements the FR-U5 download→verify→extract→sanity step…") — exactly as
  resolve.go/detect.go/download.go/delegate.go do.
- ❌ Don't match the archive entry by exact path. goreleaser puts the binary at the archive root, but a tar entry may carry a
  "./" prefix ("./stagecoach") or (rarely) a wrap dir. Match by `filepath.Base(entry) == "stagecoach"`(.exe) — robust to a "./"
  prefix. Matching by exact "stagecoach" would miss "./stagecoach".
- ❌ Don't forget the best-effort chmod. archive/tar preserves mode bits from the header (set Mode 0o755 when packing in the test);
  archive/zip does NOT preserve unix mode. To be robust across both, always `_ = os.Chmod(dest, 0o755)` after extraction (the
  sanity-run needs +x on unix). Don't fail extraction if chmod errors (it won't on a test-owned tempDir).
- ❌ Don't omit the checksums asset from the test's fake Release. FetchChecksums finds the checksums asset BY NAME in release.Assets
  (checksumsName(tag) or *_checksums.txt suffix) and downloads its DownloadURL. The test's fake Release MUST include BOTH the
  archive Asset AND a checksums Asset (DownloadURL serving "<sha>  <assetName>\n"), or FetchChecksums returns ErrNoChecksumsFile
  and the test fails for the wrong reason.
- ❌ Don't run real GitHub API/downloads in tests. CI must be hermetic + not rate-limited. Use an httptest.Server serving the
  packed archive + the checksums body, with a fake Release whose asset DownloadURLs are the absolute httptest URLs. The happy
  path IS a real end-to-end (download→verify→extract→exec) but against the LOCAL httptest server + a REAL compiled stubcli —
  not the real GitHub.
- ❌ Don't add the backup/rename/rollback or the cobra command. Those are P1.M3.T2/P1.M3.T3/P1.M4. StageNewBinary STOPS at the
  verified newBinPath. ZERO production callers after this subtask (the command layer P1.M4.T2 composes it later). Grep guard 10.

---

## Confidence Score: 9/10

The verbatim StageNewBinary/extractBinary/sanityCheck API, the LANDED primitives to compose (FetchChecksums/DownloadFile/
VerifySHA256 — signatures + sentinels verified), the DownloadAndVerifyArchive-vs-asset reconciliation (WHY compose via the asset
— it takes goos/goarch + re-selects + removes-on-failure), the file-name decision (stage.go NOT swap.go — S1 reserved swap.go for
P1.M3.T2; documented), the extract format-from-asset-suffix rule (platform-agnostic, no runtime import) + entry/dest naming +
chmod, the sanity-run semantics + the package-level execVersion seam (+ the no-parallel caveat), the "leave tempDir on failure /
don't clean on success" cleanup contract, the NEW typed errors + the download sentinels to propagate, the cmd/stubcli pattern +
the buildStubCI helper (cloning stubtest.Build) + packArchive + httptest (absolute DownloadURLs), and the 4 contract test cases
(happy/tampered/wrong-tag/non-zero-exit) are all verified against the real code. The -1 from 10/10 reflects three judgment calls
the implementer must honor: (1) the file-name deviation (stage.go, not the contract's swap.go — driven by S1's explicit
reservation); (2) composing the primitives instead of calling DownloadAndVerifyArchive (the contract names it, but it doesn't fit
the (release, asset) input + the leave-for-inspection contract); and (3) the execVersion seam being package-level (so stage_test.go
must not be parallel). All three are spelled out in the Blueprint + Anti-Patterns + grep guards, so an implementer following the
PRP won't fumble them. No new dep, no internal/* import, walled off, file-comment-not-package-doc, no symbol clash with the
parallel resolve.go/delegate.go/detect.go, zero production callers by design.