# P1.M3.T1.S2 Research Findings — StageNewBinary: download+verify+extract+sanity-run (FR-U5 steps 4-6, FR-U11)

> Research for: the download→verify→extract→sanity-run step of the direct-binary self-swap. Consumes S1's
> (Release, Asset) and P1.M1.T3's Client + checksum primitives. Produces a verified, sanity-checked
> new-binary path in a temp dir (NEVER touches the running binary — the swap is P1.M3.T2's job).

---

## §0. STATE OF THE WORLD + FILE-NAME DECISION (swap.go vs stage.go)

- **S1 (P1.M3.T1.S1) is landing `ResolveTarget(ctx, c, opts) (Release, Asset, error)` in `internal/upgrade/resolve.go`.**
  S1's PRP EXPLICITLY reserves `swap.go` for P1.M3.T2.S1 (Backup + atomic swap) and chose `resolve.go` to avoid
  the collision. The parallel_execution_context tells S2 to "not conflict with the previous PRP".
- **This task's contract says "In internal/upgrade/swap.go"** — but that would collide with P1.M3.T2.S1 (Planned),
  which will create/edit swap.go. **DECISION: use `internal/upgrade/stage.go`** (NOT swap.go). Rationale:
  (a) StageNewBinary "stages" the new binary (download+verify+extract+sanity into temp) — semantically DISTINCT
  from the on-disk "swap" (backup+atomic rename) that P1.M3.T2 owns; (b) avoids the file-ownership collision with
  the planned P1.M3.T2 swap.go; (c) mirrors S1's own deviation (resolve.go, not swap.go) for the same reason.
  The contract's "swap.go" is an oversight (it predates S1's swap.go reservation). Documented in the PRP.

## §1. THE DEPENDENCIES (all LANDED — consume, don't rebuild)

### Client / Release / Asset (releases.go)
- `type Asset struct { Name string; DownloadURL string; Size int64 }` (releases.go:44).
- `type Release struct { Tag string; Assets []Asset }` (releases.go:53).
- `type Client struct { HTTP *http.Client; BaseURL string; Repo string; Token string }` (releases.go:61).
  `httpClient()` ⇒ http.DefaultClient when HTTP==nil. **BaseURL is metadata-only for the releases API; asset
  downloads use ABSOLUTE DownloadURLs (NOT prepended).**

### Download/verify primitives (download.go) — StageNewBinary composes these DIRECTLY via the asset
- `func (c *Client) FetchChecksums(ctx, release) (map[string]string, error)` — finds the *_checksums.txt asset
  (by `checksumsName(tag)` or suffix), downloads its DownloadURL, parses "<64hex>  <filename>" lines.
  Sentinels: ErrNoChecksumsFile / ErrChecksumParse / ErrHTTP.
- `func (c *Client) DownloadFile(ctx, url, dest string) error` — streams url→dest (multi-MB safe); non-2xx/transport
  ⇒ ErrHTTP; removes partial dest on copy/close error.
- `func VerifySHA256(path, want string) error` — streams SHA256, constant-time compare; mismatch ⇒ ErrChecksumMismatch.
- `func (c *Client) DownloadAndVerifyArchive(ctx, release, goos, goarch, destDir) (string, error)` — composes
  SelectAsset→FetchChecksums→DownloadFile→VerifySHA256; removes the partial archive on any failure (FR-U11 staging
  analog). **TAKES goos/goarch, re-selects the asset — does NOT fit the (release, asset) input.** See §2.

### Asset/checksum naming (download.go, in-package helpers)
- `func assetName(tag, goos, goarch string) string` — `stagecoach_<tag-no-v>_<os>_<arch>.zip` (windows) / `.tar.gz` (else).
- `func checksumsName(tag string) string` — `stagecoach_<tag-no-v>_checksums.txt`.

## §2. THE DownloadAndVerifyArchive-vs-asset RECONCILIATION (a contract wart — resolved)

The contract says "download+verify to tempDir (DownloadAndVerifyArchive)" but the input is `(release, asset)` and
`DownloadAndVerifyArchive(ctx, release, goos, goarch, destDir)` takes **goos/goarch and re-selects the asset** (via
SelectAsset). Two mismatches: (a) it would need goos/goarch (the asset doesn't carry them cleanly); (b) it
**re-selects** the asset S1 already selected (redundant); (c) it removes the partial archive on failure (conflicts
with this task's "leave tempDir for inspection" contract — §4).

**DECISION: StageNewBinary composes the SAME primitives DownloadAndVerifyArchive composes (FetchChecksums +
DownloadFile + VerifySHA256) using the PRE-SELECTED asset directly** — skipping SelectAsset (redundant, asset is in
hand) and the os.Remove-on-failure (StageNewBinary leaves tempDir for inspection, §4). This honors the contract's
"download+verify" intent + the (release, asset) input, and is fully testable (the asset's DownloadURL points at the
httptest archive). DownloadAndVerifyArchive remains the right primitive for callers who have goos/goarch but no
pre-selected asset (it is NOT changed/removed — StageNewBinary just doesn't use it). Documented in the PRP.

## §3. THE EXTRACT STEP (FR-U5 step 5) — NEW stdlib code (no repo precedent)

`grep archive/tar|archive/zip|compress/gzip` ⇒ ZERO repo usage. So extraction is fresh stdlib code:
- **Format from the asset name suffix** (platform-agnostic — no runtime import needed): `.zip` ⇒ archive/zip;
  else (`.tar.gz`) ⇒ archive/tar + compress/gzip.
- **Entry base name**: `stagecoach` + (`.exe` if `.zip` else `""`) — goreleaser puts the binary at the archive root
  (no wrap dir); match by `filepath.Base(entry) == wantBase` (robust to a `./` prefix in tar).
- **Dest path**: `tempDir/new-stagecoach` + (`.exe` if `.zip` else `""`) — the rename avoids colliding with the
  running `stagecoach`; P1.M3.T2 renames new-stagecoach→stagecoach at swap time.
- **Extract ONLY that one entry** (the contract: "extract ONLY the single binary, not the whole archive"). Write its
  bytes to the dest; **best-effort `os.Chmod(dest, 0o755)`** after (covers unix executability; near-no-op on windows).
- A missing/wrong binary entry ⇒ a typed error (ErrArchiveNoBinary) — never silently produce a non-binary path.

## §4. THE SANITY-RUN (FR-U5 step 6 + FR-U11 abort-before-swap) + the exec seam

- Sanity-run: exec `<newBinPath> --version`, the combined/stdout output MUST contain `release.Tag`, AND exit 0.
  A binary that fails to run (non-zero exit / exec error) OR misreports the tag ⇒ **NEVER swapped** (FR-U11);
  StageNewBinary returns a typed error.
- **Exec seam (the contract's "injectable exec runner")**: a package-level `var execVersion = func(ctx, path)
  ([]byte, error)` defaulting to `exec.CommandContext(ctx, path, "--version").Output()`. `sanityCheck(ctx, path,
  wantTag)` calls it, checks err + bytes.Contains(out, wantTag). Tests OVERRIDE execVersion (see §5).
- **Caveat**: a package-level var is NOT parallel-safe if mutated concurrently. ⇒ stage_test.go's tests must NOT
  call `t.Parallel()` (they mutate execVersion). The other upgrade tests (releases/detect/delegate) are unaffected
  (they never touch execVersion) and may stay parallel. Standard Go testing caveat; documented.

## §5. FAILURE / CLEANUP SEMANTICS (FR-U11 + "leave tempDir for inspection")

The contract: "on failure leave the temp for inspection (PRD) and return an error"; "Cleanup the temp on the success
path is the swap step's job (P1.M3.T2)". So:
- **On ANY failure (verify / extract / sanity): LEAVE tempDir** (no os.RemoveAll; no os.Remove of the archive) —
  the downloaded archive + any extracted binary stay for inspection. StageNewBinary returns a typed error.
- **On SUCCESS: return newBinPath; do NOT clean tempDir** (P1.M3.T2 owns the success-path cleanup after the swap).
- This OVERRIDES download.go's "remove partial archive" for THIS caller: download.go protects callers who might
  accidentally extract a lingering tampered archive; StageNewBinary never extracts after a verify failure (it errors
  first), so leaving the archive for inspection is SAFE and matches the PRD. (Asset-direct composition §2 makes this
  natural — no os.Remove calls at all.)

## §6. THE TEST HARNESS (httptest + compiled stubcli + injectable exec)

### The fake binary: cmd/stubcli (the "cmd/stubcli pattern", env-driven)
`cmd/stubcli/main.go` is a tiny cross-platform binary driven by env: `STAGECOACH_STUBCLI_OUT` (stdout) +
`STAGECOACH_STUBCLI_EXIT` (exit code) + ignores args (so `<stubcli> --version` works). It prints OUT and exits EXIT.
**The sanity-run exec inherits os.Environ()** (exec.CommandContext with cmd.Env unset ⇒ inherits parent), so the test
sets the stub's behavior via `t.Setenv("STAGECOACH_STUBCLI_OUT", ...)` / `t.Setenv("STAGECOACH_STUBCLI_EXIT", ...)`.

### Compile stubcli in tests (clone stubtest.Build's pattern)
`internal/stubtest/stubtest.go:Build` compiles `cmd/stubagent` via `go build -buildvcs=false -o <tmp> <importpath>`
with `build.Dir = moduleRoot()` (CWD-independent). Clone it for cmd/stubcli (stubtest.Build hardcodes cmd/stubagent,
so write a local `buildStubCLI(t)` helper in stage_test.go: `go build -buildvcs=false -o tmp github.com/dabstractor/
stagecoach/cmd/stubcli`, once per process via sync.Once).

### Pack the archive (test helper)
- entryName := "stagecoach" + (".exe" if windows else "") — host-native.
- format: .zip (windows) / .tar.gz (else) — host-native. Pack the stubcli bytes under entryName.
- Compute the archive's SHA256; write a checksums body `"<sha>  <assetName>\n"`.

### httptest + fake Release
- Serve the archive at `<ts.URL>/archive` and the checksums at `<ts.URL>/checksums` (DownloadFile/FetchChecksums use
  ABSOLUTE DownloadURLs).
- fakeRelease := Release{Tag: "v1.2.3", Assets: [{Name: assetName, DownloadURL: ts.URL+"/archive"},
  {Name: checksumsName(tag), DownloadURL: ts.URL+"/checksums"}]}.
- Client: `&Client{Repo: "owner/repo"}` (HTTP nil ⇒ DefaultClient; BaseURL unused for downloads).

### execVersion override (the injectable seam)
- Happy path: override execVersion to run the REAL extracted stubcli with STUBCLI_OUT=release.Tag, EXIT=0
  (proves extraction produced a runnable binary + the tag match end-to-end).
- Wrong tag: override sets STUBCLI_OUT="v9.9.9-wrong" → sanity fails (ErrSanityVersionMismatch).
- Non-zero exit: override sets STUBCLI_EXIT=1 → exec fails → sanity fails (ErrSanityRunFailed).

### Test cases (the contract OUTPUT)
1. **Happy path**: real archive (stubcli) whose --version prints the tag → success (returns tempDir/new-stagecoach[.exe]).
2. **Tampered archive**: checksums.txt lists a WRONG sha → VerifySHA256 fails (ErrChecksumMismatch) → error, NO extract
   (assert tempDir has NO new-stagecoach file).
3. **Wrong-tag binary**: extracted stubcli prints the wrong tag → sanity fails (ErrSanityVersionMismatch) → error.
4. **Non-zero-exit binary**: extracted stubcli exits 1 → sanity fails (ErrSanityRunFailed) → error.
   (3+4 prove "a binary that fails to run or misreports is NEVER swapped" — FR-U11.)

## §7. SCOPE FENCES (what NOT to touch)

- `resolve.go` (S1) — READ-ONLY; StageNewBinary CONSUMES its (Release, Asset) output.
- `swap.go` — DO NOT CREATE (P1.M3.T2.S1 owns it). StageNewBinary lives in `stage.go`.
- releases.go / download.go / version.go / detect.go / delegate.go — READ-ONLY (LANDED primitives; consume).
- The backup + atomic swap + --rollback — P1.M3.T2/P1.M3.T3 (NOT this task). StageNewBinary stops at the verified
  newBinPath; it NEVER renames/moves/chmods the running binary.
- The cobra command / runUpgrade / confirmation prompt / --check — P1.M4 (NOT this task). ZERO production callers of
  StageNewBinary after this subtask (only stage_test.go) — expected (the command layer P1.M4.T2 is the consumer).
- NO internal/* import (FR-U12: the upgrade package imports nothing stagecoach-internal). NO new go.mod require
  (archive/tar, archive/zip, compress/gzip, os/exec are stdlib).

## §8. VALIDATION COMMANDS (project-specific, verified)
- Build: `go build ./...` + cross-build (GOOS=windows/linux/darwin — stage.go is platform-agnostic via asset suffix).
- Vet: `go vet ./internal/upgrade/...`
- Fmt: `gofmt -l internal/upgrade/stage.go internal/upgrade/stage_test.go` (empty).
- Focused: `go test ./internal/upgrade/ -run 'StageNewBinary|SanityCheck|ExtractBinary' -v`
- Full regression: `go test -race ./internal/upgrade/...` (stage_test.go NOT parallel — mutates execVersion; the
  sibling releases/detect/delegate tests stay parallel + green).
- `make test` + `make lint`.
- grep guards (see PRP Level 4).