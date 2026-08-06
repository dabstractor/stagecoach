# Bug Fix Requirements

## Overview
Validated the v3.0 self-update feature (stagecoach upgrade, FR-U1-U12) end-to-end against the PRD scope. The code is well-structured (stdlib-only internal/upgrade, injectable seams, comprehensive happy/failure/rollback test suites that all pass), but creative end-to-end testing that replicates a REAL goreleaser release (rather than the test's v-prefixed stub binaries) surfaced three gaps the existing tests mask. The headline finding (BUG-001, CRITICAL): the direct-binary self-swap's sanity-run compares the new binary's --version output against release.Tag with a raw substring check, but goreleaser injects the version WITHOUT the leading 'v' while the git tag HAS it — so the substring check always fails and `stagecoach upgrade` aborts for every real release, breaking the only self-swap-eligible channel. This was confirmed by building the actual cmd/stagecoach binary with a goreleaser-style `-X main.version=1.2.0` and running the real StageNewBinary flow against an httptest fake. BUG-002 (MAJOR): `go install` (a documented install method) is misdetected as `direct` when GOPATH is unset (the common case), despite the code comment claiming a ~/go fallback. BUG-003 (MINOR): --prerelease tag selection can pick a leading non-semver tag over a valid release because Compare returns 0 for unparseable operands (fails safe downstream via ErrNoMatchingAsset). All three are logic gaps the existing test stubs happen to align with rather than exercise adversarially.


## Critical Issues (Must Fix)
Issues that prevent core functionality from working.

### Issue 1: Direct-binary upgrade ALWAYS fails the sanity-run against real goreleaser releases (v/no-v version mismatch)
**Severity**: Critical
**ID**: BUG-001
**Location**: internal/upgrade/stage.go:65 (the bytes.Contains check); call site internal/upgrade/stage.go:239 (sanityCheck(ctx, newBinPath, release.Tag)); root cause is the v/no-v mismatch between release.Tag and the goreleaser-built binary's --version output

**Description**:
The headline v3.0 self-update feature is non-functional for the only channel that self-swaps. StageNewBinary's sanity-run (FR-U5 step 6) checks `bytes.Contains(stdout, []byte(release.Tag))`, but release.Tag is the git tag WITH a leading 'v' (e.g. 'v1.2.0') while the REAL cmd/stagecoach binary reports its version WITHOUT the leading 'v' (e.g. 'stagecoach version 1.2.0'). The mismatch exists because goreleaser injects the version via `-X main.version={{.Version}}` and `.Version` is the tag WITHOUT the leading 'v' (.goreleaser.yaml line 35: `-X main.version={{.Version}}`; PRD §21.1). So for every real v-prefixed release, the substring check is FALSE and StageNewBinary returns ErrSanityVersionMismatch, aborting the upgrade BEFORE any swap. This breaks `stagecoach upgrade` for the entire direct-binary channel (curl|sh / manual-download installs) — which is the ONLY self-swap-eligible channel in the delegate-first architecture (FR-U1/U5), i.e. the one path that actually performs a self-swap. The existing tests pass only because they build the stub binary WITH the v prefix (`buildStubVersion(t, "v0.2.0")` in internal/cmd/upgrade_swap_test.go; `t.Setenv("STAGECOACH_STUBCLI_OUT", tag)` with tag="v1.2.3" in internal/upgrade/stage_test.go), which does not match how goreleaser actually builds the real binary. Violates FR-U5 step 6 + FR-U11 (the sanity gate is meant to refuse a binary that MISREPORTS its version, not refuse a correctly-reporting one). NOTE: --check is unaffected because runCheck uses upgrade.Compare (which normalizes both operands), so only the raw-substring sanity-run is broken.

**Steps to Reproduce**:
Reproduced with an end-to-end probe that replicates a real release: (1) build the real binary as goreleaser does: `go build -ldflags "-X main.version=1.2.0" -o /tmp/sc ./cmd/stagecoach`; (2) `/tmp/sc --version` outputs `stagecoach version 1.2.0` (NOT `v1.2.0`); (3) serve a fake GitHub release with tag_name `v1.2.0` + a real archive of that binary + a valid checksums.txt; (4) call upgrade.ResolveTarget + upgrade.StageNewBinary against it. Result: `StageNewBinary FAILED: sanity-run ... output "stagecoach version 1.2.0\n" lacks tag "v1.2.0": upgrade: staged binary --version does not report the target tag`. The check at internal/upgrade/stage.go:65 (`bytes.Contains(out, []byte(wantTag))`) fails because wantTag=`release.Tag`=`v1.2.0` (passed at internal/upgrade/stage.go:239).


## Major Issues (Should Fix)
Issues that significantly impact user experience or functionality.

### Issue 1: `go install` is misdetected as `direct` when GOPATH is unset (the common case)
**Severity**: Major
**ID**: BUG-002
**Location**: internal/upgrade/detect.go:373 (the `if gopath := d.envOr("GOPATH", ""); gopath != ""` guard that skips the go-install heuristic when GOPATH is unset, with no ~/go fallback)

**Description**:
FR-U2 lists `$GOPATH/bin (go install)` as a tier-(c) path heuristic and FR-U3 lists `go install -> go install ...@latest` as a delegation target; `go install` is one of the documented install methods (PRD §21.3). But detect.go's GOPATH check only runs when GOPATH is EXPLICITLY set: `if gopath := d.envOr("GOPATH", ""); gopath != ""`. When GOPATH is unset (the overwhelmingly common case — modern Go defaults GOPATH to ~/go), envOr returns "" and the entire go-install heuristic is skipped, so a binary at `~/go/bin/stagecoach` falls through to `direct`. The code comment at internal/upgrade/detect.go even claims 'default ~/go/bin when GOPATH is unset' but the implementation does NOT implement that fallback. Consequence: a user who installed via `go install github.com/dabstractor/stagecoach/cmd/stagecoach@latest` and runs `stagecoach upgrade` is misrouted to the self-swap path instead of `go install @latest` delegation, contradicting FR-U1 (must not self-swap a channel that should delegate) and FR-U3. (Compounded by BUG-001: the self-swap would then fail the sanity-run, so a go-install user's upgrade fails entirely.) Verified by a probe: Detector with ExePath=$HOME/go/bin/stagecoach and Env returning "" for GOPATH detects `direct (default (ambiguous))`, not `go-install`.

**Steps to Reproduce**:
Construct a Detector{ExePath: "/tmp/fakehome/go/bin/stagecoach", GOOS: "linux", Env: func(k){ if k=="HOME" { return "/tmp/fakehome" }; return "" }} and call Detect. Result: channel=`direct`, evidence=`default (ambiguous)` — NOT `go-install`. With GOPATH explicitly set to "/custom/gopath" and ExePath under it, it correctly returns `go-install`, proving the only gap is the unset-GOPATH default-~/go fallback.


## Minor Issues (Nice to Fix)
Small improvements or polish items.

### Issue 1: --prerelease tag selection picks a leading non-semver tag over a valid semver release
**Severity**: Minor
**ID**: BUG-003
**Location**: internal/upgrade/releases.go:231 (the `Compare(r.TagName, rs[best].TagName) > 0` selection that lets an unparseable leading tag win); interacts with upgrade.Compare's 0-for-unparseable return at internal/upgrade/version.go

**Description**:
LatestAdmittingPrereleases (the --prerelease channel resolver) selects the highest release via `Compare(r.TagName, rs[best].TagName) > 0`. upgrade.Compare deliberately returns 0 for an UNPARSEABLE operand (documented as dev-build defense for --check). In the prerelease-selection loop this is a logic bug: if the releases array contains a non-semver tag (e.g. 'nightly', 'latest', a moving tag) BEFORE a valid semver tag, the first non-draft entry sets best to the garbage tag, and every later valid tag fails the `Compare(valid, garbage) > 0` check (Compare returns 0 because garbage is unparseable), so `best` stays stuck on the garbage tag. Verified by probe: a releases array [{tag:"nightly"},{tag:"v1.5.0"}] yields selected latest = "nightly" instead of "v1.5.0". Downstream impact is contained (SelectAsset then derives an asset name from "nightly" that matches no real asset, so the upgrade fails safely with ErrNoMatchingAsset rather than installing a wrong binary), but the tag-selection logic is incorrect for the --prerelease path when a maintainer publishes any moving/non-semver tag alongside semver releases.

**Steps to Reproduce**:
httptest server serving a /repos/{o}/{r}/releases array body of [{"tag_name":"nightly","prerelease":true,"draft":false,"assets":[]},{"tag_name":"v1.5.0","prerelease":true,"draft":false,"assets":[{"name":"x"}]}]; call Client.LatestAdmittingPrereleases(ctx). Returns Release{Tag:"nightly"} — the garbage tag wins because Compare("v1.5.0","nightly") returns 0 (nightly unparseable), never satisfying the `> 0` update condition at internal/upgrade/releases.go:231.

## Testing Summary
- Total bugs found: 3
- Critical: 1
- Major: 1
- Minor: 1

## Recommendations
- BUG-001 fix: normalize before the substring check — e.g. compare both the tag and its v-stripped form against the binary output, or strip the leading 'v' from release.Tag before `bytes.Contains` (sanityCheck could accept the tag and also try strings.TrimPrefix(tag, "v")). Add a regression test that builds the REAL cmd/stagecoach binary with a goreleaser-style no-v version against a v-prefixed release tag (the probe in this hunt can be promoted to a permanent test).
- BUG-002 fix: in detect.go's detectPath, when GOPATH is unset resolve the default ~/go (HOME/go) like `go env GOPATH` does, so a binary at ~/go/bin is detected as go-install. Honor a HOME-based fallback only when GOPATH is empty.
- BUG-003 fix: in LatestAdmittingPrereleases, treat Compare==0 against the current best (or an unparseable best) as non-winning for the best — e.g. only advance best when the candidate is parseable and strictly greater, and prefer a parseable tag over an unparseable one. Or skip unparseable tags entirely in selection.
