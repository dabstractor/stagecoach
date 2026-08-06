# Research Notes — P1.M1.T1.S1 (normalize sanityCheck tag comparison + goreleaser regression test)

Verification against the CURRENT working tree (2026-07-XX). The task description and
`architecture/bug_analysis.md` BUG-001 are accurate. These notes record the exact verified code/test
state for a one-pass fix.

## VERIFIED — the bug (internal/upgrade/stage.go:60-69 sanityCheck)
```go
func sanityCheck(ctx context.Context, path, wantTag string) error {
	out, err := execVersion(ctx, path)
	if err != nil {
		return fmt.Errorf("sanity-run %s: %w", path, ErrSanityRunFailed)
	}
	if !bytes.Contains(out, []byte(wantTag)) {                       // ← LINE 65: the broken check
		return fmt.Errorf("sanity-run %s: output %q lacks tag %q: %w", path, out, wantTag, ErrSanityVersionMismatch)
	}
	return nil
}
```
- Call site: stage.go:239 `sanityCheck(ctx, newBinPath, release.Tag)` — release.Tag = "v1.2.0" (WITH v).
- goreleaser `.goreleaser.yaml` injects `-X main.version={{.Version}}` where `{{.Version}}` is the tag
  WITHOUT the v → real `--version` outputs `stagecoach version 1.2.0` (NO v).
- `bytes.Contains("…1.2.0…", "v1.2.0")` is FALSE for EVERY real v-prefixed release →
  ErrSanityVersionMismatch → `stagecoach upgrade` aborts before swap. Breaks the only self-swap-eligible
  channel (FR-U1/U5). `--check` is unaffected (runCheck uses upgrade.Compare, which normalizes).

## VERIFIED — the fix (one line, no new import)
`strings` is ALREADY imported in stage.go (line 83 uses `strings.HasSuffix`), so `strings.TrimPrefix`
needs no import addition. Change the single guard at line 65:
```go
// accept the tag OR its v-stripped form (goreleaser injects the version without the leading 'v')
if !bytes.Contains(out, []byte(wantTag)) && !bytes.Contains(out, []byte(strings.TrimPrefix(wantTag, "v"))) {
	return fmt.Errorf("sanity-run %s: output %q lacks tag %q: %w", path, out, wantTag, ErrSanityVersionMismatch)
}
```
- SAFE: distinct semver tags do not substring-collide (v-stripped "1.2.0" is NOT a substring of
  "1.20.0"). Kept a substring check (NOT a semver compare — per the existing comment, that is the
  command layer's job; runCheck already uses Compare which normalizes).
- ROBUST to both tag forms: if a future tag lacks the v (wantTag="1.2.3"), TrimPrefix is a no-op, so the
  check reduces to the original bytes.Contains(out, "1.2.3") — works for v and no-v output.
- ErrSanityVersionMismatch is KEPT for genuinely-wrong tags (the WrongTag test still fails correctly).

## VERIFIED — error sentinels unchanged (stage.go:36-43)
```go
ErrSanityVersionMismatch = errors.New("upgrade: staged binary --version does not report the target tag")
ErrSanityRunFailed       = errors.New("upgrade: staged binary failed to run")
```
Signature `sanityCheck(ctx, path, wantTag)` is UNCHANGED. The exec/non-zero-exit path (ErrSanityRunFailed)
is UNCHANGED.

## VERIFIED — existing tests (stage_test.go) and how they relate to the fix
- **TestStageNewBinary_HappyPath** (line 201): `tag := "v1.2.3"`; `t.Setenv("STAGECOACH_STUBCLI_OUT", tag)`
  → stubcli outputs "v1.2.3" (WITH v). Currently passes (bytes.Contains matches the v-tag). AFTER FIX:
  still passes (the wantTag-with-v branch is the FIRST operand — true). NO change needed.
- **TestStageNewBinary_WrongTag** (line 281): `tag := "v1.2.3"` (wantTag="v1.2.3"); overrides execVersion
  with STUBCLI_OUT="v9.9.9-wrong". Currently fails with ErrSanityVersionMismatch. AFTER FIX: STILL fails
  correctly — bytes.Contains("…9.9.9-wrong…", "v1.2.3")=FALSE AND bytes.Contains("…9.9.9-wrong…", "1.2.3")
  (v-stripped wantTag)=FALSE → both branches false → ErrSanityVersionMismatch. NO change needed.
- **TestStageNewBinary_NonZeroExit** (line 321): tests ErrSanityRunFailed — UNAFFECTED by the fix.

So the fix preserves all existing stage_test.go tests unchanged.

## VERIFIED — the NEW regression test (the deliverable)
Mirror TestStageNewBinary_HappyPath but drive the stubcli with the NO-v version, against a v-prefixed
release tag. Use the DEFAULT exec + t.Setenv (simplest — same idiom as HappyPath):
```go
func TestStageNewBinary_RealGoreleaserNoVPrefix(t *testing.T) {
	stub := buildStubCLI(t)
	tag := "v1.2.3"            // release tag WITH v (real git tag) — wantTag
	assetNm := hostAssetName(tag)
	archive, sha := packArchive(t, stub, hostEntryName(), assetNm)
	checksumsBody := fmt.Sprintf("%s  %s\n", sha, assetNm)
	ts := archiveServer(t, archive, checksumsBody)
	defer ts.Close()
	rel, c := fakeRelease(tag, assetNm, ts.URL)
	tempDir := t.TempDir()
	// Replicate the REAL goreleaser-built binary: -X main.version={{.Version}} injects the version
	// WITHOUT the leading 'v', so --version reports "1.2.3" (no v) while release.Tag is "v1.2.3".
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
```
This test FAILS before the fix (bytes.Contains("1.2.3","v1.2.3")=FALSE) and PASSES after (v-stripped
branch). It is the authoritative regression guard. Place it next to TestStageNewBinary_HappyPath.

## VERIFIED — cmd-level (internal/cmd/upgrade_swap_test.go)
- `buildStubVersion(t, version)` (line 112) compiles cmd/stubversion with `-X main.version=<version>`.
- **TestUpgradeSwap_DirectHappyPath** (line 372): `buildStubVersion(t, "v0.2.0")` (WITH v) + asserts
  installed --version `strings.Contains(got, "v0.2.0")`. AFTER FIX: still passes (v-match branch). The
  task's HARD requirement for cmd-level is "at minimum verify the existing cmd-level happy path still
  passes post-fix" — satisfied unchanged.
- The existing cmd happy path MASKS the bug (uses v0.2.0 WITH v). The task conditionally suggests a
  no-v cmd variant. NOTE the assertion caveat: a no-v cmd test (buildStubVersion(t, "0.2.0")) would make
  the installed --version output "0.2.0" (no v), so its assertion must use Contains(got, "0.2.0") (NOT
  "v0.2.0"). The stage_test.go regression test above is the cleaner authoritative guard; a cmd no-v
  variant is OPTIONAL (recommended) — see PRP Task 5.

## VERIFIED — doc comments to update (Mode A)
- sanityCheck doc comment (stage.go:56-59): add the v-normalization rationale.
- StageNewBinary doc comment (stage.go:193, "the output contains release.Tag (a substring check …)"):
  add that the substring accepts the tag OR its v-stripped form.
- No user-facing CLI/config/API change — the fix restores intended FR-U5 step 6 / FR-U11 behavior.

## SCOPE BOUNDARIES (sibling subtasks — do NOT implement here)
- **P1.M1.T2.S1** (BUG-002): go-install ~/go GOPATH fallback in detect.go. Different file, different bug.
- **P1.M1.T3.S1** (BUG-003): prerelease tag selection semver-safety in releases.go. Different file.
- **P1.M1.T4.S1**: README/docs accuracy sweep. No docs change for S1 beyond the two code doc comments.
- Do NOT: change the sanityCheck signature; convert to a semver Compare (the comment explicitly says the
  command layer owns that); touch runCheck/Compare/release.Tag generation; or alter the error sentinels.