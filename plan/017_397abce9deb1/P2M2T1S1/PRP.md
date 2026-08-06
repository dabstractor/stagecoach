name: "P2.M2.T1.S1 — winget-releaser Action step + PackageIdentifier + release-asset URL wiring (PRD §21.2/§21.3; external_deps.md §4)"
description: |
  A CI-YAML + docs item that wires ongoing WinGet (Windows Package Manager) release automation. Adds
  a `winget` JOB to `.github/workflows/release.yml` (after `goreleaser`, sibling to the in-flight
  `npm-publish` job from P2.M1.T1.S3) that runs `vedantmgoyal9/winget-releaser@v2` (VERIFIED the
  maintained action + its v2 release, 2025-01-27; `russellbanks/winget-releaser` does NOT exist — the
  action embeds Komac, authored by Russell Banks) with `identifier: dabstractor.Stagecoach`, the
  release version (default = tag without `v`), an OVERRIDDEN `installers-regex` matching the goreleaser
  windows zip (the action's default only matches exe/msi/msix/appx — it would find ZERO urls for our
  `.zip` asset), `token: WINGET_TOKEN` (classic PAT, `public_repo` scope), `fork-user: dabstractor`.
  The step is GATED with `continue-on-error: true` because the action EXITS 1 if
  `dabstractor.Stagecoach` is not yet present in `microsoft/winget-pkgs` (its first step is a HEAD
  check) — so a pre-bootstrap first release must not fail the whole workflow. The one-time
  PackageIdentifier bootstrap (a manually-authored manifest submission to winget-pkgs establishing
  `InstallerType: zip` + `NestedInstallerType: portable` + `NestedInstallerFiles`) is documented as a
  release-day checklist item in a NEW `docs/packaging.md`. Inline release.yml comments carry the
  WINGET_TOKEN + installers-regex + continue-on-error rationale (Mode A). NO Go, NO .goreleaser.yaml
  change (the goreleaser `winget` pipe is an alternative route, NOT chosen). Validates via python3+
  pyyaml workflow parse + a job/inputs/gate assertion + the docs/packaging.md presence check. NOTE:
  the task's `prd_selectors` (§9.26 work-description mode) is a MISMATCH — the authoritative spec is
  external_deps.md §4 + .goreleaser.yaml + the action.yml (verified) + the contract.

---

## Goal

**Feature Goal**: Ship ongoing WinGet release automation so that, after a maintainer performs the
one-time `dabstractor.Stagecoach` bootstrap in `microsoft/winget-pkgs`, every subsequent `v*` tag
automatically opens a manifest PR to winget-pkgs (via `vedantmgoyal9/winget-releaser@v2`) that bumps
`PackageVersion` + the `windows_amd64.zip` `InstallerUrl`/SHA256 — without the bootstrap blocking or
failing the first release, and without any goreleaser-config change.

**Deliverable**:
- `.github/workflows/release.yml` — **MODIFIED**: a new `winget` JOB (`needs: goreleaser`,
  `runs-on: windows-latest`, `continue-on-error: true` on the action step) + an inline `WINGET_TOKEN`
  comment line in the secrets-header block.
- `docs/packaging.md` — **CREATED**: the PackageIdentifier decision + the one-time bootstrap
  procedure + the bootstrap manifest YAML template (`zip` + `portable` NestedInstallerType) + the
  release-day checklist + a pointer to the release.yml `winget` job.

**Success Definition**:
- release.yml is valid YAML; the `winget` job is present with `needs: goreleaser`, `runs-on:
  windows-latest`, `uses: vedantmgoyal9/winget-releaser@v2`, `with: identifier:
  dabstractor.Stagecoach`, a non-default `installers-regex` matching `windows_amd64\.zip$`,
  `token: ${{ secrets.WINGET_TOKEN }}`, `fork-user: dabstractor`, and `continue-on-error: true`.
- `docs/packaging.md` exists with the bootstrap procedure + the manifest template + the checklist.
- `git status --porcelain` shows ONLY `.github/workflows/release.yml` (modified) + `docs/packaging.md`
  (new). NO Go, NO `.goreleaser.yaml`, NO `npm/*`, NO PRD/plan/tasks.
- The npm-publish job (P2.M1.T1.S3, in flight) and the goreleaser job are UNCHANGED by this task.

## User Persona (if applicable)

**Target User**: (1) The Windows end user who installs stagecoach with `winget install
dabstractor.Stagecoach` — gets the matching release's `windows_amd64.zip`, installed as a portable
CLI. (2) The stagecoach maintainer — after a one-time bootstrap, every release auto-opens the winget
manifest PR; the job is best-effort and never blocks a Go release.
**Use Case**: Maintainer pushes `v1.2.0` → goreleaser builds + publishes the GitHub Release (with
`stagecoach_1.2.0_windows_amd64.zip` + checksums) → the `winget` job runs the action, which matches
the zip asset via `installers-regex`, fetches the existing manifest from winget-pkgs, bumps version +
URL + SHA256, and opens (and submits) a PR to `microsoft/winget-pkgs`. A Windows user then `winget
install dabstractor.Stagecoach` → gets 1.2.0.
**Pain Points Addressed**: (1) the action's default `installers-regex` does not match `.zip` → an
override so the windows zip is found; (2) the action hard-fails before the PackageIdentifier is
registered → a `continue-on-error` gate so the first release ships clean; (3) the winget manifest's
`zip`+`NestedInstallerType: portable` structure must be established once (Komac can't infer it from a
URL) → a documented bootstrap checklist + manifest template.

## Why

- **PRD §21.2/§21.3 distribution surface**: WinGet is the Windows default package manager; this job is
  the ongoing-automation half of the winget channel (the one-time bootstrap is the other half).
- **external_deps.md §4**: the chosen mechanism is the winget-releaser Action opening a PR to
  `microsoft/winget-pkgs`; the repo's contribution is (a) adding the Action step to release.yml and
  (b) the `PackageIdentifier` decision (`dabstractor.Stagecoach`). Most impl work is wiring + the
  bootstrap doc; the action authors the per-version manifest YAML.
- **Scope discipline**: this task ships ONLY release.yml (one job + one comment line) + a new
  docs/packaging.md. No Go, no `.goreleaser.yaml` (the goreleaser `winget` pipe is an alternative
  route, NOT chosen per the contract), no npm/* (P2.M1.T1 owns that surface). The winget job is a
  sibling of the npm-publish job; the two never collide.

## What

Two artifacts: a `winget` job in release.yml + a new docs/packaging.md. The goreleaser windows zip is
the installer asset; the action auto-discovers it via `installers-regex`; the SHA256 is computed by
Komac from the asset (no manual checksum wiring in release.yml).

### Success Criteria
- [ ] release.yml has a `winget` job with `needs: goreleaser`, `runs-on: windows-latest`, and a step
      `uses: vedantmgoyal9/winget-releaser@v2` with `with: { identifier: dabstractor.Stagecoach,
      installers-regex: <matches windows amd64 zip>, token: ${{ secrets.WINGET_TOKEN }}, fork-user:
      dabstractor }` and `continue-on-error: true` on that step.
- [ ] The `installers-regex` value matches the goreleaser windows amd64 zip and is NOT the action's
      default (`.(exe|msi|msix|appx)(bundle){0,1}$`).
- [ ] release.yml's secrets-header comment block has a `WINGET_TOKEN` line (classic PAT,
      `public_repo` scope). The NPM_TOKEN line (P2.M1.T1.S3) is untouched.
- [ ] `docs/packaging.md` exists and documents: the `dabstractor.Stagecoach` PackageIdentifier
      decision; the one-time bootstrap procedure; the bootstrap manifest template (`InstallerType:
      zip`, `NestedInstallerType: portable`, `NestedInstallerFiles` → `stagecoach.exe`,
      `Architecture: x64`); the release-day checklist; a pointer to the release.yml `winget` job.
- [ ] release.yml parses as valid YAML (python3+pyyaml); the goreleaser + npm-publish jobs unchanged.
- [ ] `git status --porcelain` shows ONLY release.yml (M) + docs/packaging.md (new).

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes** — the EXACT action.yml inputs (verified verbatim), the critical installers-
regex override (default does not match .zip), the action's PackageIdentifier-existence gate (exits 1
→ continue-on-error), the WINGET_TOKEN PAT scope, the goreleaser windows asset name + download URL,
the winget zip+portable manifest structure (and WHY Komac can't infer it), the sibling-coordination
with P2.M1.T1.S3's npm-publish job, and the validation commands are all below.

### Documentation & References

```yaml
# MUST READ — the verified analysis for THIS item (action inputs, the 3 critical gotchas, the
# bootstrap rationale, sibling coordination).
- docfile: plan/017_397abce9deb1/P2M2T1S1/research/findings.md
  why: "§0 the fork resolution (vedantmgoyal9@v2 is the only action; russellbanks/ doesn't exist —
        Komac is by Russell Banks); §1 the EXACT action.yml inputs; §2 installers-regex must be
        overridden for .zip; §3 the action EXITS 1 pre-bootstrap (→ continue-on-error); §4 komac
        update REUSES the existing manifest (why the bootstrap defines zip+portable structure);
        §5 WINGET_TOKEN (classic PAT, public_repo); §6 goreleaser asset names + URL; §7 windows-latest;
        §8 sibling coordination with S3; §9 do NOT use the goreleaser winget pipe; §10 validation."

# MUST READ — the authoritative spec (the winget-releaser Action route + manifest fields).
- docfile: plan/017_397abce9deb1/architecture/external_deps.md
  section: "§4 Winget (mechanism = Action opens a PR to microsoft/winget-pkgs; PackageIdentifier =
            dabstractor.Stagecoach; InstallerType zip for the goreleaser windows zip / portable for a
            bare exe; Installers[].InstallerUrl = the windows_amd64.zip URL; InstallerSha256 from
            checksums.txt; Architecture x64; 'the Action does the YAML authoring'; verify at impl)."
  critical: "§4 names the manifest fields; §1/§2 give the goreleaser asset-name + checksums formats
             (Komac hashes the asset from the URL, so no manual SHA wiring in release.yml)."

# MUST READ — the action.yml (VERIFIED verbatim at research time): inputs + the PackageIdentifier gate.
- url: https://github.com/vedantmgoyal9/winget-releaser/blob/main/action.yml
  why: "The canonical input list + defaults. identifier/version/installers-regex/max-versions-to-keep/
        release-repository/release-tag/release-notes-url/token/fork-user. The FIRST step is a HEAD
        check that EXITS 1 if the PackageIdentifier is not in winget-pkgs (the continue-on-error basis).
        installers-regex default `.(exe|msi|msix|appx)(bundle){0,1}$` does NOT match .zip (override
        basis). komac update --urls <browser_download_url> (Komac computes the SHA256)."
- url: https://github.com/marketplace/actions/winget-releaser
  why: "Confirms the PAT requirement: 'You will need to create a classic Personal Access Token (PAT)'
        with public_repo scope; fork-user specifies the account holding the winget-pkgs fork."

# MUST READ — the goreleaser config (the windows asset name + the version source + the download URL).
- file: .goreleaser.yaml
  why: "archives.name_template `{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}` +
        format_overrides (windows→zip) → asset `stagecoach_<ver>_windows_amd64.zip`. ldflags
        `-X main.version={{.Version}}` → {{.Version}} = tag WITHOUT leading v (the action's default
        `version` input matches this). scoops[].url_template gives the canonical download URL shape:
        https://github.com/dabstractor/stagecoach/releases/download/{{ .Tag }}/{{ .ArtifactName }}."
  gotcha: "The asset is a ZIP (windows override), NOT an exe/msi — the action's default installers-
           regex will not match it. This is THE reason for the installers-regex override."

# MUST READ — the existing release.yml (this task ADDS the winget job as a sibling; preserves all else).
- file: .github/workflows/release.yml
  why: "The `goreleaser` job (the job to depend on), the secrets-header comment block (ADD the
        WINGET_TOKEN line; do NOT touch the NPM_TOKEN line P2.M1.T1.S3 adds), `permissions: contents:
        write`, the `on: push: tags: v*` trigger. This task ADDS a sibling `winget` job."
  pattern: "Job shape: <name>: needs: <job>; runs-on: <os>; steps: [- uses: actions/checkout@v4 ...].
            Match this for the winget job."

# CONTRACT (in-flight) — P2.M1.T1.S3's npm-publish job is a SIBLING; do not duplicate or collide.
- docfile: plan/017_397abce9deb1/P2M1T1S3/PRP.md
  why: "S3 adds an `npm-publish` job (needs: goreleaser) + an NPM_TOKEN comment line to release.yml.
        This task adds a SEPARATE `winget` job + a WINGET_TOKEN comment line. Sibling jobs never
        collide; each task adds ONLY its own job + its own comment line to the shared header block."

# VERIFIED — winget manifest InstallerType (zip) + NestedInstallerType (portable) for a zip-of-a-bare-exe.
- url: https://learn.microsoft.com/en-us/windows/package-manager/package/manifest
  why: "The winget manifest schema. InstallerType `zip` REQUIRES NestedInstallerType +
        NestedInstallerFiles (since winget 1.5+). For a goreleaser zip containing a bare CLI exe, the
        correct combo is InstallerType: zip + NestedInstallerType: portable + NestedInstallerFiles:
        RelativeFilePath: stagecoach.exe, PortableCommandAlias: stagecoach. Architecture amd64→x64."
  critical: "Komac's `update` REUSES the existing manifest structure — it CANNOT infer
             NestedInstallerType from a URL. So the bootstrap manifest (manually authored) is where
             the zip+portable structure is pinned; the action just bumps version/URL/SHA thereafter."

# VERIFIED — the action's latest release tag (for the `uses:` pin).
- url: https://github.com/vedantmgoyal9/winget-releaser/releases
  why: "Latest tag is `v2` (Release v2, published 2025-01-27). Pin `vedantmgoyal9/winget-releaser@v2`
        (the moving major tag). FR-D5 record: verified 2025-01-27."
```

### Current Codebase tree (relevant slice — READ-ONLY references)

```bash
.goreleaser.yaml                       # windows asset = stagecoach_<ver>_windows_amd64.zip (zip override)
.github/workflows/release.yml          # ADD the `winget` job + WINGET_TOKEN comment line here
docs/                                  # cli/configuration/how-it-works/providers/README/windows-test-support — NO packaging.md yet
plan/017_397abce9deb1/architecture/external_deps.md   # §4 (the winget spec)
plan/017_397abce9deb1/P2M1T1S3/PRP.md  # S3 CONTRACT (the npm-publish sibling job)
plan/017_397abce9deb1/P2M2T1S1/research/findings.md   # THIS item's verified research
```

### Desired Codebase tree with files to be added/edited

```bash
.github/workflows/
  release.yml        # MODIFIED — ADD `winget` job (needs: goreleaser) + WINGET_TOKEN comment line
docs/
  packaging.md       # CREATED — PackageIdentifier decision + one-time bootstrap + manifest template + checklist
# NOT touched: .goreleaser.yaml, npm/*, all Go, PRD.md, plan/**, tasks.json, ci.yml, .gitignore
```

### Known Gotchas of our codebase & Library Quirks

```yaml
# CRITICAL (the action's default installers-regex does NOT match .zip): goreleaser's windows asset is a
#   ZIP (format_overrides windows→zip) named stagecoach_<ver>_windows_amd64.zip. The action's default
#   installers-regex `.(exe|msi|msix|appx)(bundle){0,1}$` matches NONE of that → zero installer urls → a
#   broken/empty manifest. MUST override: installers-regex: 'windows_amd64\.zip$' (PowerShell -match is
#   .NET regex; escape the dot). This matches exactly the one x64 zip.

# CRITICAL (the action EXITS 1 if the PackageIdentifier is not yet in winget-pkgs): the action's FIRST
#   step is a HEAD check on microsoft/winget-pkgs/manifests/d/dabstractor/Stagecoach; if absent it
#   prints an ::error:: and exit 1. Until dabstractor.Stagecoach is bootstrapped (one-time manual
#   manifest submission), the job FAILS. GATE the step with `continue-on-error: true` so a pre-bootstrap
#   first release does not fail the whole release.yml. (Also a sane permanent posture: winget is a
#   best-effort downstream surface — a winget-pkgs PR hiccup must never block a Go release.)

# CRITICAL (komac update REUSES the existing manifest — the bootstrap defines zip+portable structure):
#   Komac's update fetches the EXISTING manifest and bumps version/URL/SHA256; it does NOT inspect the
#   zip (only has the URL), so it CANNOT infer NestedInstallerType. The bootstrap manifest (manually
#   authored, submitted once) pins InstallerType: zip + NestedInstallerType: portable +
#   NestedInstallerFiles (stagecoach.exe, PortableCommandAlias: stagecoach). Document this in
#   docs/packaging.md; it is NOT code in release.yml.

# CRITICAL (WINGET_TOKEN is a classic PAT, public_repo scope — NOT the default GITHUB_TOKEN): the
#   default GITHUB_TOKEN is scoped to dabstractor/stagecoach and CANNOT fork or push to
#   dabstractor/winget-pkgs (Komac sync-fork forks microsoft/winget-pkgs → dabstractor/winget-pkgs).
#   Create a classic PAT with public_repo scope; store as repo secret WINGET_TOKEN. Mirrors the existing
#   HOMEBREW_TAP_GITHUB_TOKEN / SCOOP_BUCKET_GITHUB_TOKEN pattern.

# GOTCHA (runs-on: windows-latest, NOT ubuntu-latest): the action is shell: pwsh end-to-end and the
#   README/marketplace examples pin windows-latest. pwsh exists on ubuntu/macos too, but use
#   windows-latest to match convention and avoid cross-OS pwsh quirks. cargo-binstall + komac install
#   cleanly there.

# GOTCHA (do NOT use the goreleaser winget pipe): goreleaser has a wingets: publish pipe (alternative
#   route). The contract (external_deps.md §4) chose the winget-releaser Action. Do NOT add a wingets:
#   block to .goreleaser.yaml — it would duplicate the Action and need its own --skip/token handling.

# GOTCHA (needs: goreleaser is MANDATORY): the action reads the release ASSETS via the GitHub API
#   (releases/tags/<tag> → assets[].browser_download_url filtered by installers-regex). The GitHub
#   Release (archives + checksums) MUST exist first. Without needs: the jobs run concurrently and winget
#   could run before the release exists → zero assets → a broken manifest. needs: goreleaser also
#   fail-fasts if goreleaser failed.

# GOTCHA (sibling coordination with P2.M1.T1.S3): S3 adds an npm-publish job (needs: goreleaser) + an
#   NPM_TOKEN comment. This task adds a SEPARATE winget job + a WINGET_TOKEN comment. Add ONLY your own
#   line to the shared secrets-header comment block; do NOT edit or remove S3's NPM_TOKEN line.

# GOTCHA (version input — leave UNSET): the action defaults version to the release tag without leading v
#   ($ReleaseInfo.tag_name -replace '^v'). goreleaser {{.Version}} = tag without v. They MATCH. Do not
#   set the version input (no need; the default is correct).

# GOTCHA (release-repository / release-tag / fork-user defaults are correct): release-repository
#   defaults to github.event.repository.name = stagecoach; release-tag to the tag; fork-user to
#   github.repository_owner = dabstractor. Set fork-user explicitly to dabstractor for clarity
#   (Komac forks winget-pkgs under that account). release-repository + release-tag can stay default.
```

## Implementation Blueprint

### Data models and structure
No data models — this is CI YAML + one Markdown doc. The "data" is: the `winget` job's step (action
+ inputs + the continue-on-error gate), the `WINGET_TOKEN` comment line, and the docs/packaging.md
content (PackageIdentifier decision + bootstrap procedure + manifest template + checklist).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY .github/workflows/release.yml — ADD the `winget` job + WINGET_TOKEN comment
  - PRESERVE: the entire `goreleaser` job, the `on: push: tags: v*` trigger, `permissions: contents:
    write`, the existing secrets-header comment block (incl. HOMEBREW_TAP/SCOOP_BUCKET/AUR lines and
    the NPM_TOKEN line P2.M1.T1.S3 adds). This task ADDS a sibling `winget` job + ONE comment line.
  - ADD to the secrets-header comment block (alongside NPM_TOKEN, not replacing it):
        #   WINGET_TOKEN                classic PAT, public_repo scope — forks microsoft/winget-pkgs to
        #                              dabstractor/winget-pkgs + opens the manifest PR. Settings → Secrets → Actions.
  - ADD a new job `winget` AFTER the `goreleaser` job (and after S3's `npm-publish` if present —
    sibling order does not matter; YAML map keys):
      # --- WinGet (Windows Package Manager) manifest automation (PRD §21.2/§21.3; external_deps §4) ---
      # vedantmgoyal9/winget-releaser@v2 opens a PR to microsoft/winget-pkgs per release, bumping the
      # PackageVersion + the windows_amd64.zip InstallerUrl/SHA256. Komac (the action's core) REUSES the
      # EXISTING manifest structure, so dabstractor.Stagecoach must be bootstrapped ONCE in winget-pkgs
      # first (see docs/packaging.md). Until then this step exits 1 (the action HEAD-checks the
      # PackageIdentifier) → continue-on-error keeps the first release green.
      winget:
        name: WinGet manifest PR (microsoft/winget-pkgs)
        needs: goreleaser           # the GitHub Release (windows zip + checksums) must exist first
        runs-on: windows-latest     # action is pwsh-native; matches the winget-releaser convention
        steps:
          - name: Open winget-pkgs manifest PR via WinGet Releaser
            # continue-on-error: BEST-EFFORT + FIRST-RELEASE GATE. The action exits 1 until
            # dabstractor.Stagecoach is bootstrapped in winget-pkgs (its first step HEAD-checks the
            # PackageIdentifier path). After bootstrap it succeeds; the gate is then a harmless safety
            # net (a winget-pkgs PR hiccup must never block a Go release).
            continue-on-error: true
            uses: vedantmgoyal9/winget-releaser@v2
            with:
              identifier: dabstractor.Stagecoach   # PackageIdentifier: <Publisher>.<Product>
              # installers-regex: the action's DEFAULT (.(exe|msi|msix|appx)...) does NOT match .zip —
              # our goreleaser windows asset is stagecoach_<ver>_windows_amd64.zip. Override to match
              # exactly the x64 zip (Komac maps amd64→x64). PowerShell -match is .NET regex (escape .).
              installers-regex: 'windows_amd64\.zip$'
              # version: left UNSET — the action defaults to the tag without leading v (= goreleaser
              # {{.Version}}); they match. release-repository/release-tag/fork-user defaults are correct
              # for this trigger; fork-user is set explicitly for clarity.
              fork-user: dabstractor               # Komac forks microsoft/winget-pkgs → dabstractor/winget-pkgs
              max-versions-to-keep: '0'            # keep all versions (0 = keep all; the default)
              token: ${{ secrets.WINGET_TOKEN }}   # classic PAT, public_repo scope (NOT the default GITHUB_TOKEN)
  - VALIDATE: python3 -c "import yaml; d=yaml.safe_load(open('.github/workflows/release.yml')); assert 'winget' in d['jobs']; j=d['jobs']['winget']; assert j['needs']=='goreleaser'; assert j['runs-on']=='windows-latest'; s=j['steps'][0]; assert s['uses']=='vedantmgoyal9/winget-releaser@v2'; w=s['with']; assert w['identifier']=='dabstractor.Stagecoach'; assert 'windows_amd64' in w['installers-regex'] and w['installers-regex']!='.(exe|msi|msix|appx)(bundle){0,1}$'; assert w['token']=='${{ secrets.WINGET_TOKEN }}'; assert s.get('continue-on-error') is True; print('release.yml winget OK')"

Task 2: CREATE docs/packaging.md — PackageIdentifier decision + one-time bootstrap + manifest template + checklist
  - CREATE docs/packaging.md with the sections below (Mode A maintainer doc). This is the winget
    specifics the item's DOCS field calls for (README install section is Mode B / P3 — out of scope).
  - CONTENT (adapt prose; keep the manifest YAML template verbatim-correct):
      # Packaging notes (maintainer)

      Distribution-surface decisions and one-time bootstraps that are NOT code. This file covers
      WinGet (PRD §21.2/§21.3); npm is documented in npm/README.md; Homebrew/Scoop/AUR are pending
      their target repos (release.yml `--skip=homebrew,scoop,aur`).

      ## WinGet (`dabstractor.Stagecoach`)

      Every `v*` tag runs a `winget` job in
      [release.yml](../.github/workflows/release.yml) that uses
      [`vedantmgoyal9/winget-releaser@v2`](https://github.com/vedantmgoyal9/winget-releaser) to open a
      manifest PR to [`microsoft/winget-pkgs`](https://github.com/microsoft/winget-pkgs). The action
      (powered by Komac) matches the release's `stagecoach_<version>_windows_amd64.zip` via
      `installers-regex`, then bumps `PackageVersion` + the `InstallerUrl`/`InstallerSha256` of the
      existing manifest. Windows users install with `winget install dabstractor.Stagecoach`.

      - **PackageIdentifier**: `dabstractor.Stagecoach` — `<Publisher>.<Product>` convention.
      - **Installer asset**: the goreleaser `stagecoach_<version>_windows_amd64.zip` (a ZIP containing
        the bare `stagecoach.exe`). Download URL:
        `https://github.com/dabstractor/stagecoach/releases/download/v<version>/stagecoach_<version>_windows_amd64.zip`.
      - **InstallerType**: `zip` (goreleaser ships a zip, not an exe/msi). **NestedInstallerType**:
        `portable` (the zip holds a bare CLI exe). winget REQUIRES these for a zip installer (1.5+).
      - **Secret**: `WINGET_TOKEN` — a classic PAT with `public_repo` scope (the default GITHUB_TOKEN
        cannot fork winget-pkgs). Add under repo Settings → Secrets → Actions.

      ### One-time bootstrap (release-day checklist — NOT a code step)

      The action's first step HEAD-checks `microsoft/winget-pkgs/manifests/d/dabstractor/Stagecoach`
      and EXITS 1 if the PackageIdentifier does not yet exist. So the FIRST release must be preceded
      by a one-time manual manifest submission that ESTABLISHES `dabstractor.Stagecoach`. Komac's
      `update` then REUSES this manifest's structure for every later version (it cannot infer
      `NestedInstallerType` from a URL — the bootstrap pins it).

      1. After the first `v*` release exists (goreleaser published the GitHub Release), draft the
         initial manifest. Use `wingetcreate` (winget-pkgs PR tool) or the New-Package PR template:
            wingetcreate new https://github.com/dabstractor/stagecoach/releases/download/v<version>/stagecoach_<version>_windows_amd64.zip
         then edit the generated YAML so the installer block is EXACTLY:
            PackageIdentifier: dabstractor.Stagecoach
            PackageVersion: <version>
            Installers:
              - Architecture: x64
                InstallerType: zip
                InstallerUrl: https://github.com/dabstractor/stagecoach/releases/download/v<version>/stagecoach_<version>_windows_amd64.zip
                InstallerSha256: <the windows_amd64.zip SHA256 from stagecoach_<version>_checksums.txt>
                NestedInstallerType: portable
                NestedInstallerFiles:
                  - RelativeFilePath: stagecoach.exe
                    PortableCommandAlias: stagecoach
      2. Submit the manifest as a New-Package PR to microsoft/winget-pkgs and let it merge.
      3. Confirm the PackageIdentifier path now exists. From the NEXT release on, the release.yml
         `winget` job auto-opens the version-bump PR. (The job's `continue-on-error: true` keeps the
         pre-bootstrap first release green; it is harmless thereafter.)

      ### Release-day checklist (WinGet)
      - [ ] `WINGET_TOKEN` secret exists (classic PAT, public_repo scope).
      - [ ] FIRST RELEASE ONLY: bootstrap manifest submitted + merged (above).
      - [ ] The release.yml `winget` job ran (best-effort; a winget-pkgs PR hiccup does not block).
      - [ ] The winget-pkgs manifest PR for `<version>` was opened (check microsoft/winget-pkgs PRs).

      > FR-D5 (verify at impl): re-confirm the `wingetcreate new` flow, the exact
      > `NestedInstallerType`/`NestedInstallerFiles` shape, and the `vedantmgoyal9/winget-releaser`
      > version against the current winget manifest spec + action release at implementation time.
  - VALIDATE: test -f docs/packaging.md && grep -q 'dabstractor.Stagecoach' docs/packaging.md && grep -q 'NestedInstallerType: portable' docs/packaging.md && grep -q 'One-time bootstrap' docs/packaging.md && echo "docs/packaging.md OK"

Task 3: VALIDATE — YAML parse, job/inputs/gate assertion, docs presence, scope
  - python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml')); print('YAML OK')"
  - (re-run the Task 1 assertion; then:)
  - grep -q WINGET_TOKEN .github/workflows/release.yml && echo "WINGET_TOKEN comment present"
  - test -f docs/packaging.md && grep -q 'dabstractor.Stagecoach' docs/packaging.md && echo "docs OK"
  - git status --porcelain   # ONLY release.yml (M) + docs/packaging.md (new)
```

### Implementation Patterns & Key Details

```yaml
# PATTERN (the winget job — action + the installers-regex override + the continue-on-error gate):
winget:
  name: WinGet manifest PR (microsoft/winget-pkgs)
  needs: goreleaser
  runs-on: windows-latest
  steps:
    - name: Open winget-pkgs manifest PR via WinGet Releaser
      continue-on-error: true          # FIRST-RELEASE GATE + best-effort (action exits 1 pre-bootstrap)
      uses: vedantmgoyal9/winget-releaser@v2
      with:
        identifier: dabstractor.Stagecoach
        installers-regex: 'windows_amd64\.zip$'   # OVERRIDE — default does not match .zip
        fork-user: dabstractor
        max-versions-to-keep: '0'
        token: ${{ secrets.WINGET_TOKEN }}        # classic PAT, public_repo scope
# CRITICAL load-bearing details: the installers-regex override (else zero installer urls), the
# continue-on-error gate (else pre-bootstrap first release fails), WINGET_TOKEN (not GITHUB_TOKEN),
# needs: goreleaser (release assets must exist). Get all four right.

# PATTERN (the WINGET_TOKEN comment line — add to the shared secrets-header block, not a new block):
#   WINGET_TOKEN   classic PAT, public_repo scope — forks microsoft/winget-pkgs to dabstractor/winget-pkgs
#                 + opens the manifest PR. Settings → Secrets → Actions.

# PATTERN (the bootstrap manifest template — docs/packaging.md, NOT release.yml):
#   InstallerType: zip + NestedInstallerType: portable + NestedInstallerFiles (stagecoach.exe).
#   Komac update REUSES this; it cannot infer NestedInstallerType from a URL. Architecture amd64→x64.
```

### Integration Points

```yaml
CONSUMES (READ-ONLY):
  - .goreleaser.yaml: the windows asset name (stagecoach_<ver>_windows_amd64.zip) + {{.Version}} (tag w/o v).
  - release.yml `goreleaser` job: the winget job `needs:` it (the GitHub Release must precede the manifest PR).
PRODUCES (downstream / maintainer):
  - The `winget` job: opens a manifest version-bump PR to microsoft/winget-pkgs per release (after bootstrap).
  - docs/packaging.md: the PackageIdentifier decision + the one-time bootstrap + the release-day checklist.
NO Go / build / config / migration changes. NO .goreleaser.yaml change (the goreleaser winget pipe is an
  alternative route, NOT chosen). NO npm/* (P2.M1.T1 owns that surface). NO ci.yml change.
```

## Validation Loop

> **This is a CI-YAML + Markdown item, not a Go one.** The template's `ruff`/`mypy`/`pytest`/`go test`
> gates do NOT apply. Validation = YAML parse (python3+pyyaml, available here) + a job/inputs/gate
> assertion + the docs presence check + a scope guard. The winget job CANNOT be fully exercised
> locally (it needs a real `v*` tag + a real GitHub Release + WINGET_TOKEN + the bootstrapped
> PackageIdentifier) — the gates below validate the WIRING, which is our entire contribution.

### Level 1: YAML + Markdown validity (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach
# release.yml is valid YAML AND the winget job + its load-bearing details are present + correct.
python3 -c "import yaml; d=yaml.safe_load(open('.github/workflows/release.yml')); assert 'winget' in d['jobs'], 'missing winget job'; j=d['jobs']['winget']; assert j['needs']=='goreleaser', 'needs!=goreleaser'; assert j['runs-on']=='windows-latest', 'not windows-latest'; s=j['steps'][0]; assert s['uses']=='vedantmgoyal9/winget-releaser@v2', 'wrong action pin'; w=s['with']; assert w['identifier']=='dabstractor.Stagecoach', 'wrong identifier'; assert 'windows_amd64' in w['installers-regex'], 'installers-regex missing windows_amd64'; assert w['installers-regex']!='.(exe|msi|msix|appx)(bundle){0,1}$', 'installers-regex is the default (will not match .zip)'; assert w['token']=='\${{ secrets.WINGET_TOKEN }}', 'wrong token secret'; assert w.get('fork-user')=='dabstractor', 'wrong fork-user'; assert s.get('continue-on-error') is True, 'continue-on-error gate missing'; assert 'goreleaser' in d['jobs'], 'goreleaser job changed/missing'; print('release.yml winget OK')"
# Expected: "release.yml winget OK", exit 0.

# The WINGET_TOKEN comment line is present in the secrets-header block.
grep -q 'WINGET_TOKEN' .github/workflows/release.yml && echo "WINGET_TOKEN comment present"
# Expected: "WINGET_TOKEN comment present", exit 0.

# docs/packaging.md exists with the PackageIdentifier decision + the bootstrap + the manifest template.
test -f docs/packaging.md && grep -q 'dabstractor.Stagecoach' docs/packaging.md && grep -q 'One-time bootstrap' docs/packaging.md && grep -q 'NestedInstallerType: portable' docs/packaging.md && grep -q 'InstallerType: zip' docs/packaging.md && echo "docs/packaging.md OK"
# Expected: "docs/packaging.md OK", exit 0.
```

### Level 2: Sibling-coordination sanity (do not collide with P2.M1.T1.S3)

```bash
cd /home/dustin/projects/stagecoach
# The goreleaser job is UNCHANGED (this task only ADDS the winget sibling).
python3 -c "import yaml; d=yaml.safe_load(open('.github/workflows/release.yml')); assert 'goreleaser' in d['jobs']; g=d['jobs']['goreleaser']; assert any(('goreleaser' in (s.get('uses',''))) for s in g['steps']); print('goreleaser job intact')"
# Expected: "goreleaser job intact", exit 0.
# (The npm-publish job from S3 may or may not be present depending on merge order; if present, it is a
#  separate sibling and must NOT be edited by this task. Do not assert on it.)
```

### Level 3: Asset-name / URL correctness cross-check (against .goreleaser.yaml)

```bash
cd /home/dustin/projects/stagecoach
# The installers-regex matches the actual goreleaser windows asset name template.
python3 -c "
import yaml,re
g=yaml.safe_load(open('.goreleaser.yaml'))
# Reconstruct the windows amd64 asset name from the archive name_template + format_override.
# name_template: {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}; windows→zip.
sample='stagecoach_1.0.0_windows_amd64.zip'
r='windows_amd64\\\\.zip\$'   # the value used in release.yml (escape for python)
assert re.search(r, sample), 'installers-regex does not match the goreleaser windows asset'
assert not re.search(r, 'stagecoach_1.0.0_windows_arm64.zip'.replace('arm64','ARM64')), 'noop'
print('installers-regex matches the goreleaser windows amd64 zip OK')
"
# Expected: "installers-regex matches ... OK", exit 0. (Confirms the override actually selects the asset.)
```

### Level 4: Scope guard

```bash
cd /home/dustin/projects/stagecoach
# ONLY release.yml (modified) + docs/packaging.md (new).
git status --porcelain
# Expected: " M .github/workflows/release.yml"  "?? docs/packaging.md"  (nothing else).
git status --porcelain | grep -vE '^ ?M \.github/workflows/release\.yml$|^\?\? docs/packaging\.md$' | grep -E '.' && echo "FAIL: out-of-scope file" || echo "OK: scope clean"
# Expected: "OK: scope clean".
git status --porcelain | grep -E '\.goreleaser\.yaml|npm/|internal/|cmd/|go\.(mod|sum)|Makefile|ci\.yml|PRD\.md|plan/|tasks\.json|\.gitignore' && echo "FAIL: forbidden file" || echo "OK: no forbidden files"
# Expected: "OK: no forbidden files" (.goreleaser.yaml, npm/*, Go, ci.yml, PRD/plan/tasks UNTOUCHED).
```

## Final Validation Checklist

### Technical Validation
- [ ] release.yml parses as YAML; the `winget` job is present with `needs: goreleaser`, `runs-on:
      windows-latest`, `uses: vedantmgoyal9/winget-releaser@v2`, the non-default `installers-regex`,
      `token: WINGET_TOKEN`, `fork-user: dabstractor`, and `continue-on-error: true` (Level 1)
- [ ] The `installers-regex` value actually matches the goreleaser `stagecoach_<ver>_windows_amd64.zip`
      asset name (Level 3)
- [ ] docs/packaging.md exists with the PackageIdentifier decision + bootstrap + manifest template (Level 1)
- [ ] The WINGET_TOKEN comment line is present in the release.yml secrets-header block (Level 1)
- [ ] The goreleaser job is intact (Level 2)

### Feature Validation
- [ ] `identifier: dabstractor.Stagecoach` (the `<Publisher>.<Product>` PackageIdentifier)
- [ ] `installers-regex` is overridden (NOT the action default) so the windows `.zip` asset is matched
- [ ] `continue-on-error: true` gates the step so a pre-bootstrap first release does not fail the workflow
- [ ] `token: ${{ secrets.WINGET_TOKEN }}` (classic PAT, public_repo scope — not the default GITHUB_TOKEN)
- [ ] `needs: goreleaser` (the GitHub Release must precede the manifest PR)
- [ ] docs/packaging.md documents the one-time bootstrap + the release-day checklist + the manifest template

### Scope-Boundary Validation
- [ ] `git status --porcelain` shows ONLY `.github/workflows/release.yml` (M) + `docs/packaging.md` (new)
- [ ] NO edit to `.goreleaser.yaml` (the goreleaser winget pipe is an alternative route, not chosen)
- [ ] NO edit to `npm/*` (P2.M1.T1's surface), `ci.yml`, any Go file, go.mod/sum, Makefile, .gitignore
- [ ] NO edit to PRD.md, plan/**, tasks.json, prd_snapshot.md
- [ ] The `goreleaser` job in release.yml is UNCHANGED (this task only ADDS a sibling `winget` job)

### Documentation & Deployment
- [ ] release.yml inline comments: the installers-regex override rationale, the continue-on-error gate
      rationale (action exits 1 pre-bootstrap), WINGET_TOKEN (PAT, public_repo scope), needs: goreleaser
- [ ] docs/packaging.md: PackageIdentifier decision + bootstrap procedure + manifest template + checklist + FR-D5 note
- [ ] The one-time PackageIdentifier bootstrap is a release-day CHECKLIST item, NOT a code step

---

## Anti-Patterns to Avoid

- ❌ Don't use the action's DEFAULT `installers-regex` — it matches only exe/msi/msix/appx, and the
  goreleaser windows asset is a `.zip`; the default finds ZERO installer urls → a broken manifest.
  Override to `windows_amd64\.zip$`.
- ❌ Don't omit `continue-on-error: true` — the action EXITS 1 if `dabstractor.Stagecoach` is not yet
  in winget-pkgs (its first step HEAD-checks the PackageIdentifier path). Without the gate the
  pre-bootstrap first release FAILS the whole release.yml. The gate is also a sane permanent posture.
- ❌ Don't use the default `GITHUB_TOKEN` for `token` — it is scoped to `dabstractor/stagecoach` and
  CANNOT fork or push to `dabstractor/winget-pkgs`. Use a classic PAT (`WINGET_TOKEN`, `public_repo`
  scope). Komac's sync-fork needs to fork microsoft/winget-pkgs under the dabstractor account.
- ❌ Don't run the winget job on `ubuntu-latest`/`macos-latest` "for speed" — the action is pwsh-native
  and the winget-releaser convention is `windows-latest`. Use `windows-latest`.
- ❌ Don't drop `needs: goreleaser` — the action reads release ASSETS via the API; the GitHub Release
  (windows zip + checksums) MUST exist first, or the action finds zero assets. `needs:` also fail-fasts
  on a goreleaser failure.
- ❌ Don't add a `wingets:` block to `.goreleaser.yaml` — goreleaser has a winget pipe, but the contract
  (external_deps.md §4) chose the winget-releaser Action route. Two routes would duplicate + conflict.
- ❌ Don't set the `version` input — the action defaults to the tag without leading `v` (= goreleaser
  `{{.Version}}`); they match. Setting it is redundant and risks a mismatch.
- ❌ Don't put the bootstrap manifest YAML in release.yml — Komac's `update` REUSES the existing
  winget-pkgs manifest; the bootstrap is a one-time MANUAL submission (a New-Package PR), documented in
  docs/packaging.md. release.yml carries only the ongoing-automation job.
- ❌ Don't edit the `goreleaser` job or S3's `npm-publish` job — the winget job is a SIBLING. Add ONLY
  your `winget` job + your `WINGET_TOKEN` comment line; do not touch NPM_TOKEN or other jobs.
- ❌ Don't match BOTH amd64 + arm64 zips in the first cut "to be complete" — the contract names the
  `windows_amd64.zip` (singular). Match `windows_amd64\.zip$`. (arm64 is a documented optional
  enhancement; Komac would add an arm64 installer entry only if the bootstrap manifest defines one.)
- ❌ Don't pin the action to `@main`/`@latest` — pin to the moving major tag `@v2` (latest release v2,
  2025-01-27) for reproducibility + auto minor/patch updates.
- ❌ Don't forget FR-D5 — re-verify the `vedantmgoyal9/winget-releaser` version, the `installers-regex`
  behavior, and the `NestedInstallerType`/`NestedInstallerFiles` shape against live docs at impl, and
  record the date in docs/packaging.md.

---

## Confidence Score

**One-pass success likelihood: 8/10.** The wiring is fully verified against the action's actual
`action.yml` (inputs, the PackageIdentifier-existence gate, the installers-regex default that does
NOT match .zip), the goreleaser asset name, and the release.yml job shape; the four load-bearing
details (installers-regex override, continue-on-error gate, WINGET_TOKEN PAT, needs: goreleaser) are
spelled out with their failure modes. The two residual risks that keep it from 9/10: (1) the
**bootstrap manifest's `NestedInstallerType`/`NestedInstallerFiles` shape** for a goreleaser zip-of-
a-bare-exe is correct per the winget manifest schema (zip requires NestedInstallerType since 1.5;
portable is right for a bare CLI exe) but is a human-authored artifact in docs/packaging.md that can
only be confirmed against the live winget spec + `wingetcreate new` output at first release — hence
the explicit FR-D5 checklist item; (2) **Komac's behavior on a `.zip` asset** (whether it cleanly
maps amd64→x64 and reuses the bootstrap's NestedInstallerType) is best-effort and can only be
exercised by a real release after bootstrap — mitigated by the `continue-on-error: true` gate (a
winget hiccup never blocks a Go release) and the documented bootstrap-first ordering. The YAML/docs
wiring itself — our entire code contribution — is low-risk and locally validatable. No Go /
.goreleaser.yaml / npm scope surprises remain.