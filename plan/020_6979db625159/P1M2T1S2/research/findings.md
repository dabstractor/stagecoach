# Research Findings — P1.M2.T1.S2 (release.yml: delete winget job + WINGET_TOKEN doc, add CHOCOLATEY_API_KEY)

## 0. Task shape (one sentence)
Pure **Mode-A config-doc** edit to ONE file, `.github/workflows/release.yml` (235 lines): (a) DELETE the
entire `winget:` job (comment banner + job definition); (b) DELETE the `WINGET_TOKEN` header-secret doc;
(c) ADD a `CHOCOLATEY_API_KEY` header-secret doc block; (d) ADD `CHOCOLATEY_API_KEY` to the `goreleaser`
job's env block. NO Go code, NO tests, NO other file.

## 1. The 4 edit regions (verified live; LOCATE BY CONTENT — line numbers drift)

### (b) WINGET_TOKEN header doc — DELETE (lines 13–14)
Exact text (in the `# Required repo SECRETS` comment block at the top of the file):
```
#   WINGET_TOKEN                classic PAT, public_repo scope — forks microsoft/winget-pkgs to
#                              dabstractor/winget-pkgs + opens the manifest PR. Settings → Secrets → Actions.
```
Locate via `grep -n 'WINGET_TOKEN' .github/workflows/release.yml`. The line immediately AFTER it is
`# (GITHUB_TOKEN is auto-provided — no secret needed for the GitHub Release itself.)` — preserve it.

### (c) CHOCOLATEY_API_KEY header doc — ADD (replace the deleted WINGET_TOKEN slot)
The header secret block documents each secret as `#   SECRET_NAME<pad>description…` (3-space indent,
description column-aligned, multi-line where needed — see the NPM_TOKEN 4-line block). Add a
CHOCOLATEY_API_KEY block in the SAME slot the WINGET_TOKEN block occupied (Chocolatey REPLACES WinGet as
the Windows package-manager channel — PRD §21.2/G27). Match the NPM_TOKEN multi-line style:
```
#   CHOCOLATEY_API_KEY          Chocolatey community-source push key (chocolatey.org → Account Settings
#                              → API Key). Used by the goreleaser-native `chocolateys:` pipe (in
#                              .goreleaser.yaml) to publish the .nupkg to push.chocolatey.org.
#                              Settings → Secrets → Actions.
```
(Net header change: −2 lines WINGET_TOKEN, +4 lines CHOCOLATEY_API_KEY.)

### (a) winget: job — DELETE ENTIRELY (comment banner lines 109–114 + job 115–139)
The region runs from the `# --- WinGet (Windows Package Manager) manifest automation …` banner through
the job's final line `          token: ${{ secrets.WINGET_TOKEN }}   # classic PAT …`. It is preceded by
a blank line (line 108) and followed by a blank line then the asdf-mirror banner (`# --- asdf / mise
plugin mirror …`). DELETE the banner + job block AND one adjacent blank line so no double-blank remains.
Locate the start via `grep -n '# --- WinGet' .github/workflows/release.yml` and the end via
`grep -n 'secrets.WINGET_TOKEN' .github/workflows/release.yml` (the token line is the job's last line).

### (d) CHOCOLATEY_API_KEY in the goreleaser job env — ADD
The `goreleaser:` job (starts line 27) HAS an env block (lines ~54–58), under the `Run GoReleaser` step:
```
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}
          SCOOP_BUCKET_GITHUB_TOKEN: ${{ secrets.SCOOP_BUCKET_GITHUB_TOKEN }}
          AUR_SSH_PRIVATE_KEY: ${{ secrets.AUR_SSH_PRIVATE_KEY }}
```
ADD after the `AUR_SSH_PRIVATE_KEY` line:
```
          CHOCOLATEY_API_KEY: ${{ secrets.CHOCOLATEY_API_KEY }}
```
Locate via `grep -n 'AUR_SSH_PRIVATE_KEY: \${{ secrets.AUR_SSH_PRIVATE_KEY }}' .github/workflows/release.yml`.
WHY here (not a new job): the goreleaser-native `chocolateys:` pipe (added by P1.M2.T1.S1 in
.goreleaser.yaml) runs WITHIN the goreleaser job and reads `{{ .Env.CHOCOLATEY_API_KEY }}`. No other job
(npm-publish / asdf-mirror / apt-dnf-repo) needs it.

## 2. ⭐ Deletion-safety proof (deleting winget breaks nothing)
The winget job is a LEAF with no dependents:
- It declares `needs: goreleaser` (line 117) — it runs AFTER goreleaser; nothing runs after IT.
- The three sibling jobs — `npm-publish` (line 66, `needs: goreleaser`), `asdf-mirror` (line 150,
  `needs: goreleaser`), `apt-dnf-repo` (line 195, `needs: goreleaser`) — ALL depend on `goreleaser`, NOT
  on `winget`. (Verified: `grep -n '^    needs:'` shows only `needs: goreleaser`.)
- The winget job itself was already non-blocking: `continue-on-error: true` + `if: ${{ !cancelled() }}`.
→ Deleting the whole `winget:` block cannot break any `needs:` edge. The DAG stays valid (goreleaser →
{npm-publish, asdf-mirror, apt-dnf-repo}).

## 3. Coordination with the parallel S1 (P1.M2.T1.S1) — CONTRACT
S1 edits `.goreleaser.yaml` ONLY. It adds the `chocolateys:` pipe whose `api_key` is
`'{{ .Env.CHOCOLATEY_API_KEY }}'`. That `.Env.CHOCOLATEY_API_KEY` reference is SATISFIED by THIS task's
edit (d) — wiring `CHOCOLATEY_API_KEY: ${{ secrets.CHOCOLATEY_API_KEY }}` into the goreleaser job env.
So S1 + S2 together close the loop: goreleaser reads the env var → the workflow injects the secret.
- S1 does NOT touch release.yml (this file is MY exclusive domain).
- S1's success gate is `rg -ni winget .goreleaser.yaml` = 0 (in the OTHER file).
- This task's success gate is `rg -ni winget .github/workflows/release.yml` = 0 (THIS file).

## 4. Validation tools available (verified)
- `yq` at `/usr/bin/yq` → `yq e '.' .github/workflows/release.yml` (YAML parse gate).
- `python3` + pyyaml OK → `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"`.
- `actionlint` is NOT installed → no deep workflow-semantic lint. Rely on YAML parse + grep gates + the
  DAG-safety proof (§2). (The edits are pure deletion of a leaf job + a doc/secret line swap, so a YAML
  parse + the grep success gate is sufficient.)
- `make test` / `make lint` (Go) are unaffected by a workflow edit — run only to confirm a clean tree.

## 5. The mechanical success gate (unambiguous)
```
rg -ni winget .github/workflows/release.yml    # MUST return ZERO hits (no WINGET_TOKEN, no winget job, no comment banner)
```
Currently 17 hits (header doc 13–14, banner 109–114, job 115–139). After the edit: 0.
Secondary gates: `yq e '.' …` parses; the `goreleaser` env block contains CHOCOLATEY_API_KEY; the header
contains a CHOCOLATEY_API_KEY doc block; exactly 4 jobs remain (goreleaser/npm-publish/asdf-mirror/apt-dnf-repo).

## 6. Scope fences (what NOT to do)
- NO `.goreleaser.yaml` edit (S1's domain; S1 adds the chocolateys: pipe).
- NO Go code / tests (detect.go/delegate.go are the parallel P1.M1 milestone, already Complete).
- NO install.ps1 (P1.M2.T1.S3), NO docs/packaging.md or docs/cli.md or README.md (P1.M3.T1/S2 — a later
  Mode-A docs task; do NOT touch docs here).
- NO PRD.md / tasks.json / prd_snapshot.md (read-only).
- The ONLY file touched: `.github/workflows/release.yml`.
- Do NOT add a winget-releaser-style external PR job for Chocolatey — the native `chocolateys:` pipe
  publishes directly within the goreleaser job (no separate job, no PR, no microsoft repo).