# External Dependencies — goreleaser Chocolatey pipe + PowerShell installer

## goreleaser `chocolateys:` Pipe (native, verified against goreleaser master)

**Source:** `goreleaser/goreleaser` `pkg/config/config.go` — `Chocolatey` struct + `Project.Chocolateys` field.

### YAML Key
The section key is **`chocolateys:`** (plural with 's'), NOT `chocolatey:`. This mirrors the
convention of `brews:`, `scoops:`, `nfpms:`, `aurs:`.

### Field Schema (all optional unless noted)
```yaml
chocolateys:
  - name: stagecoach                          # package name
    ids: [default]                             # build IDs
    package_source_url: ""                     # package source URL
    owners: dabstractor                        # package owners
    title: Stagecoach                          # display title
    authors: Dustin Schultz                    # authors
    project_url: https://github.com/dabstractor/stagecoach
    url_template: 'https://github.com/dabstractor/stagecoach/releases/download/{{ .Tag }}/{{ .ArtifactName }}'
    icon_url: ""                               # icon URL
    copyright: ""                              # copyright string
    license_url: ""                            # license URL
    require_license_acceptance: false          # bool
    project_source_url: https://github.com/dabstractor/stagecoach
    docs_url: ""                               # docs URL
    bug_tracker_url: ""                        # bug tracker URL
    tags: "git commit cli ai"                  # tags (space-separated string)
    summary: "Snapshot-based AI commit message generator"
    description: "Snapshot-based AI commit message generator that uses YOUR local CLI agent"
    release_notes: ""                          # release notes
    dependencies: []                           # list of {id, version}
    skip_publish: false                        # bool — if true, creates .nupkg but does not push
    api_key: '{{ .Env.CHOCOLATEY_API_KEY }}'   # Chocolatey API key (env template)
    source_repo: https://push.chocolatey.org/  # Chocolatey community source repo
    goamd64: v1                                # goamd64 variant
```

### Key Notes
1. **The `api_key` is an env-var template** (`'{{ .Env.CHOCOLATEY_API_KEY }}'`), not a static string — same pattern as `brews:.repository.token`.
2. **The `source_repo` is the push target** — the Chocolatey community repository is `https://push.chocolatey.org/`.
3. **`skip_publish: false`** publishes the `.nupkg` to the source repo during `goreleaser release`.
4. **No per-release VM scan** — unlike winget-pkgs's Defender gate, Chocolatey publication is a direct API push.
5. goreleaser also has a native `winget:` pipe (`Winget` struct), but Stagecoach used external CI automation (vedantmgoyal9/winget-releaser) instead. We are NOT switching to goreleaser's winget pipe — we are replacing the entire winget channel with Chocolatey.

### DECISION GATE
If `goreleaser check` rejects any field, comment it out and ship the rest — Chocolatey must not
block the core release contract (same pattern as the existing `aurs:` block).

---

## PowerShell Installer Pattern (`install.ps1`)

**Reference implementations:** `install.sh` (repo root, 9002 bytes), `rustup`/`starship`/`uv` installers.

### Invocation
```powershell
irm https://github.com/dabstractor/stagecoach/raw/main/install.ps1 | iex
```

### Required behavior (from PRD P1.M2.T1):
1. Detect arch: `$env:PROCESSOR_ARCHITECTURE` → amd64/arm64
2. Resolve latest release tag: GitHub Releases API (unauthenticated `Invoke-RestMethod`)
3. Download `stagecoach_<v>_windows_<arch>.zip` + `checksums.txt`
4. SHA256-verify (hard gate — abort on mismatch)
5. Extract `stagecoach.exe` to `$LOCALAPPDATA\stagecoach`
6. Prepend to **user** PATH: `[Environment]::SetEnvironmentVariable('PATH', ..., 'User')`
7. Set `STAGECOACH_INSTALL_METHOD=direct` in **user** env
8. Print "re-open your shell" notice
9. Dependency-free (no PowerShell gallery modules)

### SHA256 Verification Pattern (from install.sh):
```
1. Read checksums.txt
2. Find the line matching the downloaded zip filename
3. Extract the SHA256 hash
4. Compute SHA256 of the downloaded file
5. Compare — abort on mismatch
```

---

## New Secret: CHOCOLATEY_API_KEY
- **Purpose:** Auth for goreleaser's `chocolateys:` pipe to push `.nupkg` to the Chocolatey community source.
- **Obtained from:** chocolatey.org → Account Settings → API Key
- **Added to:** GitHub repo Settings → Secrets → Actions as `CHOCOLATEY_API_KEY`
- **Replaces:** `WINGET_TOKEN` (which is being removed)