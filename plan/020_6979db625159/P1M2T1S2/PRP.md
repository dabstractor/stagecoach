name: "P1.M2.T1.S2 — release.yml: delete the winget job + WINGET_TOKEN doc, add CHOCOLATEY_API_KEY (Chocolatey replaces WinGet, PRD §21.2/G27)"
description: >
  Pure Mode-A config-doc edit to ONE file, `.github/workflows/release.yml` (235 lines). Four changes:
  (a) DELETE the entire `winget:` job (comment banner lines ~109-114 + job definition ~115-139); (b)
  DELETE the `WINGET_TOKEN` header-secret doc (lines ~13-14); (c) ADD a `CHOCOLATEY_API_KEY`
  header-secret doc block in the same slot; (d) ADD `CHOCOLATEY_API_KEY` to the `goreleaser` job's env
  block (where the goreleaser-native `chocolateys:` pipe reads `{{ .Env.CHOCOLATEY_API_KEY }}` — that
  pipe is added by the parallel P1.M2.T1.S1 in .goreleaser.yaml). Deleting winget is SAFE: it is a leaf
  job (`needs: goreleaser`; `continue-on-error: true`; the 3 siblings all need `goreleaser`, not winget).
  The mechanical success gate is `rg -ni winget .github/workflows/release.yml` = ZERO hits. NO Go code,
  NO tests, NO .goreleaser.yaml (S1), NO install.ps1 (S3), NO docs (P1.M3), NO winget-releaser-style
  Chocolatey job (the native pipe publishes directly).

---

## Goal

**Feature Goal**: Remove the WinGet release channel from `.github/workflows/release.yml` entirely (the
`winget:` job + the `WINGET_TOKEN` secret doc) and wire in its Chocolatey replacement secret
(`CHOCOLATEY_API_KEY`) — both the header doc and the goreleaser job env that the goreleaser-native
`chocolateys:` pipe (added by P1.M2.T1.S1) consumes. PRD §21.2/G27: Chocolatey replaces WinGet because
microsoft/winget-pkgs runs a Microsoft Defender install-gate that hard-blocks the unsigned binary every
release, and Chocolatey is goreleaser-native (no separate CI job, no PR, no Defender gate).

**Deliverable**: ONE modified file, `.github/workflows/release.yml`:
1. The `winget:` job + its comment banner — DELETED.
2. The `WINGET_TOKEN` header-secret doc — DELETED.
3. A `CHOCOLATEY_API_KEY` header-secret doc block — ADDED (in the WINGET_TOKEN slot).
4. `CHOCOLATEY_API_KEY: \${{ secrets.CHOCOLATEY_API_KEY }}` — ADDED to the goreleaser job env block.

**Success Definition**:
- `rg -ni winget .github/workflows/release.yml` → **ZERO hits**.
- The YAML parses: `yq e '.' .github/workflows/release.yml` succeeds.
- Exactly 4 jobs remain (goreleaser, npm-publish, asdf-mirror, apt-dnf-repo) — no `winget:` job.
- The `goreleaser` job env block contains `CHOCOLATEY_API_KEY: \${{ secrets.CHOCOLATEY_API_KEY }}`.
- The header secret block contains a `CHOCOLATEY_API_KEY` doc line and NO `WINGET_TOKEN` doc line.
- No other file changed; `make test` stays green (a workflow edit cannot break Go tests).

## User Persona (if applicable)

**Target User**: The release engineer who tags `v*` to cut a release, and the Windows developer who
installs via `choco install stagecoach`.

**Use Case**: Maintainer tags `v1.2.3` → release.yml's goreleaser job runs → the native `chocolateys:`
pipe (S1) builds + pushes the `.nupkg` to push.chocolatey.org using the `CHOCOLATEY_API_KEY` secret this
task wires into the goreleaser env → Windows users `choco install stagecoach` immediately. The winget job
(noisy, Defender-gate-blocked, leaf) is gone.

**User Journey**: before — tagging triggers a `winget:` job that forks microsoft/winget-pkgs and opens a
PR that Defender hard-blocks; the `WINGET_TOKEN` secret is documented but the channel never ships. After
— no winget job; the goreleaser job's `chocolateys:` pipe publishes directly to chocolatey.org via
`CHOCOLATEY_API_KEY`.

**Pain Points Addressed**: the per-release Microsoft Defender install-gate tax on WinGet (PRD §21.2); the
stale `WINGET_TOKEN` doc that references a dead channel.

## Why

- **PRD §21.2/G27**: Chocolatey is the established Windows package manager AND goreleaser-native — so it
  needs NO separate CI job (unlike WinGet's `winget-releaser` Action) and NO microsoft/winget-pkgs PR
  (no Defender gate). This task drops the WinGet CI job and wires the Chocolatey secret.
- **Loop closure with S1**: S1 adds the `chocolateys:` pipe in `.goreleaser.yaml` whose `api_key` is
  `'{{ .Env.CHOCOLATEY_API_KEY }}'`. That `.Env.CHOCOLATEY_API_KEY` is satisfied ONLY by this task's env
  wiring. S1 + S2 together make the publish work.
- **Honesty/cleanup**: the `WINGET_TOKEN` header doc references a channel being dropped; leaving it
  misleads the next contributor. Removing it (and the job) keeps the workflow honest.
- **Bounded scope**: one workflow file, four localized edits. The Go-side detection/delegation
  (ChannelChocolatey) is the already-Complete P1.M1 milestone; the goreleaser pipe is the parallel S1;
  install.ps1 is S3; the docs sync is P1.M3.

## What

**User-visible behavior**: None directly (release plumbing). Effect: on the next tag, no `winget:` job
runs; the goreleaser job publishes a Chocolatey `.nupkg` via `CHOCOLATEY_API_KEY`.

**Technical change** (one file, four edits; verbatim content in the Blueprint):

### Edit (b) + (c) — header secrets section: DELETE WINGET_TOKEN doc, ADD CHOCOLATEY_API_KEY doc
DELETE the 2-line WINGET_TOKEN doc; ADD a 4-line CHOCOLATEY_API_KEY doc in the same slot (matching the
NPM_TOKEN multi-line style). See Blueprint for exact text.

### Edit (d) — goreleaser job env: ADD CHOCOLATEY_API_KEY
Append `CHOCOLATEY_API_KEY: \${{ secrets.CHOCOLATEY_API_KEY }}` after the `AUR_SSH_PRIVATE_KEY` line in
the goreleaser job's `env:` block.

### Edit (a) — DELETE the entire winget: job (banner + definition)
Delete the comment banner (`# --- WinGet …` block) + the `winget:` job (through its final `token: \${{
secrets.WINGET_TOKEN }}` line) + one adjacent blank line so no double-blank remains.

### Success Criteria
- [ ] `rg -ni winget .github/workflows/release.yml` → 0 hits
- [ ] `yq e '.' .github/workflows/release.yml` parses (valid YAML)
- [ ] exactly 4 jobs remain (goreleaser, npm-publish, asdf-mirror, apt-dnf-repo) — no `winget:`
- [ ] goreleaser job env contains `CHOCOLATEY_API_KEY: \${{ secrets.CHOCOLATEY_API_KEY }}`
- [ ] header secret block has a `CHOCOLATEY_API_KEY` doc line and no `WINGET_TOKEN` doc line
- [ ] only `.github/workflows/release.yml` changed

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact text of all 4 edit regions (verified live), the deletion-safety proof (winget is a
leaf job with no dependents), the CHOCOLATEY_API_KEY placement rationale (the chocolateys: pipe runs in
the goreleaser job), the header-doc style to match (NPM_TOKEN multi-line), the validation tooling (yq +
python3 available; actionlint NOT installed), the S1 coordination fact (S1 adds the `.Env.CHOCOLATEY_API_KEY`
reference this task satisfies), and the explicit scope fences.

### Documentation & References

```yaml
# MUST READ — the authoritative research (exact edit regions + deletion-safety proof + tooling + scope)
- docfile: plan/020_6979db625159/P1M2T1S2/research/findings.md
  why: "§1 = the 4 edit regions with exact text + 'locate by content' grep anchors. §2 = the
        deletion-safety proof (winget is a leaf; the 3 siblings all need goreleaser, NOT winget — so
        deleting it cannot break any needs: edge). §3 = the S1 coordination (the chocolateys: pipe's
        {{ .Env.CHOCOLATEY_API_KEY }} is satisfied by this task's env wiring). §4 = validation tools
        (yq + python3 present; actionlint absent). §5 = the mechanical success gate."
  critical: "§2 is the load-bearing safety fact: grep -n '^    needs:' shows only 'needs: goreleaser'
             across all jobs — no job needs winget. Deleting it is DAG-safe. §1's line numbers (13-14,
             54-58, 109-139) are advisory — locate each region by content (the grep anchors are given)."

# MUST READ — the parallel S1 PRP (the chocolateys: pipe whose env var THIS task wires; CONTRACT)
- docfile: plan/020_6979db625159/P1M2T1S1/PRP.md
  why: "S1 edits .goreleaser.yaml ONLY, adding the native `chocolateys:` pipe with
        `api_key: '{{ .Env.CHOCOLATEY_API_KEY }}'`. That env reference is what edit (d) here satisfies.
        Confirms S1 does NOT touch release.yml (no file overlap)."
  critical: "Treat S1 as a CONTRACT — assume its chocolateys: pipe lands exactly as specified. This task
             must add the matching CHOCOLATEY_API_KEY env so the publish loop closes. Do NOT edit
             .goreleaser.yaml (S1's exclusive domain)."

# MUST READ — the file being edited (locate edit points by content, not line number)
- file: .github/workflows/release.yml
  why: "235 lines. The header secret block is lines 1-16 ('# Required repo SECRETS'); the WINGET_TOKEN
        doc is lines 13-14. The goreleaser job starts line 27; its env block (under 'Run GoReleaser') is
        lines ~54-58 (GITHUB_TOKEN/HOMEBREW_TAP_GITHUB_TOKEN/SCOOP_BUCKET_GITHUB_TOKEN/AUR_SSH_PRIVATE_KEY).
        The winget job is the comment banner (109-114) + definition (115-139). LOCATE each via the grep
        anchors in findings §1."
  pattern: "Header secrets are documented as '#   SECRET_NAME<pad>description…' (3-space indent, aligned
            description column, multi-line where needed — cf. NPM_TOKEN's 4-line block). Job env vars are
            '          SECRET: \${{ secrets.SECRET }}' (10-space indent). Jobs are '  jobname:' (2-space)."
  gotcha: "Line numbers drift on any sibling edit. Locate: `grep -n WINGET_TOKEN` (header + job token),
           `grep -n '# --- WinGet'` (job banner start), `grep -n 'secrets.WINGET_TOKEN'` (job end),
           `grep -n 'AUR_SSH_PRIVATE_KEY'` (env insertion point)."

# MUST READ — the release-plumbing findings (the winget job + WINGET_TOKEN doc + safety confirmed)
- docfile: plan/020_6979db625159/architecture/release_plumbing.md
  why: "The '.github/workflows/release.yml' section confirms the winget job lines (109-139), the
        WINGET_TOKEN header doc (13-14), and 'Safe to remove: the winget job uses continue-on-error:true
        + if:!cancelled(); no other job depends on it (npm-publish, asdf-mirror, apt-dnf-repo all use
        needs: goreleaser, not winget)'. Also confirms CHOCOLATEY_API_KEY goes in the goreleaser job env."
  critical: "Its header note 'also document it in the header secrets section' is exactly edit (c)."

# CONTEXT — the PRD spec (Chocolatey replaces WinGet; §21.2/G27)
- docfile: plan/020_6979db625159/prd_snapshot.md
  why: "§21.2 (h3.103) documents the Chocolatey-over-WinGet decision (the microsoft/winget-pkgs
        validationDefender install-gate hard-blocks the unsigned binary every release; Chocolatey does
        not) and that Chocolatey is a goreleaser-native chocolateys: pipe to the community repo.
        §21.3 (h3.104) shows 'choco install stagecoach' in the install paths. G27 lists Chocolatey as
        the established Windows PM complementing Scoop. Cite these in the CHOCOLATEY_API_KEY doc."
  section: "§21.2 goreleaser (Chocolatey bullet); §21.3 Install paths; G27"

# CONTEXT — CI validation procedure (how to confirm the workflow change is sound)
- file: docs/ci-validation.md
  why: "The repo's AGENTS.md mandates CI validation after a change. This is a workflow-file edit, so the
        'trigger a run on the current branch' step applies — but the winget job deletion only manifests
        on a 'v*' tag-triggered release run, not a CI run. The local gates (rg winget=0, yq parse, make
        test) are the deterministic proof; a full release-run confirmation is the maintainer's tag-time
        check (out of scope for this subtask)."
  critical: "Do NOT tag to validate. The local gates are sufficient for this leaf-job deletion + secret swap."
```

### Current Codebase tree (relevant slice)

```bash
.github/workflows/
  release.yml           # EDIT — delete winget job + WINGET_TOKEN doc; add CHOCOLATEY_API_KEY (header doc + goreleaser env)
  ci.yml                # READ-ONLY — build/test/lint/coverage CI (unrelated to the tag-triggered release)
.goreleaser.yaml        # READ-ONLY / NOT this task — parallel S1 adds the chocolateys: pipe (the consumer of CHOCOLATEY_API_KEY)
install.ps1             # READ-ONLY / NOT this task — P1.M2.T1.S3 (does not exist yet)
docs/packaging.md       # READ-ONLY / NOT this task — P1.M3.T1 (WinGet→Chocolatey section rewrite)
internal/upgrade/       # READ-ONLY / NOT this task — parallel P1.M1 (detect.go/delegate.go ChannelChocolatey)
Makefile                # READ-ONLY — make test/make lint (Go only; workflows not linted by make)
```

### Desired Codebase tree with files to be added/modified

```bash
# MODIFIED (no new files):
.github/workflows/release.yml   # -winget job -WINGET_TOKEN doc +CHOCOLATEY_API_KEY doc +CHOCOLATEY_API_KEY goreleaser env
```

### Known Gotchas of our codebase & Library Quirks

```yaml
# CRITICAL (locate edit points by CONTENT, not line number): line numbers (13-14, 54-58, 109-139) are
#   advisory — any sibling edit (or a re-read after a drift) shifts them. Anchors:
#     grep -n 'WINGET_TOKEN'              → header doc (13-14) + job token (139)
#     grep -n '# --- WinGet'             → job banner start (109)
#     grep -n 'secrets.WINGET_TOKEN'     → job end (139)
#     grep -n 'AUR_SSH_PRIVATE_KEY'      → goreleaser env insertion point

# CRITICAL (deletion is DAG-safe — but verify, don't assume): the winget job is a LEAF. Confirm with
#   `grep -n '^    needs:' .github/workflows/release.yml` — every job shows `needs: goreleaser`; NONE
#   shows `needs: winget`. If a future edit added `needs: winget` somewhere, that dependency must be
#   removed first. (Currently there is none — verified.)

# CRITICAL (the CHOCOLATEY_API_KEY env goes in the GORELEASER job, not a new job): the goreleaser-native
#   `chocolateys:` pipe runs INSIDE the goreleaser job and reads `{{ .Env.CHOCOLATEY_API_KEY }}` — so the
#   env var must be in that job's env block. Do NOT add a separate winget-releaser-style Chocolatey job
#   (the native pipe publishes directly; no PR, no microsoft repo). Do NOT add it to npm-publish/asdf/apt-dnf.

# GOTCHA (header-doc style — match NPM_TOKEN's multi-line format): the CHOCOLATEY_API_KEY doc is longer
#   than one line, so use the same multi-line style as NPM_TOKEN (a '#   SECRET<pad>first line' header +
#   '#<pad>continuation' lines). Keep the 3-space indent + aligned description column. Do NOT misalign.

# GOTCHA (blank-line hygiene on the job deletion): the winget block is preceded AND followed by a blank
#   line. Delete the banner + job AND one adjacent blank line so the result has a single blank separator
#   between the npm-publish block and the asdf-mirror banner (no double-blank, no missing separator).

# GOTCHA (actionlint is NOT installed): there is no deep workflow linter available locally. The gates are
#   `yq e '.' …` (YAML parse) + `rg -ni winget …` = 0 (the mechanical success gate) + the DAG-safety
#   proof (leaf job, no dependents). This is sufficient for a pure deletion + secret swap.

# GOTCHA (do NOT add a CHOCOLATEY job): Chocolatey is goreleaser-native — the `chocolateys:` pipe (S1)
#   runs within goreleaser and publishes directly to push.chocolatey.org. There is NO external-PR job to
#   add (unlike WinGet's vedantmgoyal9/winget-releaser fork-PR pattern). Adding one would duplicate the pipe.
```

## Implementation Blueprint

### Data models and structure
None. Pure YAML workflow file. No code, no types.

### Implementation Tasks (ordered — header edits first, then env, then job deletion; all in one file)

```yaml
Task 1: EDIT .github/workflows/release.yml header — replace WINGET_TOKEN doc with CHOCOLATEY_API_KEY doc
  - LOCATE: grep -n 'WINGET_TOKEN' .github/workflows/release.yml → the header doc is the FIRST hit
    (lines ~13-14), the 2-line block:
        #   WINGET_TOKEN                classic PAT, public_repo scope — forks microsoft/winget-pkgs to
        #                              dabstractor/winget-pkgs + opens the manifest PR. Settings → Secrets → Actions.
  - REPLACE those 2 lines with the CHOCOLATEY_API_KEY 4-line block (match NPM_TOKEN's multi-line style):
        #   CHOCOLATEY_API_KEY          Chocolatey community-source push key (chocolatey.org → Account Settings
        #                              → API Key). Used by the goreleaser-native `chocolateys:` pipe (in
        #                              .goreleaser.yaml) to publish the .nupkg to push.chocolatey.org.
        #                              Settings → Secrets → Actions.
  - PRESERVE the line after it: '# (GITHUB_TOKEN is auto-provided — no secret needed for the GitHub Release itself.)'
  - PRESERVE every other header secret doc (HOMEBREW_TAP_GITHUB_TOKEN, SCOOP_BUCKET_GITHUB_TOKEN, ASDF_TOKEN,
    AUR_SSH_PRIVATE_KEY, NPM_TOKEN multi-line block).

Task 2: EDIT .github/workflows/release.yml — add CHOCOLATEY_API_KEY to the goreleaser job env block
  - LOCATE: grep -n 'AUR_SSH_PRIVATE_KEY: ' .github/workflows/release.yml → the goreleaser job's env
    block (under the 'Run GoReleaser' step). The block is:
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}
          SCOOP_BUCKET_GITHUB_TOKEN: ${{ secrets.SCOOP_BUCKET_GITHUB_TOKEN }}
          AUR_SSH_PRIVATE_KEY: ${{ secrets.AUR_SSH_PRIVATE_KEY }}
  - INSERT, immediately AFTER the AUR_SSH_PRIVATE_KEY line (same 10-space indent):
          CHOCOLATEY_API_KEY: ${{ secrets.CHOCOLATEY_API_KEY }}
  - PRESERVE the existing 4 env vars and the env: block structure. Do NOT touch any other job's env.

Task 3: EDIT .github/workflows/release.yml — DELETE the entire winget: job (banner + definition)
  - LOCATE the start: grep -n '# --- WinGet' .github/workflows/release.yml → the comment banner
    (6 comment lines: '# --- WinGet (Windows Package Manager) manifest automation …' + the
    vedantmgoyal9/winget-releaser/Komac/bootstrap-gate explanation).
  - LOCATE the end: grep -n 'secrets.WINGET_TOKEN' → the job's FINAL line:
        '          token: ${{ secrets.WINGET_TOKEN }}   # classic PAT, public_repo scope (NOT the default GITHUB_TOKEN)'
  - DELETE the contiguous region from the banner's first line through that final token line, PLUS one
    adjacent blank line (the block is bracketed by blank lines before/after — delete ONE so a single
    blank separator remains between the npm-publish job and the asdf-mirror banner).
  - The deleted region includes: the 6-line comment banner, the `winget:` job header
    (`  winget:`, `name:`, `needs: goreleaser`, `if: \${{ !cancelled() }}`, `runs-on: windows-latest`),
    the single `steps:` step (`- name: Open winget-pkgs manifest PR via WinGet Releaser`, `continue-on-error: true`,
    `uses: vedantmgoyal9/winget-releaser@v2`, the `with:` block: identifier/installers-regex/fork-user/
    max-versions-to-keep/token).
  - PRESERVE the asdf-mirror banner (`# --- asdf / mise plugin mirror …`) and job that follow.
  - VERIFY DAG-safety: grep -n '^    needs:' .github/workflows/release.yml → every hit is 'needs: goreleaser'
    (NONE is 'needs: winget') — confirms no broken dependency.

Task 4: VERIFY — grep success gate + YAML parse + scope guard
  - rg -ni winget .github/workflows/release.yml          # MUST be 0 hits (THE mechanical gate)
  - yq e '.' .github/workflows/release.yml >/dev/null    # MUST parse (valid YAML)
  - grep -c '^  winget:' .github/workflows/release.yml   # MUST be 0 (no winget job)
  - grep -c 'CHOCOLATEY_API_KEY' .github/workflows/release.yml   # >= 2 (header doc + goreleaser env)
  - git diff --name-only                                 # == only .github/workflows/release.yml
```

### Implementation Patterns & Key Details

```yaml
# PATTERN: the header secret doc (multi-line, NPM_TOKEN style — 3-space indent, aligned column)
#   CHOCOLATEY_API_KEY          Chocolatey community-source push key (chocolatey.org → Account Settings
#                              → API Key). Used by the goreleaser-native `chocolateys:` pipe (in
#                              .goreleaser.yaml) to publish the .nupkg to push.chocolatey.org.
#                              Settings → Secrets → Actions.

# PATTERN: the goreleaser job env var (10-space indent, secrets.* reference)
#           CHOCOLATEY_API_KEY: ${{ secrets.CHOCOLATEY_API_KEY }}

# PATTERN: the job-deletion region (banner start → token end + one blank line)
#   the winget block is a self-contained leaf: { comment banner, job header, one steps step } — delete
#   the whole contiguous span; the surrounding jobs (npm-publish before, asdf-mirror after) are unaffected.

# CRITICAL: the {{ .Env.CHOCOLATEY_API_KEY }} reference in .goreleaser.yaml (S1) is the CONSUMER;
#   the ${{ secrets.CHOCOLATEY_API_KEY }} env line in the goreleaser job (this task) is the PROVIDER.
#   Both must exist for the publish to work. The secret itself is configured in GitHub repo settings
#   (Settings → Secrets → Actions) — out of scope for this file edit (the doc line just tells the
#   maintainer how to obtain/set it).
```

### Integration Points

```yaml
NO Go code / tests / new deps / new CLI flags / new jobs. ONE workflow YAML file edited.

RELEASE PLUMBING (.github/workflows/release.yml):
  - header secrets: -WINGET_TOKEN doc +CHOCOLATEY_API_KEY doc.
  - goreleaser job env: +CHOCOLATEY_API_KEY (consumed by S1's chocolateys: pipe via {{ .Env.CHOCOLATEY_API_KEY }}).
  - jobs: -winget (the leaf WinGet manifest-PR job).

COORDINATION (separate subtasks, NOT this one):
  - .goreleaser.yaml: parallel S1 (P1.M2.T1.S1) adds the chocolateys: pipe that CONSUMES CHOCOLATEY_API_KEY.
  - install.ps1: P1.M2.T1.S3 (Windows curl|sh analog — the no-PM fallback; unrelated to choco).
  - docs/packaging.md + docs/cli.md + README.md: P1.M3.T1/S2 (WinGet→Chocolatey docs sync — a later task).
  - internal/upgrade/detect.go + delegate.go: parallel P1.M1 (Go ChannelChocolatey detect+PRINT — Complete).

SCOPE FENCES: NO .goreleaser.yaml; NO install.ps1; NO docs/*; NO Go code; NO PRD.md/tasks.json/prd_snapshot.md;
  NO new Chocolatey job (the native pipe publishes directly); NO tag to validate (local gates suffice).
```

## Validation Loop

> This is a GitHub-Actions workflow YAML. The primary gate is the mechanical `rg -ni winget` = 0 + a YAML
> parse (`yq`). `actionlint` is NOT installed locally, so there is no deep workflow-semantic lint — but
> the edit is a pure leaf-job deletion + a secret-doc/env swap, so YAML-parse + grep + the DAG-safety
> proof are sufficient. `make test`/`make lint` (Go) confirm the tree is otherwise clean.

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# THE mechanical success gate: zero WinGet references in any case.
rg -ni winget .github/workflows/release.yml
# Expected: NO output (0 hits). (Currently 17: header doc 13-14, banner 109-114, job 115-139.)

# YAML parse gate (yq is installed at /usr/bin/yq).
yq e '.' .github/workflows/release.yml >/dev/null && echo 'YAML OK'
# Expected: 'YAML OK'. A parse error means a bad indent / dangling key from the deletion — fix it.
# (Alt: python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml')); print('YAML OK')")

# Scope guard: only the workflow file changed.
git diff --name-only
# Expected: .github/workflows/release.yml (only).

# Confirm no Go file changed (workflow-only task).
git diff --stat -- '*.go' '*.yaml'
# Expected: '*.go' empty; '*.yaml' shows only .github/workflows/release.yml (NOT .goreleaser.yaml).
```

### Level 2: Unit Tests (Component Validation)

```bash
# No Go tests are authored or affected by a workflow edit. Run the suite ONLY to prove the tree is clean
# (parallel Go work may be in flight, but this YAML edit cannot break Go tests).
make test
# Expected: green (race detector). Unaffected by this workflow edit.
```

### Level 3: Integration Testing (System Validation)

```bash
# DAG-safety proof: confirm NO job depends on winget (deletion breaks no needs: edge).
grep -n '^    needs:' .github/workflows/release.yml
# Expected: every hit is 'needs: goreleaser' (npm-publish, asdf-mirror, apt-dnf-repo); ZERO 'needs: winget'.

# Job inventory: exactly 4 jobs remain (the winget job is gone).
grep -nE '^  [a-z][a-z-]*:' .github/workflows/release.yml
# Expected: goreleaser, npm-publish, asdf-mirror, apt-dnf-repo (4 top-level job keys); NO 'winget:'.

# Env-wiring proof: CHOCOLATEY_API_KEY is in the goreleaser job env (the chocolateys: pipe consumer).
grep -n 'CHOCOLATEY_API_KEY: \$({{)? *secrets.CHOCOLATEY_API_KEY' .github/workflows/release.yml
# (Simpler:) grep -n 'CHOCOLATEY_API_KEY' .github/workflows/release.yml
# Expected: >= 2 hits — one in the header doc block, one in the goreleaser job env block.

# NOTE: a full release-run confirmation requires tagging a 'v*' release (out of scope — the maintainer
# does that at release time). The local gates above are the deterministic proof for this subtask.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard 1 (THE gate): zero WinGet references in any case.
rg -ni winget .github/workflows/release.yml
# Expected: no output.

# Grep guard 2: no winget job key.
grep -c '^  winget:' .github/workflows/release.yml
# Expected: 0.

# Grep guard 3: no WINGET_TOKEN anywhere (header doc gone, job token gone).
grep -c 'WINGET_TOKEN' .github/workflows/release.yml
# Expected: 0.

# Grep guard 4: CHOCOLATEY_API_KEY present (header doc + goreleaser env).
grep -c 'CHOCOLATEY_API_KEY' .github/workflows/release.yml
# Expected: >= 2.

# Grep guard 5: the goreleaser env block still has its 4 original secrets + the new one.
grep -nE 'GITHUB_TOKEN:|HOMEBREW_TAP_GITHUB_TOKEN:|SCOOP_BUCKET_GITHUB_TOKEN:|AUR_SSH_PRIVATE_KEY:|CHOCOLATEY_API_KEY:' .github/workflows/release.yml
# Expected: each present (5 env vars in the goreleaser block — the 4 originals + CHOCOLATEY_API_KEY).

# Grep guard 6: the other 3 jobs are UNCHANGED.
grep -c '^  npm-publish:\|^  asdf-mirror:\|^  apt-dnf-repo:' .github/workflows/release.yml
# Expected: 3 (each job key present once).

# Grep guard 7: header doc — the WINGET_TOKEN block is gone; the GITHUB_TOKEN note is preserved.
grep -c 'auto-provided' .github/workflows/release.yml
# Expected: 1 (the '# (GITHUB_TOKEN is auto-provided …)' line survives the WINGET_TOKEN deletion).

# Scope guard: only the workflow file modified; no .goreleaser.yaml / docs / Go touched.
git diff --name-only | grep -v '^\.github/workflows/release\.yml$'
# Expected: empty (no output).
git diff --name-only | grep -E 'goreleaser|install\.ps1|docs/|\.go$'
# Expected: empty (S1's file + S3's file + P1.M3 docs + all Go untouched).
```

## Final Validation Checklist

### Technical Validation
- [ ] `rg -ni winget .github/workflows/release.yml` → 0 hits (THE mechanical gate)
- [ ] `yq e '.' .github/workflows/release.yml` parses (valid YAML)
- [ ] `make test` green (unaffected — confirms a clean tree)
- [ ] DAG-safety: `grep -n '^    needs:'` shows only `needs: goreleaser` (no broken edge)

### Feature Validation
- [ ] no `winget:` job key; exactly 4 jobs remain (goreleaser, npm-publish, asdf-mirror, apt-dnf-repo)
- [ ] no `WINGET_TOKEN` reference anywhere (header doc + job token both gone)
- [ ] header secret block has a `CHOCOLATEY_API_KEY` doc block; the `GITHUB_TOKEN is auto-provided` note preserved
- [ ] goreleaser job env block has `CHOCOLATEY_API_KEY` (+ the 4 original secrets intact)

### Scope-Boundary Validation
- [ ] `git diff --name-only` == only `.github/workflows/release.yml`
- [ ] NO `.goreleaser.yaml` edit (S1's domain); NO install.ps1 (S3); NO docs/* (P1.M3); NO Go code (P1.M1)
- [ ] NO new Chocolatey job added (the native chocolateys: pipe publishes directly within goreleaser)
- [ ] NO tag/release-run created to validate (local gates suffice for this leaf-job deletion + secret swap)

### Code Quality & Docs
- [ ] CHOCOLATEY_API_KEY header doc matches the NPM_TOKEN multi-line style (3-space indent, aligned column)
- [ ] CHOCOLATEY_API_KEY doc cites the goreleaser-native `chocolateys:` pipe + chocolatey.org source
- [ ] Edit points located by content (grep anchors), not stale line numbers
- [ ] Blank-line hygiene: single blank separator where the winget block was removed (no double-blank)

---

## Anti-Patterns to Avoid

- ❌ Don't leave ANY "winget"/"WinGet"/"WINGET" string in the file. The success gate is `rg -ni winget`
  = 0. That includes the header doc (WINGET_TOKEN, lines 13-14), the comment banner (109-114), AND the
  job (115-139) — all must go. Check with the grep before finishing.
- ❌ Don't add a separate Chocolatey job. Chocolatey is goreleaser-native: the `chocolateys:` pipe (S1,
  in .goreleaser.yaml) runs WITHIN the goreleaser job and publishes directly to push.chocolatey.org. There
  is NO external-PR pattern (unlike WinGet's vedantmgoyal9/winget-releaser fork-PR). Adding a job would
  duplicate the pipe and reintroduce the exact "separate CI job" cost Chocolatey was chosen to avoid.
- ❌ Don't put CHOCOLATEY_API_KEY in the wrong job's env. It goes in the GORELEASER job env (where the
  chocolateys: pipe runs), NOT in npm-publish/asdf-mirror/apt-dnf-repo. Those jobs have nothing to do
  with Chocolatey.
- ❌ Don't delete the winget job without confirming the DAG. Verify `grep -n '^    needs:'` shows only
  `needs: goreleaser` (no `needs: winget`) BEFORE deleting. (Currently verified safe — it's a leaf — but
  re-check at edit time in case a sibling task added a dependency.)
- ❌ Don't edit .goreleaser.yaml, install.ps1, docs/*, or any Go file. Each is a separate subtask
  (P1.M2.T1.S1, S3, P1.M3, P1.M1). This task is the release.yml winget-removal + CHOCOLATEY_API_KEY wiring ONLY.
- ❌ Don't anchor to line numbers (13-14, 54-58, 109-139). They drift. Locate each region by content:
  `grep -n WINGET_TOKEN` (header + job token), `grep -n '# --- WinGet'` (job banner start),
  `grep -n 'secrets.WINGET_TOKEN'` (job end), `grep -n 'AUR_SSH_PRIVATE_KEY'` (env insertion point).
- ❌ Don't drop the header-doc alignment. The CHOCOLATEY_API_KEY doc must match the existing
  `#   SECRET<pad>description` style (cf. NPM_TOKEN's multi-line block). A misaligned block looks sloppy
  and breaks the visual scan the header is designed for.
- ❌ Don't leave a double-blank or missing separator after the job deletion. The winget block is bracketed
  by blank lines; delete ONE so a single blank line separates the npm-publish job from the asdf-mirror
  banner. (Eyeball the region after the edit.)
- ❌ Don't tag a release to validate. A tag triggers the real release (publishes artifacts). The local
  gates (`rg winget`=0, `yq` parse, DAG-safety grep, `make test`) are the deterministic proof for this
  leaf-job deletion + secret swap; release-time confirmation is the maintainer's job.

---

## Confidence Score: 9/10

This is a four-edit change to one workflow YAML: two deletions (winget job, WINGET_TOKEN doc), two
additions (CHOCOLATEY_API_KEY doc, CHOCOLATEY_API_KEY env). The exact text of all four edit regions is
verified live (findings §1); the deletion is proven DAG-safe (winget is a leaf; the 3 siblings all need
`goreleaser`, never `winget` — findings §2); the CHOCOLATEY_API_KEY placement rationale is established
(the chocolateys: pipe runs in the goreleaser job — findings §3 + S1 PRP); validation tooling is
confirmed (yq + python3 present; actionlint absent but unnecessary for a leaf deletion + secret swap);
and the mechanical success gate (`rg -ni winget` = 0) is unambiguous. The one residual (not a full 10)
is the absence of a local deep-workflow linter (actionlint) — but the edit is structurally simple (a
contiguous leaf-job block deletion + an aligned doc/env swap), so a YAML parse + the grep gates + the
DAG-safety proof fully cover it. One-pass success is highly likely.