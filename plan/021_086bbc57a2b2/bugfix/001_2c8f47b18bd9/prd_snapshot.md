# Bug Fix Requirements

## Overview
Rigorous end-to-end PRD validation against a built binary driven through live git repos via the stub-agent harness (capturing rendered prompts), plus the e2e suite and four parallel deep reviews of the heaviest subsystems (lock/watchdog, decompose freeze, self-update, work-description/multi-turn). The freshest feature (+body format, v3.4) and the core commit/CAS/hook machinery are solid: +body grammar validation rejects every malformed form, the scaffold x directive matrix is exactly right across all 8 variants, composition with --locale/--template/--context and subject-based dedupe are correct, +body is honored by all message-producing paths (message/FR-M11 shortcut/arbiter); binary and exclude placeholders render correctly; full git hook execution matches 'git commit' including --no-verify semantics, abort safety, and prepare-commit-msg edit honoring; the FR-SH1 leaked-env stub guard, unborn-repo/unicode/nothing-to-commit, multi-commit freeze boundary, FR-T multi-turn fallback, and the orphan watchdog (parent-pid-change detection) are all correct. Four real bugs cluster in two newer subsystems plus one spec-drift regression: (1) [major] work-description mode turns a READ of a non-staged path into the commit subject (silent garbage commits, FR-W3/W7); (2) [minor] work-description emits an empty body instead of 'end of diff' for a fully-read file (FR-W5); (3) [major] decompose eagerly requires a stager at role resolution, blocking the FR-M13 file-disjoint fast-path for no-tooled providers; (4) [major] 'stagecoach upgrade' production detection runner has no per-query timeout or WaitDelay, so a hung package-manager query hangs indefinitely (FR-U2(b)/section 7). Eight additional minor issues (work-desc chunk/label quality, Linuxbrew detection gap, unescaped repo in release URLs, narrow lock TOCTOU, Windows/non-functional reaping, status-display orphan hint, stale comment). BUG-001 is the most urgent: it silently writes garbage commit subjects. BUG-001, BUG-002, BUG-003, BUG-004 are recommended for immediate fix (bug fixes bringing code into line with existing FRs; no PRP required).


## Critical Issues (Must Fix)
Issues that prevent core functionality from working.

None.


## Major Issues (Should Fix)
Issues that significantly impact user experience or functionality.

### Issue 1: Work-description mode: a READ request for a non-staged path silently becomes the commit subject
**Severity**: Major
**ID**: BUG-001
**Location**: internal/generate/workdesc.go:99

**Description**:
In RunWorkDescription, when the model emits a 'READ <path>' line whose path is NOT in the staged skeleton, parseReadLines filters it out and returns an empty path list. The loop's 'len(paths)==0' branch then treats the ENTIRE raw response as the commit-message candidate via ParseOutput(out, manifest) WITHOUT stripping READ lines. So a response of 'READ typo.go' (or any path the model guesses wrong / that isn't staged) is parsed as the commit message and committed. This violates FR-W3 (non-staged reads must be 'ignored by the caller WITH A NOTE') and mis-applies FR-W7 ('a response with no valid READ line is the message' does not cover a response that IS a READ line for a bad path). The forced-conclusion path correctly calls stripReadLines; the natural path does not. Result: silent garbage commit subjects in the user's repo. Verified end-to-end.

**Steps to Reproduce**:
git init a repo, stage a one-line change to a.txt, then run: stagecoach --provider stub --work-description 'refactor auth' with the agent's first response set to 'READ typo.go'. Resulting commit subject is literally 'READ typo.go'. Any real LLM that typos a path or asks for a file outside the staged skeleton triggers this.

### Issue 2: Decompose FR-M13 violation: eager stager resolution blocks the file-disjoint fast-path for no-tooled providers
**Severity**: Major
**ID**: BUG-003
**Location**: internal/decompose/roles.go:132

**Description**:
ResolveRoles resolves the stager role (with the FR-D4 tooled fallback) for EVERY decompose run at Decompose entry, BEFORE the planner runs and before disjointness is determined. A provider whose manifest declares no tooled_flags (opencode, or any user-defined provider), when no built-in tooled agent (pi/claude) is installed, hard-errors with 'role stager: provider X cannot stage (tooled_flags empty) and no other installed provider is stager-capable' even for a perfectly file-disjoint tree. This contradicts FR-M13 (v3.2), which explicitly states a no-tooled provider 'can now decompose a disjoint tree, where it otherwise could not serve the stager role' via the fast-path that needs no stager. The fast-path is unreachable because role resolution gates it first. FirstTooledProvider only considers built-ins, so a tooled user-defined provider is never used as the fallback either. Verified via a focused unit test that builds a registry with all built-ins bogus and a no-tooled custom provider.

**Steps to Reproduce**:
Unit test: config.Provider='custom' (command=/bin/true, no tooled_flags), registry with all built-ins (pi/opencode/cursor/agy/codex/claude) overridden to a bogus command and a 'custom' provider installed. ResolveRoles(cfg, reg) returns error 'role "stager": provider "custom" cannot stage (tooled_flags empty) and no other installed provider is stager-capable' before any planner call.

### Issue 3: stagecoach upgrade: production detection runner has no per-query timeout or WaitDelay, so a hung package-manager query hangs indefinitely
**Severity**: Major
**ID**: BUG-004
**Location**: internal/cmd/upgrade_run.go:152

**Description**:
cmdRunner.Run is the PRODUCTION runner for the tier-(b) package-manager DB queries (brew/scoop/choco/dpkg/rpm/npm/mise/asdf) used by install-method detection. It has NEITHER the 3s context.WithTimeout NOR cmd.WaitDelay that upgrade.osRunner (detect.go:121) was correctly built with. osRunner is unexported, so package cmd wrote a timeout-less twin. The source comment claims 'the command's ctx already bounds the whole upgrade', but main.go:62 builds the root context via signal.Install(context.Background(), ...) which is signal-cancelable with NO deadline. So a hung PM query (e.g. brew refreshing its DB, a broken PM, an NFS-mounted home) hangs 'stagecoach upgrade' indefinitely, recoverable only via Ctrl-C; and the missing WaitDelay means a forked grandchild still holding the stdout pipe can block past the cancel. FR-U2(b)/external_deps section 7 ('these queries must not hang') is violated.

**Steps to Reproduce**:
Code inspection: internal/cmd/upgrade_run.go cmdRunner.Run uses exec.CommandContext(ctx,...) with no per-call WithTimeout and no cmd.WaitDelay, unlike the timeout+WaitDelay-bearing osRunner at internal/upgrade/detect.go:121 which is unreachable from package cmd (unexported). The ctx originates from signal.Install(context.Background()) in cmd/stagecoach/main.go:62 (cancel-on-signal only, no deadline).


## Minor Issues (Nice to Fix)
Small improvements or polish items.

### Issue 1: Work-description mode: a fully-read file emits an empty body instead of an 'end of diff' note
**Severity**: Minor
**ID**: BUG-002
**Location**: internal/generate/workdesc.go:281

**Description**:
buildReadAnswer calls StagedFileDiff, which returns the FULL diff on every call (cursor-unaware). When the per-file byte offset is exhausted, nextChunk returns ("", 1, 0); that falls into the 'total <= 1' branch which prints '<path>:\n' followed by an empty body. The 'end of diff' / 'not in staged changes' note lives only in the 'diff == ""' branch, which never fires for a staged file (diff is the full non-empty diff). So re-requesting an already-read file delivers an empty section to the model instead of an explicit 'end of diff'. FR-W5 violation. The loop still terminates via the round cap, so no data corruption, but the model receives a confusing empty payload. Verified end-to-end.

**Steps to Reproduce**:
In work-description mode with a small staged file a.txt, have the stub agent request 'READ a.txt' twice. The answer turn for the second request is literally 'a.txt:' with a blank body (no 'end of diff' marker).

### Issue 2: Work-description read chunks anchor to newline boundaries, not @@ hunk edges, so hunks can be split mid-hunk
**Severity**: Minor
**ID**: BUG-005
**Location**: internal/generate/workdesc.go:306

**Description**:
nextChunk advances the per-file cursor by a rune budget then anchors FORWARD to the next newline. The cap can land in the middle of a git hunk, splitting it across two chunks with no 'line-cut' note, which can confuse the model about hunk continuity. FR-W5 asks for clean, comprehensible chunks.

**Steps to Reproduce**:
Code inspection of nextChunk: the forward-anchor uses strings.IndexByte(diff[end:], '\n'), not a 'diff --git'/'@@' hunk boundary, so a chunk boundary can fall between the '@@' header and its lines.

### Issue 3: Work-description 'part i of N' label divides a byte offset by a rune budget, miscounting for multibyte UTF-8
**Severity**: Minor
**ID**: BUG-006
**Location**: internal/generate/workdesc.go:284

**Description**:
The chunk label computes 'part := (st.offsets[p] / chunkRuneBudget()) + 1', but st.offsets[p] is a BYTE offset and chunkRuneBudget() is in runes. For diffs containing multibyte UTF-8 (e.g. non-ASCII comments/strings), the part index drifts from the true chunk number, so 'part i of N' can be wrong.

**Steps to Reproduce**:
Code inspection: st.offsets is advanced by nextChunk's byte 'advance' return, while chunkRuneBudget = readChunkTokenCap*4 runes; the division mixes byte and rune units.

### Issue 4: Upgrade path heuristic omits the Linuxbrew Cellar root, mis-detecting Linuxbrew installs as 'direct'
**Severity**: Minor
**ID**: BUG-007
**Location**: internal/upgrade/detect.go:368

**Description**:
pathHeuristics recognizes '/opt/homebrew/Cellar/' (Apple Silicon) and '/usr/local/Cellar/' (Intel macOS), but not '/home/linuxbrew/.linuxbrew/Cellar/' used by Linuxbrew on Linux. A Linuxbrew install would fall through to the 'direct' channel, so 'stagecoach upgrade' would attempt a self-swap of a Homebrew-owned binary instead of delegating to 'brew upgrade'.

**Steps to Reproduce**:
Code inspection: internal/upgrade/detect.go pathHeuristics table (lines ~368-376) has no /home/linuxbrew/.linuxbrew/Cellar/ entry.

### Issue 5: Upgrade GitHub releases URL interpolates c.Repo unescaped (tag is escaped, repo is not)
**Severity**: Minor
**ID**: BUG-008
**Location**: internal/upgrade/releases.go:179

**Description**:
releases.go builds request paths as '/repos/'+c.Repo+'/releases/latest' and '/repos/'+c.Repo+'/releases' (c.Repo is the 'owner/repo' string) without url.PathEscape, while the tag-based path at line 198 correctly uses url.PathEscape(tag). If Repo ever carried a character requiring escaping the request would be malformed. Low severity / defense-in-depth, since Repo is sourced from config and normally validated.

**Steps to Reproduce**:
Code inspection: compare releases.go:179/223 (c.Repo unescaped) with releases.go:198 (url.PathEscape(tag)).

### Issue 6: Lock stale-file reaper has a narrow TOCTOU window between pid-liveness check and acquire
**Severity**: Minor
**ID**: BUG-009
**Location**: internal/lock/lock.go:128

**Description**:
reapStaleLocks checks whether a recorded pid is a dead process on its hostname, then removes the stale lock file before acquiring. A contender could read the pid, find it dead, and reap a file whose inode is now held by a different live process that reused it. In practice this is bounded by the CAS-backed acquire (flock + write+rename) and the hostname check, so it is not an exploitable force-break of a live lock; flagged as defense-in-depth.

**Steps to Reproduce**:
Code inspection of reapStaleLocks: the liveness check and the subsequent acquire are not atomic; mitigated by flock/CAS.

### Issue 7: Lock stale-file reaping is a no-op on Windows (processAlive always returns true)
**Severity**: Minor
**ID**: BUG-010
**Location**: internal/lock/lock_windows.go:19

**Description**:
On Windows, processAlive(pid, hostname) unconditionally returns true, so stale lock files are never reaped there. This is effectively non-functional rather than harmful: flock is a no-op on Windows (the spec's FR-K7 states the CAS is the guarantee and flock is a no-op), so lock files are not created on the Windows commit path to begin with. Noted for completeness.

**Steps to Reproduce**:
Code inspection: internal/lock/lock_windows.go:19-21 returns true unconditionally.

### Issue 8: 'lock status' orphan hint uses a ppid==1 test that under-reports orphans under subreapers (display only)
**Severity**: Minor
**ID**: BUG-011
**Location**: internal/watchdog/orphan_unix.go:42

**Description**:
The read-only 'stagecoach lock status' orphan hint (appearsOrphaned) keys off ppid==1. Under subreapers (systemd-run, Docker, supervisior) a reparented orphan's ppid is the subreaper, not 1, so the hint reports 'not orphaned' for a genuinely orphaned holder. NOTE: this affects ONLY the informational status hint; the actual self-termination watchdog is correct (arm_unix.go:40 uses parent-pid CHANGE, osGetppid() != originalPpid, exactly per FR-K2) and reaping uses pid-liveness, so no live lock is ever force-broken. Cosmetic/informational.

**Steps to Reproduce**:
Code inspection: the status-display orphan check vs arm_unix.go:40's parent-pid-change detection.

### Issue 9: Stale doc comment claims the arbiter stages from the live working tree (contradicts FR-M1d)
**Severity**: Minor
**ID**: BUG-012
**Location**: internal/decompose/decompose.go:981

**Description**:
A comment near the arbiter phase states the arbiter's staging 'is UNCHANGED — it stages from the working tree', but FR-M1d (v2.2) extended the freeze boundary into the arbiter so gate, diff, AND staging all derive from the frozen T_start via OverlayTreePaths, never the live working tree. The code is correct (resolveArbiter/resolveNewCommit/amendTip use tStart, not AddAll); only the comment is stale and misleading.

**Steps to Reproduce**:
Code inspection: compare the comment at decompose.go:~981 ('stages from the working tree') with resolveArbiter's treePrime := tStart usage.

## Testing Summary
- Total bugs found: 12
- Critical: 0
- Major: 3
- Minor: 9

## Recommendations
- Fix BUG-001 first: in RunWorkDescription's len(paths)==0 branch, detect whether the response actually contained any READ verb line; if it did, do NOT treat it as a message — instead emit FR-W3 notes ('<path> is not in the staged changes') for the non-staged paths and continue the loop (bounded by the round cap), mirroring the forced-conclusion path's use of stripReadLines.
- Fix BUG-002: in buildReadAnswer, distinguish 'cursor exhausted' (nextChunk returns total==1 AND chunk=='') from 'fits in one chunk', and emit the 'end of diff' note for the exhausted case rather than an empty body. Simplest: check st.offsets[p] >= len(diff) before/after nextChunk.
- Fix BUG-003: defer the stager requirement in ResolveRoles (or make it lazy) so a no-tooled provider can reach the FR-M13 disjoint fast-path; only require a stager-capable provider when the partition is NOT file-disjoint (i.e. the tooled loop will actually run). Alternatively, let FirstTooledProvider also consider user-defined tooled providers as a fallback.
- Fix BUG-004: export upgrade.osRunner (or replicate its 3s context.WithTimeout + cmd.WaitDelay) in the production cmdRunner so a hung package-manager query is bounded and recoverable, satisfying FR-U2(b)/section 7.
- Add regression tests: a work-desc scenario where the model READs a non-staged path (asserts it is noted, not committed as subject); a ResolveRoles test with a no-tooled provider and no built-in tooled agent over a disjoint partition; an upgrade test asserting each detection query is bounded by a deadline.
- Address minor issues opportunistically: Linuxbrew Cellar heuristic, repo URL escaping, chunk hunk-edge anchoring, part-label byte/rune units, and correct the stale arbiter comment.
