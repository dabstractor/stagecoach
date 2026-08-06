# Doc Accuracy Audit — README.md / docs/cli.md / docs/configuration.md vs the three fixes

Scope: verify the three user-facing docs (README.md, docs/cli.md, docs/configuration.md) against the
completed fixes P1.M1.T1.S1 (BUG-001 sanity-run v-normalization), P1.M1.T2.S1 (BUG-002 ~/go default
GOPATH fallback), P1.M1.T3.S1 (BUG-003 non-semver tag deprioritization). This is the Mode B changeset-
level doc sweep; Mode A (per-function source doc comments) is already done inside each fix subtask.

## Method
Read the three target docs' upgrade-relevant sections directly, cross-checked against the
implementing PRPs (P1M1T1S1/T2S1/T3S1) and architecture/{system_context,bug_analysis}.md. Verified
README anchors, the Makefile `install` target, and `go env GOPATH`.

## Fix → doc-claim matrix

### BUG-001 (sanity-run v/no-v) — internal logic fix, no CLI/config surface change
- README: no doc describes the sanity-run. The user-facing claim ("self-swaps only for a direct
  install"; exit 0 = upgraded) was BROKEN pre-fix for every real release; post-fix it is TRUE.
  → No drift. The fix makes the README/Updating claim accurate. No edit.
- docs/cli.md: same — "self-swapping only for the direct-binary channel" (L402) is now TRUE.
- docs/configuration.md: no mention. No drift.
- VERDICT: accurate. No statement claims the buggy substring behavior.

### BUG-002 (~/go default GOPATH fallback) — restores documented "go install" delegation
- README Install > Package managers (L112-113): `# Go install (anywhere with Go)` / `go install
  github.com/dabstractor/stagecoach/cmd/stagecoach@latest`. Accurate — installs to ~/go/bin
  (GOPATH unset, the common case). Post-fix, `stagecoach upgrade` detects + delegates. CONSISTENT.
- README Build from source (L132): `make install # installs the binary to $GOPATH/bin` + L135 note
  "Ensure $GOPATH/bin (usually ~/go/bin) is on your $PATH". Accurate. The "usually ~/go/bin" already
  acknowledges the GOPATH-unset default the fix implements. CONSISTENT.
- README Updating (L146): "...delegates to that channel's own updater (..., `go install`),
  ...self-swaps only for a direct (curl|sh / manual) install." This was BROKEN pre-fix (go-install
  users misdetected as `direct`); post-fix it is TRUE.
- docs/cli.md upgrade (L402): "...delegates to that channel's updater (..., go install), self-swapping
  only for the direct-binary channel." Now TRUE post-fix.
- docs/configuration.md: no install-method surface. No drift.
- CANDIDATE CLARIFYING NOTE (the one the contract calls out): note that go-install installs are
  auto-detected via ~/go/bin. See candidate-edit.md for the exact text + placement + decision rubric.

### BUG-003 (non-semver tag deprioritization) — internal selection robustness
- docs/cli.md L412 `--prerelease`: "Admit pre-release tags (shorthand for --channel prerelease)
  (FR-U10)". BUG-003 changes WHICH tag is selected when non-semver tags exist; it does NOT change the
  flag's user-facing behavior of admitting pre-release tags. STILL ACCURATE.
- docs/cli.md L417 `--channel <stable|prerelease>`: accurate.
- docs/configuration.md L122 / L158: "channel ... stable | prerelease (admits -rc/-beta tags)".
  Accurate. BUG-003 doesn't change what the channel admits.
- VERDICT: accurate. The non-semver deprioritization is an internal robustness detail; --prerelease's
  user-facing behavior is unchanged. A note here would be maintainer noise, not user help. NO edit.

## Anchors / cross-references (verified intact)
- README `### Updating` @ L144 resolves the `(#updating)` links @ L87, L392, L422 (GitHub auto-anchor).
- docs/cli.md `### upgrade` @ L400 resolves `docs/cli.md#upgrade` links in README.
- docs/how-it-works.md (OUT OF SCOPE) only references `stagecoach upgrade` as the network exception
  (L197) — consistent with README; no upgrade-internals content; no edit needed there.

## Makefile interaction (context only — NOT a fix target)
`make install` (Makefile L58-61): `go install` to $(GOBIN) THEN `ln -sfn` into ~/.local/bin. So a
make-install user's PATH entry is the ~/.local/bin symlink. This is pre-existing Makefile/README
relationship, NOT introduced by the three fixes, and NOT in BUG-002's scope (BUG-002 is the
GOPATH-unset ~/go fallback, not symlink resolution). Out of scope for this doc sweep.

## go env GOPATH (verified)
`go env GOPATH` = /home/dustin/go even when GOPATH env var is unset — confirms the ~/go default the
BUG-002 fix and the README L135 "usually ~/go/bin" note both rely on.

## Bottom line
All three docs are ACCURATE post-fix — they describe the intended (now-restored) behavior. The fixes
do not change any CLI flag, config key, or API surface. The ONLY candidate edit is the single go-install
~/go/bin auto-detection clarifying note (see candidate-edit.md). A no-edit outcome (record the
verification finding in the task result + commit message) is a fully valid deliverable per the contract.