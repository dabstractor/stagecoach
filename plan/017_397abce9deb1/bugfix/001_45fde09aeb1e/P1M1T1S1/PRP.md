name: "P1.M1.T1.S1 — Normalize the tag comparison in sanityCheck + add a goreleaser-style regression test (BUG-001)"
description: >
  Fix the Critical v/no-v version mismatch in the direct-binary upgrade sanity-run (BUG-001, FR-U5
  step 6 / FR-U11). sanityCheck does a raw substring check `bytes.Contains(out, wantTag)` but
  release.Tag carries the leading 'v' (e.g. "v1.2.0") while goreleaser-built binaries report the
  version WITHOUT it (e.g. "stagecoach version 1.2.0"). Normalize: accept the tag OR its v-stripped
  form. One-line guard change + doc-comment updates + a regression test that replicates the real
  goreleaser no-v output against a v-prefixed release tag.

---

## Goal

**Feature Goal**: Make `stagecoach upgrade` succeed for real goreleaser releases on the direct-binary
(self-swap) channel by fixing the sanity-run's raw substring check so it tolerates the v/no-v
mismatch between release.Tag (git tag, WITH v) and the binary's `--version` output (goreleaser
`-X main.version={{.Version}}`, WITHOUT v).

**Deliverable**: (1) a one-line normalization in `internal/upgrade/stage.go::sanityCheck` — accept the
output if it contains `wantTag` OR `strings.TrimPrefix(wantTag, "v")`; (2) updated doc comments on
`sanityCheck` and `StageNewBinary`; (3) a new regression test `TestStageNewBinary_RealGoreleaserNoVPrefix`
in `internal/upgrade/stage_test.go` that drives the stubcli with a no-v version against a v-prefixed
release tag and asserts `StageNewBinary` succeeds. No signature change, no error-sentinel change, no
semver-compare conversion.

**Success Definition**:
- `sanityCheck` returns nil when the binary outputs the version WITHOUT the leading 'v' (e.g. "1.2.3")
  against wantTag="v1.2.3".
- `sanityCheck` still returns `ErrSanityVersionMismatch` for a genuinely-wrong tag
  (`TestStageNewBinary_WrongTag` still passes), and `ErrSanityRunFailed` for exec/non-zero-exit
  (`TestStageNewBinary_NonZeroExit` still passes).
- The new regression test FAILS before the fix and PASSES after it.
- The existing `TestStageNewBinary_HappyPath` (v WITH output) still passes unchanged.
- `go build ./...`, `go test ./internal/upgrade/...`, `go test ./internal/cmd/...` (cmd happy path),
  `make test`, `make lint` all pass.

## Why

- **BUG-001 (Critical)**: The headline v3.0 self-update feature is non-functional for the ONLY channel
  that self-swaps. For every real v-prefixed release, the substring check is FALSE and `stagecoach
  upgrade` aborts with `ErrSanityVersionMismatch` BEFORE any swap. PRD §9.29 FR-U5 step 6 + FR-U11: the
  sanity gate is meant to refuse a binary that MISREPORTS its version, not refuse a correctly-reporting
  one. The existing tests mask the bug because their stub binaries bake/output the version WITH the v,
  which does not match how goreleaser actually builds the real binary.
- **Root cause** (no action beyond the fix): `.goreleaser.yaml` injects `-X main.version={{.Version}}`
  where `{{.Version}}` is the tag WITHOUT the leading 'v' (PRD §21.1); the git tag (`release.Tag`)
  carries the 'v'. The sanity-run's raw substring check bridged neither direction.

## What

**User-visible behavior**: `stagecoach upgrade` (direct-binary channel) no longer aborts on a
correctly-reporting goreleaser release; the sanity-run accepts the binary's no-v `--version` output.
`--check` was already unaffected (it uses `upgrade.Compare`, which normalizes both operands).

**Technical change (small, surgical):**
1. `sanityCheck` (stage.go:65): change the guard to accept `wantTag` OR its v-stripped form.
2. Doc comments: document the v-normalization on `sanityCheck` (stage.go:56-59) and `StageNewBinary`
   (stage.go:193).
3. New regression test in stage_test.go replicating the real goreleaser no-v output.

### Success Criteria
- [ ] `sanityCheck` accepts no-v output against a v-prefixed wantTag
- [ ] `ErrSanityVersionMismatch` still returned for genuinely-wrong tags (WrongTag passes)
- [ ] `ErrSanityRunFailed` still returned for exec/non-zero-exit (NonZeroExit passes)
- [ ] New `TestStageNewBinary_RealGoreleaserNoVPrefix` test added (fails before fix, passes after)
- [ ] `TestStageNewBinary_HappyPath` unchanged and passing
- [ ] `go build ./...`, `make test`, `make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact buggy line, the exact one-line fix (with the no-new-import confirmation), the exact test patterns to mirror, the WrongTag/NonZeroExit/HappyPath preservation analysis, and the scope boundaries are all enumerated below (verified by reading source).

### Documentation & References

```yaml
- file: internal/upgrade/stage.go
  why: "THE change site. sanityCheck (lines 56-69); the broken guard at line 65; call site at 239 (sanityCheck(ctx, newBinPath, release.Tag)); error sentinels at 36-43; the execVersion seam at ~51."
  pattern: "sanityCheck is a raw substring check (NOT a semver compare — the existing comment says the command layer owns that; runCheck uses upgrade.Compare which normalizes). Keep it a substring check; just normalize the v-prefix."
  gotcha: "strings is ALREADY imported in stage.go (line 83 uses strings.HasSuffix) — strings.TrimPrefix needs NO new import. Do NOT change the sanityCheck signature. Do NOT convert to a semver Compare."

- file: internal/upgrade/stage_test.go
  why: "The test patterns. TestStageNewBinary_HappyPath (201) = DEFAULT exec + t.Setenv(STUBCLI_OUT, tag). TestStageNewBinary_WrongTag (281) = overrides execVersion, STUBCLI_OUT='v9.9.9-wrong'. buildStubCLI/packArchive/archiveServer/fakeRelease/hostAssetName/hostEntryName/exeSuffix are the helpers."
  pattern: "Mirror HappyPath EXACTLY but set STUBCLI_OUT to the NO-v version ('1.2.3') while release.Tag stays 'v1.2.3'. Use the DEFAULT exec (just t.Setenv) — simplest. Assert StageNewBinary returns (newBinPath, nil)."
  gotcha: "WrongTag (line 281) and NonZeroExit (321) MUST still pass unchanged — they do (WrongTag's '9.9.9-wrong' contains neither 'v1.2.3' nor v-stripped '1.2.3'; NonZeroExit hits the exec-error path before the tag check). Do NOT run these in parallel (the file comment explains the shared execVersion seam)."

- file: internal/upgrade/version.go (upgrade.Compare)
  why: "Confirm runCheck/--check ALREADY normalizes both operands via Compare, so --check is unaffected by this bug and needs NO change. Only the raw-substring sanityCheck was broken."

- file: internal/cmd/upgrade_swap_test.go
  why: "The cmd-level self-swap happy path. buildStubVersion (112) bakes -X main.version=<version>. TestUpgradeSwap_DirectHappyPath (372) uses buildStubVersion(t, 'v0.2.0') WITH v + asserts installed --version Contains 'v0.2.0'."
  gotcha: "The cmd happy path MASKS the bug (uses v WITH). The task's HARD requirement is 'verify the existing cmd happy path still passes post-fix' (it does — v-match branch). A no-v cmd variant is OPTIONAL/recommended (see PRP Task 5): if added, buildStubVersion(t,'0.2.0') makes installed --version output '0.2.0', so its assertion must use Contains(got,'0.2.0') NOT 'v0.2.0'."

- docfile: plan/017_397abce9deb1/bugfix/001_45fde09aeb1e/architecture/bug_analysis.md
  why: "The full BUG-001 root-cause + the existing-test-masks analysis + the recommended fix (matches this PRP)."
  section: "BUG-001 (CRITICAL) — Sanity-run v/no-v version mismatch"

- docfile: plan/017_397abce9deb1/bugfix/001_45fde09aeb1e/P1M1T1S1/research/verification_deltas.md
  why: "The exact verified code/test state, the no-new-import confirmation, the WrongTag/NonZeroExit preservation analysis, and the exact new-test body. READ THIS before editing."
```

### Current Codebase tree (relevant slice)

```bash
internal/upgrade/
  stage.go          # sanityCheck (56-69, guard at 65); StageNewBinary (210, call at 239); sentinels (36-43); execVersion seam (~51)
  stage_test.go     # HappyPath(201) / WrongTag(281) / NonZeroExit(321) + NEW RealGoreleaserNoVPrefix test
  version.go        # upgrade.Compare (normalizes — used by runCheck/--check, UNAFFECTED here)
  releases.go       # release.Tag source (git tag, WITH v) — UNCHANGED
internal/cmd/upgrade_swap_test.go  # buildStubVersion(112); DirectHappyPath(372) — verify still passes post-fix (optional no-v variant)
.goreleaser.yaml    # -X main.version={{.Version}} (no v) — the root-cause context (no change)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (no new import): strings is ALREADY imported in stage.go (line 83 strings.HasSuffix).
//   Use strings.TrimPrefix directly — do NOT add an import (would be an unused/duplicate import error).

// CRITICAL (keep it a substring check): do NOT convert sanityCheck to a semver Compare. The existing
//   comment explicitly says the semver compare is the COMMAND layer's job (runCheck uses Compare which
//   normalizes). The fix is purely normalizing the v-prefix on the raw substring.

// CRITICAL (signature unchanged): do NOT change sanityCheck(ctx, path, wantTag). The wantTag is
//   release.Tag (always v-prefixed in this project). The fix accepts wantTag OR TrimPrefix(wantTag,"v").

// GOTCHA (substring safety): v-stripped "1.2.0" is NOT a substring of "1.20.0" (the ".0" vs ".20" boundary
//   differs), so distinct semver tags do not collide. The normalization is safe.

// GOTCHA (robust to both tag forms): if a future release tag lacks the v (wantTag="1.2.3"), TrimPrefix is
//   a no-op, so the check reduces to the original bytes.Contains(out,"1.2.3") — works for v and no-v output.

// GOTCHA (test parallelism): the stage_test.go file comment says do NOT add t.Parallel() — the execVersion
//   seam is shared state. The new test follows the same convention (no t.Parallel()).

// GOTCHA (WrongTag/NonZeroExit preservation): the fix MUST NOT weaken these. WrongTag's "v9.9.9-wrong"
//   output contains neither "v1.2.3" nor "1.2.3" → still ErrSanityVersionMismatch. NonZeroExit hits the
//   exec-error path (line 63) BEFORE the tag check → still ErrSanityRunFailed. Both pass unchanged.
```

## Implementation Blueprint

### Data models and structure

No struct/type changes. One guard expression change + doc-comment updates + one new test. The
sanityCheck signature, error sentinels, and execVersion seam are unchanged.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/upgrade/stage.go — normalize the sanityCheck tag guard (line 65)
  - REPLACE the single guard:
        if !bytes.Contains(out, []byte(wantTag)) {
            return fmt.Errorf("sanity-run %s: output %q lacks tag %q: %w", path, out, wantTag, ErrSanityVersionMismatch)
        }
    WITH:
        // Accept the tag OR its v-stripped form: goreleaser injects the version via -X main.version={{.Version}}
        // WITHOUT the leading 'v', while release.Tag carries it. FR-U5 step 6 / FR-U11.
        if !bytes.Contains(out, []byte(wantTag)) && !bytes.Contains(out, []byte(strings.TrimPrefix(wantTag, "v"))) {
            return fmt.Errorf("sanity-run %s: output %q lacks tag %q: %w", path, out, wantTag, ErrSanityVersionMismatch)
        }
  - CONFIRM strings is already imported (line 83) — no import edit.
  - DO NOT touch: the exec-error path (line 63, ErrSanityRunFailed), the signature, the sentinels.
  - DEPENDENCIES: none.

Task 2: MODIFY internal/upgrade/stage.go — update doc comments (Mode A)
  - sanityCheck doc comment (lines 56-59): append the v-normalization rationale, e.g.
    "Accepts wantTag OR its v-stripped form (strings.TrimPrefix(wantTag, \"v\")) because goreleaser
     injects the version WITHOUT the leading 'v' (-X main.version={{.Version}}) while release.Tag
     carries it (PRD §21.1). Distinct semver tags do not substring-collide."
  - StageNewBinary doc comment (line ~193, "the output contains release.Tag (a substring check …)"):
    note the substring accepts the tag OR its v-stripped form (same rationale).
  - DEPENDENCIES: Task 1.

Task 3: CREATE the regression test in internal/upgrade/stage_test.go
  - ADD TestStageNewBinary_RealGoreleaserNoVPrefix next to TestStageNewBinary_HappyPath (mirror it).
  - BODY (use the DEFAULT exec + t.Setenv idiom, same helpers as HappyPath):
        func TestStageNewBinary_RealGoreleaserNoVPrefix(t *testing.T) {
            stub := buildStubCLI(t)
            tag := "v1.2.3" // release tag WITH v (real git tag) — wantTag
            assetNm := hostAssetName(tag)
            archive, sha := packArchive(t, stub, hostEntryName(), assetNm)
            checksumsBody := fmt.Sprintf("%s  %s\n", sha, assetNm)
            ts := archiveServer(t, archive, checksumsBody)
            defer ts.Close()
            rel, c := fakeRelease(tag, assetNm, ts.URL)
            tempDir := t.TempDir()
            // Replicate the REAL goreleaser-built binary: -X main.version={{.Version}} injects the
            // version WITHOUT the leading 'v', so --version reports "1.2.3" while release.Tag is "v1.2.3".
            t.Setenv("STAGECOACH_STUBCLI_OUT", "1.2.3")
            newBinPath, err := StageNewBinary(context.Background(), c, rel, rel.Assets[0], tempDir)
            if err != nil {
                t.Fatalf("StageNewBinary no-v goreleaser output (BUG-001 regression): unexpected error: %v", err)
            }
            want := filepath.Join(tempDir, "new-stagecoach"+exeSuffix())
            if newBinPath != want {
                t.Errorf("newBinPath = %q; want %q", newBinPath, want)
            }
        }
  - DO NOT add t.Parallel() (shared execVersion seam — see file comment).
  - This test FAILS before the Task-1 fix (bytes.Contains("1.2.3","v1.2.3")=FALSE) and PASSES after.
  - DEPENDENCIES: Task 1.

Task 4: VERIFY existing tests still pass (no edits — confirmation)
  - TestStageNewBinary_HappyPath (v WITH output) — v-match branch (first operand) → still passes.
  - TestStageNewBinary_WrongTag ("v9.9.9-wrong") — neither "v1.2.3" nor "1.2.3" substring → still ErrSanityVersionMismatch.
  - TestStageNewBinary_NonZeroExit — exec-error path before tag check → still ErrSanityRunFailed.
  - DEPENDENCIES: Tasks 1-3.

Task 5 (recommended, optional): no-v cmd-level variant in internal/cmd/upgrade_swap_test.go
  - The existing TestUpgradeSwap_DirectHappyPath (buildStubVersion(t,"v0.2.0") WITH v) masks the bug.
  - OPTIONALLY add a sibling that builds the NEW stub WITHOUT the v (buildStubVersion(t,"0.2.0")) against
    a v0.2.0 release tag, and assert the swap succeeds. CRITICAL assertion caveat: the installed exe's
    --version then outputs "0.2.0" (no v), so assert strings.Contains(got,"0.2.0"), NOT "v0.2.0".
  - The HARD requirement for cmd-level is only "the existing cmd happy path still passes post-fix"
    (Task 4 + the cmd test run) — this no-v variant is recommended, not required.
  - DEPENDENCIES: Task 1.
```

### Implementation Patterns & Key Details

```go
// PATTERN: the normalized substring guard (stage.go:65) — accept tag OR v-stripped form
if !bytes.Contains(out, []byte(wantTag)) && !bytes.Contains(out, []byte(strings.TrimPrefix(wantTag, "v"))) {
    return fmt.Errorf("sanity-run %s: output %q lacks tag %q: %w", path, out, wantTag, ErrSanityVersionMismatch)
}
// wantTag = release.Tag = "v1.2.0" (git tag, WITH v); goreleaser binary outputs "1.2.0" (NO v).
// TrimPrefix("v1.2.0","v") = "1.2.0" → second bytes.Contains matches the real binary output.

// PATTERN: the regression test mirrors HappyPath but flips the stub output to no-v
//   tag := "v1.2.3"          // release tag WITH v (wantTag)
//   t.Setenv("STAGECOACH_STUBCLI_OUT", "1.2.3")   // NO v — replicates goreleaser's real --version
//   → StageNewBinary must SUCCEED (was ErrSanityVersionMismatch before the fix)
```

### Integration Points

```yaml
NO struct / signature / sentinel / public-API changes. One guard expression + doc comments + one test.

CODE:
  - internal/upgrade/stage.go:65 — guard normalized (tag OR v-stripped)
  - internal/upgrade/stage.go:56-59 + ~193 — doc comments updated

TESTS:
  - internal/upgrade/stage_test.go — +TestStageNewBinary_RealGoreleaserNoVPrefix (new)
  - internal/cmd/upgrade_swap_test.go — existing DirectHappyPath verified (optional no-v variant)

UNCHANGED: sanityCheck signature; ErrSanityVersionMismatch/ErrSanityRunFailed; execVersion seam;
  runCheck/upgrade.Compare (--check already normalized); release.Tag generation; .goreleaser.yaml.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...
go vet ./...
gofmt -l internal/upgrade/
# Expected: empty (strings already imported — no import drift).
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The upgrade package — the fix + new regression test + preserved WrongTag/NonZeroExit/HappyPath
go test ./internal/upgrade/... -run 'StageNewBinary' -v
# Expected: HappyPath PASS, RealGoreleaserNoVPrefix PASS (new), WrongTag PASS (ErrSanityVersionMismatch),
#           NonZeroExit PASS (ErrSanityRunFailed), TamperedArchive PASS.

# Full upgrade package
go test ./internal/upgrade/... -v

# Cmd-level self-swap happy path (verify it still passes post-fix)
go test ./internal/cmd/... -run 'UpgradeSwap' -v
# Expected: DirectHappyPath PASS (v-match branch).

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# (Optional) Replicate the original BUG-001 reproduction end-to-end, now expecting SUCCESS:
#   1. Build the real binary as goreleaser does (NO v):
go build -ldflags "-X main.version=1.2.0" -o /tmp/sc-goreleaser ./cmd/stagecoach
/tmp/sc-goreleaser --version
# Expected: "stagecoach version 1.2.0" (NO v).
#   2. The sanity-run against a v1.2.0 release tag now accepts this output (the unit test in Task 3
#      proves it deterministically without needing a live GitHub fake; this manual step is optional).
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: the fix uses strings.TrimPrefix (v-normalization present)
grep -n "TrimPrefix(wantTag" internal/upgrade/stage.go
# Expected: 1 hit (the normalized guard).

# Grep guard: the sanityCheck signature is unchanged
grep -n "func sanityCheck(ctx context.Context, path, wantTag string) error" internal/upgrade/stage.go
# Expected: 1 hit (unchanged).

# Grep guard: no semver.Compare introduced into sanityCheck (the comment's contract preserved)
grep -n "Compare" internal/upgrade/stage.go
# Expected: empty (sanityCheck stays a substring check; Compare lives in runCheck/version.go).

# Grep guard: the new regression test exists and uses the no-v output
grep -n "TestStageNewBinary_RealGoreleaserNoVPrefix" internal/upgrade/stage_test.go
grep -n 'STAGECOACH_STUBCLI_OUT", "1.2.3"' internal/upgrade/stage_test.go
# Expected: the test name + the no-v OUT both present.

# Scope-boundary guard: detect.go (BUG-002) and releases.go (BUG-003) untouched
git diff --name-only
# Expected: only internal/upgrade/stage.go + internal/upgrade/stage_test.go (+ optionally cmd/upgrade_swap_test.go).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `gofmt -l internal/upgrade/` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) all pass

### Feature Validation
- [ ] `sanityCheck` accepts no-v output against a v-prefixed wantTag (new test passes)
- [ ] `ErrSanityVersionMismatch` still returned for genuinely-wrong tags (WrongTag passes)
- [ ] `ErrSanityRunFailed` still returned for exec/non-zero-exit (NonZeroExit passes)
- [ ] `TestStageNewBinary_HappyPath` (v WITH output) passes unchanged
- [ ] New `TestStageNewBinary_RealGoreleaserNoVPrefix` fails before the fix, passes after

### Scope-Boundary Validation
- [ ] sanityCheck signature UNCHANGED
- [ ] NO semver Compare added to sanityCheck (substring check preserved)
- [ ] NO change to runCheck/upgrade.Compare, release.Tag, .goreleaser.yaml, or the error sentinels
- [ ] detect.go (BUG-002) and releases.go (BUG-003) UNTOUCHED (sibling subtasks)
- [ ] No docs change beyond the two code doc comments (README/docs sweep is P1.M1.T4.S1)

### Code Quality
- [ ] strings.TrimPrefix uses the already-imported `strings` package (no new import)
- [ ] Doc comments document the v-normalization rationale (FR-U5 step 6 / FR-U11, PRD §21.1)
- [ ] New test follows the file's no-t.Parallel() convention (shared execVersion seam)

---

## Anti-Patterns to Avoid

- ❌ Don't convert sanityCheck to a semver `Compare` — the existing comment explicitly assigns the semver compare to the command layer (runCheck uses Compare, which normalizes). The fix normalizes the v-prefix on the raw substring only.
- ❌ Don't change the `sanityCheck(ctx, path, wantTag)` signature or the error sentinels — the fix is internal to the guard expression.
- ❌ Don't add a `strings` import — it's already imported (line 83). Adding it again is a duplicate-import compile error.
- ❌ Don't weaken `TestStageNewBinary_WrongTag` or `NonZeroExit` — verify they still pass (WrongTag's "9.9.9-wrong" matches neither "v1.2.3" nor "1.2.3"; NonZeroExit hits the exec path before the tag check).
- ❌ Don't touch `runCheck`/`upgrade.Compare`/`release.Tag`/`.goreleaser.yaml` — `--check` was never broken (Compare normalizes), and release.Tag legitimately carries the 'v'.
- ❌ Don't add `t.Parallel()` to the new test — the file comment forbids it (shared execVersion seam).
- ❌ Don't edit detect.go (BUG-002) or releases.go (BUG-003) — those are sibling subtasks T2/T3.
- ❌ Don't forget the cmd-level no-v assertion caveat IF you add the optional Task-5 variant: buildStubVersion(t,"0.2.0") makes the installed --version output "0.2.0", so assert Contains(got,"0.2.0") not "v0.2.0".

---

## Confidence Score: 9/10

One-pass success is very high: the fix is a single guard expression with a no-new-import confirmation
(`strings` already imported), the test to mirror is identified with its exact helpers, and the
preservation of WrongTag/NonZeroExit/HappyPath is analyzed. The -1 is for the optional cmd-level no-v
variant (Task 5), whose assertion caveat (Contains "0.2.0" not "v0.2.0") could trip an implementer who
copies the existing cmd happy path's assertion verbatim — mitigated by making the stage_test.go
regression test the authoritative deliverable and marking the cmd variant optional with the caveat
called out.