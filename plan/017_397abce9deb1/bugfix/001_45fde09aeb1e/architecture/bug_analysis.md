# Bug Analysis: Three Upgrade-Path Logic Gaps

All three bugs were validated by reading the current source. Each is confirmed present.

---

## BUG-001 (CRITICAL) — Sanity-run v/no-v version mismatch

### Location
- **Check**: `internal/upgrade/stage.go:65` — `if !bytes.Contains(out, []byte(wantTag))`
- **Call site**: `internal/upgrade/stage.go:239` — `sanityCheck(ctx, newBinPath, release.Tag)`
- **wantTag** = `release.Tag` = `"v1.2.0"` (the git tag, WITH leading `v`)

### Root Cause
goreleaser injects `-X main.version={{.Version}}` where `{{.Version}}` = tag WITHOUT the `v`
(e.g. `1.2.0`). The real `cmd/stagecoach` binary's `--version` outputs `stagecoach version 1.2.0`
(NO `v`). The substring check `bytes.Contains("…1.2.0…", "v1.2.0")` is **FALSE** for every real
v-prefixed release → `ErrSanityVersionMismatch` → `stagecoach upgrade` aborts before swap.

### Why Existing Tests Pass (the mask)
- `internal/upgrade/stage_test.go::TestStageNewBinary_HappyPath` sets
  `t.Setenv("STAGECOACH_STUBCLI_OUT", "v1.2.3")` — the stubcli outputs the tag **WITH** the `v`,
  which does not match how goreleaser builds the real binary.
- `internal/cmd/upgrade_swap_test.go` builds `stubversion` with `-X main.version=v0.2.0` — again
  WITH the `v`. Neither exercises the adversarial no-v case.

### Impact
Breaks `stagecoach upgrade` for the ENTIRE direct-binary channel (curl|sh / manual download) —
the ONLY self-swap-eligible channel (FR-U1/U5). `--check` is unaffected (it uses `Compare` which
normalizes both operands).

### Fix Strategy
In `sanityCheck`, normalize the v-prefix before the substring check: accept the output if it
contains `wantTag` OR `strings.TrimPrefix(wantTag, "v")`. This is safe because semver tags are
distinct (v-stripped `1.2.0` is NOT a substring of `1.20.0`). Keep it a substring check (NOT a
semver compare — per the existing comment, that is the command layer's job). Keep
`ErrSanityVersionMismatch` for genuinely wrong tags.

### Regression Test
Add a test that replicates the real goreleaser binary: drive the stubcli with the **no-v** version
(`STAGECOACH_STUBCLI_OUT=1.2.3` or build `stubversion` with `-X main.version=0.2.0`) against a
release tag that HAS the `v` (`v1.2.3` / `v0.2.0`). Must now succeed (return newBinPath, no error).

---

## BUG-002 (MAJOR) — go-install misdetected as direct when GOPATH unset

### Location
- `internal/upgrade/detect.go:373` — `if gopath := d.envOr("GOPATH", ""); gopath != "" { … }`

### Root Cause
When `GOPATH` is unset (the overwhelmingly common case — modern Go defaults to `~/go`), `envOr`
returns `""`, the `gopath != ""` guard is FALSE, and the entire go-install heuristic is **skipped**.
A binary at `~/go/bin/stagecoach` falls through to `direct`. The code COMMENT claims "default
~/go/bin when GOPATH is unset" but the implementation does NOT implement the fallback.

### Impact
A user who installed via `go install github.com/…/stagecoach@latest` and runs `stagecoach upgrade`
is misrouted to the self-swap path instead of `go install @latest` delegation, contradicting FR-U1
(must not self-swap a channel that should delegate) and FR-U3. Compounded by BUG-001: the self-swap
would then fail the sanity-run, so a go-install user's upgrade fails entirely.

### Fix Strategy
In `detectPath`'s go-install block, when `envOr("GOPATH", "")` is empty, resolve the default
`~/go` via `d.envOr("HOME", "")` + `filepath.Join(home, "go")` (matching `go env GOPATH`). If that
default is non-empty, use it as the gopath for the same `filepath.Join(gopath, "bin")` prefix check.
Honor the HOME-based fallback ONLY when GOPATH is empty. The evidence string can distinguish the
default case (e.g. `"path: ~/go/bin (default GOPATH)"`).

### Regression Test
`Detector{ExePath: filepath.Join(home, "go", "bin", "stagecoach"), Env: func(k) { if k=="HOME" { return home }; return "" }}`
→ `detectPath()` returns `(ChannelGoInstall, …, true)`. Verify with GOPATH unset AND HOME set.
Also: explicit GOPATH still works (existing `TestDetect_Path_GoInstallViaGOPATH` covers this).
Also: both GOPATH and HOME unset → falls through to direct (no false positive).

---

## BUG-003 (MINOR) — Prerelease tag selection picks non-semver tag over valid release

### Location
- `internal/upgrade/releases.go:231` — `if best < 0 || Compare(r.TagName, rs[best].TagName) > 0`
- Interacts with `upgrade.Compare` (`version.go:112`) which returns **0 for unparseable** operands.

### Root Cause
If the releases array contains a non-semver tag (e.g. `"nightly"`, `"latest"`) BEFORE a valid
semver tag, the first non-draft entry sets `best` to the garbage tag. Every later valid tag fails
`Compare(valid, garbage) > 0` (Compare returns 0 because garbage is unparseable), so `best` stays
stuck on the garbage tag. Example: `[{tag:"nightly"},{tag:"v1.5.0"}]` → selected = `"nightly"`.

### Impact (contained)
Downstream `SelectAsset` derives an asset name from `"nightly"` that matches no real asset, so the
upgrade fails safely with `ErrNoMatchingAsset` rather than installing a wrong binary. But the
tag-selection logic is incorrect for the `--prerelease` path when a maintainer publishes any
moving/non-semver tag alongside semver releases.

### Fix Strategy
Modify the selection loop so a non-semver tag can NEVER win over a parseable one:
- Track `best`; for each non-draft candidate compute `okCandidate = ParseAndClean(r.TagName).ok`.
- Advance `best` when: (a) `best < 0`; OR (b) candidate is parseable and best is unparseable; OR
  (c) both parseable and `Compare(candidate, best) > 0`.
- This deprioritizes non-semver tags without erroring (Compare's 0-for-unparseable contract is
  preserved for its original --check dev-build defense purpose).

### Regression Test
httptest serving `[{tag:"nightly",prerelease:true,draft:false,…},{tag:"v1.5.0",prerelease:true,draft:false,…}]`
→ `LatestAdmittingPrereleases` returns `Release{Tag:"v1.5.0"}`. Verify the existing
`TestClient_Prerelease_PicksHighest` still passes (all-parseable tags). Add a case where ALL tags
are non-semver → still returns the first non-draft (graceful, no crash, no ErrNoReleases since
entries exist).

---

## Documentation Surface Check
- `README.md` (Updating section ~L146–154): describes `stagecoach upgrade` detecting install
  method + delegating/self-swapping. Already accurate post-fix — no change needed unless the
  maintainer wants to call out the go-install ~/go default.
- `docs/cli.md` (upgrade command ~L402+): describes the command + flags. No flag/config change
  from these fixes — no change needed.
- `docs/configuration.md` (`[upgrade]` section ~L121+): describes `[upgrade].channel`. No change.
- Source doc-comments on the fixed functions ARE updated per Mode A (within each fix subtask).