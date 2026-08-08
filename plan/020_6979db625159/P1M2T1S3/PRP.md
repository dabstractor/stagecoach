# PRP — install.ps1: Windows `irm | iex` installer (P1.M2.T1.S3)

> **Single new file:** `install.ps1` at repo root. The Windows analog of `install.sh` (PRD §21.3;
> goal **G27**). No Go / CI / docs changes (release.yml = P1.M2.T1.S2; docs/packaging.md =
> P1.M3.T1.S1). **Mirror `install.sh` phase-for-phase** so the same asset-naming + SHA256 gate
> produces a functionally identical install.

---

## Goal

**Feature Goal**: A dependency-free PowerShell installer at the repo root that, via
`irm https://github.com/dabstractor/stagecoach/raw/main/install.ps1 | iex`, detects the Windows
arch, downloads the matching goreleaser windows zip from the latest GitHub Release, SHA256-verifies
it against `checksums.txt`, extracts `stagecoach.exe` to a user-local dir, and prepends that dir to
the **User** `PATH` (no admin) — the `rustup`/`starship`/`uv` pattern. It is the fallback for Windows
users with **no package manager** and places a package-manager-**unowned** binary tagged
`STAGECOACH_INSTALL_METHOD=direct`, so `stagecoach upgrade` self-swaps it (FR-U5).

**Deliverable**: `install.ps1` (repo root, git-tracked). Windows PowerShell 5.1-compatible (the
default Windows shell the `irm | iex` invocation lands in). Header comment block cross-referencing
PRD §21.3 and explaining it is the Windows analog of `install.sh`. A `-DryRun` switch for validation.

**Success Definition**:
- `pwsh -NoProfile` AST parse check reports **zero parse errors** (validation gate).
- `pwsh -NoProfile -File install.ps1 -DryRun` resolves the latest tag + computes the exact asset
  URLs and prints them (no download/install) — OR, with `$env:STAGECOACH_VERSION` pinned, prints the
  resolved zip/checksums names + URLs without any network call.
- A read of the script confirms all 9 contract steps (a–i below) are present and ordered, and the
  PATH read-back + tag-write use **User** scope.
- The file invokes cleanly as `irm <raw url> | iex` (no `param` named `$Args`; no `$PROFILE` reliance).

---

## Why

- **PRD §21.2 / §21.3, G27**: the distribution surface must reach the channels the audience lives in.
  `install.ps1` is the named Windows analog of the Unix `curl|sh` one-liner and the fallback for
  users with no package manager.
- **Rides the existing `direct` channel (FR-U5)**: by tagging `STAGECOACH_INSTALL_METHOD=direct` in
  the **User** environment, `stagecoach upgrade` (§9.29 / G26) recognizes the install via
  `internal/upgrade/detect.go` (reads `STAGECOACH_INSTALL_METHOD` → `ChannelDirect`, the **only**
  self-swap-eligible channel). No new upgrade channel is needed — this is the loop closure.
- **Mirrors the other three installers byte-for-byte in behavior**: `install.sh`, `npm/install.cjs`,
  `plugins/asdf-stagecoach/bin/install` all share the asset-naming + SHA256 gate. install.ps1 joins
  that set as the Windows twin.

---

## What

User-visible behavior: a Windows user runs the one-liner from PRD §21.3 and gets a working
`stagecoach.exe` on their PATH with no admin prompt. On any failure (network, checksum mismatch,
extraction), the installer aborts **without leaving any binary at the install location** (the
abort-before-write invariant shared with `install.sh` + `download.go`).

### Success Criteria

- [ ] `install.ps1` exists at repo root and passes a `pwsh` AST parse check (zero errors).
- [ ] Arch detect via `$env:PROCESSOR_ARCHITECTURE` → `amd64` (`AMD64`/`X64`) or `arm64` (`ARM64`);
  anything else (e.g. `x86`) aborts with a clear, actionable error (mirrors install.sh step 1).
- [ ] Latest tag resolved via the GitHub Releases API (`Invoke-RestMethod
  https://api.github.com/repos/dabstractor/stagecoach/releases/latest`, unauthenticated) — OR pinned
  by `$env:STAGECOACH_VERSION` (mirror install.sh's pin env, since `irm|iex` passes no args).
- [ ] Downloads `stagecoach_<v>_windows_<arch>.zip` + `stagecoach_<v>_checksums.txt` (version WITHOUT
  leading `v` in filenames; URL tag WITH `v`) from the release assets.
- [ ] **SHA256 hard gate**: computes `Get-FileHash -Algorithm SHA256` on the zip, exact-matches the
  hex from the matching `checksums.txt` line; mismatch → `Write-Error` + `exit 1`, nothing installed.
- [ ] Extracts `stagecoach.exe` to `$LOCALAPPDATA\stagecoach` via `System.IO.Compression.ZipFile`
  (no PowerShell gallery modules; no `Expand-Archive`).
- [ ] Prepends `$LOCALAPPDATA\stagecoach` to the **User** `PATH` via
  `[Environment]::SetEnvironmentVariable('PATH', ..., 'User')` (NOT `Machine` — no admin), reading
  the CURRENT User-scope PATH (not `$env:PATH`), idempotently.
- [ ] Sets `STAGECOACH_INSTALL_METHOD=direct` in the **User** environment.
- [ ] Prints a success message including **"Re-open your terminal for PATH changes to take effect."**
- [ ] Header comment block cross-references PRD §21.3 and states this is the Windows analog of
  `install.sh`; dependency-free; keeps the abort-before-write invariant.

---

## All Needed Context

### Context Completeness Check

A developer who has never seen this repo can implement install.ps1 from: `install.sh` (the phase
template), the asset-naming contract (below), the SHA256/checksums format (below), the 9 contract
steps, the PowerShell gotchas (below), and the validation gate. All of those are in this PRP.

### Documentation & References

```yaml
# MUST READ — the structural template (phase-for-phase mirror)
- file: install.sh
  why: The POSIX-sh twin. install.ps1 mirrors its 9 commented phases EXACTLY (arch detect → resolve
       tag → asset+checksums names → download → SHA256 gate → extract → install to PATH dir →
       success msg). Copy its header-comment style (cross-ref PRD, env hooks, abort invariant).
  pattern: "err() { printf 'stagecoach: %s\\n' \"$*\" >&2; }" helper + "exit 1" on any failure;
       trap cleans temp; verified binary moved atomically into place AFTER the checksum passes.
  gotcha: install.sh resolves "latest" via `git ls-remote` (no quota) with an API fallback; install.ps1
       uses the GitHub API directly (PowerShell users may have no git) — see releases.go pattern.

# MUST READ — the GitHub Releases API + asset-naming contract (Go twins)
- file: internal/upgrade/releases.go
  why: The canonical GitHub client. Endpoint GET /repos/{owner}/{repo}/releases/latest; REQUIRED
       headers User-Agent (GitHub 403s without) + Accept; JSON fields tag_name (WITH leading v),
       assets[].name + assets[].browser_download_url (302→objects.githubusercontent.com); 60/h/IP
       unauthenticated. Use the assets[].browser_download_url from the single API response.
- file: internal/upgrade/download.go
  why: assetName()/checksumsName() define the EXACT filenames; VerifySHA256 is the checksum gate.
  pattern: "tag → strip leading 'v' → stagecoach_<ver>_windows_<arch>.zip"; checksums line is
       "<64hex>  <filename>" (TWO spaces); normalize hex lowercased; mismatch = hard error.

# MUST READ — the loop closure (env tag → upgrade channel)
- file: internal/upgrade/detect.go
  why: Confirms STAGECOACH_INSTALL_METHOD=direct is read (detect.go:227) → ChannelDirect (the ONLY
       self-swap channel, detect.go:50/FR-U5). This is WHY the User-scope tag matters.

# MUST READ — goreleaser (proves the zip contains stagecoach.exe at the root + exact asset names)
- file: .goreleaser.yaml
  why: "binary: stagecoach" → the windows zip holds a root-level stagecoach.exe (no wrapper dir).
       archives.name_template → stagecoach_<Version-without-v>_windows_<Arch> + format_override zip.
       checksum name_template → stagecoach_<Version-without-v>_checksums.txt, algorithm sha256.

# External authoritative docs
- url: https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_environment_variables
  why: Confirms SetEnvironmentVariable's third arg = scope ('User'|'Machine'|'Process'); User takes
       priority over Machine; User-scope writes do NOT touch the current process $env:PATH.
  critical: "#1 installer bug: reading $env:PATH (Process=merged) and writing it back to 'User'
       POLLUTES User PATH with system entries. Read CURRENT User-scope PATH via
       GetEnvironmentVariable('PATH','User') first."
- url: https://learn.microsoft.com/en-us/dotnet/api/system.io.compression.zipfile.extracttodirectory
  why: ExtractToDirectory signature + the "throws IOException if dest EXISTS" rule.
  critical: "Point it at a fresh NON-EXISTENT temp dir (New-Guid); do NOT pre-create it. Load the
       assembly via Add-Type -AssemblyName System.IO.Compression.FileSystem (works on PS 5.1)."
- url: https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api
  why: 60 req/h/IP unauthenticated — fine for a one-shot installer; pinning via STAGECOACH_VERSION
       skips the API call entirely.
- url: https://stackoverflow.com/questions/41618766/powershell-invoke-webrequest-fails-with-ssl-tls-secure-channel
  why: The canonical answer: PS 5.1 defaults to TLS 1.0/1.1, GitHub REJECTS it → connection failure.
  critical: "MUST set [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
       BEFORE any Invoke-RestMethod/Invoke-WebRequest. (pwsh 7 defaults to TLS 1.2+, unaffected.)
       This is the single most common install.ps1 failure mode."
- url: https://github.com/JuliusBrussee/caveman/issues/381
  why: Real-world proof that a param NAMED $Args breaks `irm | iex`.
  critical: "Use [switch]$DryRun only; pin version via $env:STAGECOACH_VERSION (piping passes no args)."

# PRD (the authority)
- docfile: spec/SPEC.md
  section: "§21.3 Install paths" (the `irm ... install.ps1 | iex` invocation line) and "§21.2 goreleaser"
           (the PowerShell installer spec + the Beyond-goreleaser channels block).
```

### Current Codebase tree (relevant slice)

```bash
install.sh                 # POSIX-sh twin — the phase template (9002 bytes, git-tracked)
.goreleaser.yaml           # binary=stagecoach; windows→zip; checksums sha256 (asset names)
internal/upgrade/
  releases.go              # GitHub Releases API client (endpoint, headers, JSON, rate limit)
  download.go              # assetName()/checksumsName()/VerifySHA256() (the checksum gate)
  detect.go                # STAGECOACH_INSTALL_METHOD → ChannelDirect (loop closure)
.github/workflows/
  release.yml              # P1.M2.T1.S2 owns this (winget job deletion + CHOCOLATEY_API_KEY)
  ci.yml                   # shellcheck only for POSIX plugin scripts; NO ps1 linting today
docs/packaging.md          # P1.M3.T1.S1 owns the PowerShell docs section (NOT this task)
```

### Desired Codebase tree (file to add)

```bash
install.ps1                # NEW — the Windows irm|iex installer (this task's only output)
```

### Known Gotchas of our codebase & PowerShell Quirks

```powershell
# CRITICAL 1 — TLS 1.2 on Windows PowerShell 5.1 (the #1 failure mode). MUST precede any HTTP call.
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

# CRITICAL 2 — PATH User-scope read-back. $env:PATH is PROCESS scope (Machine+User merged). Writing it
# back to 'User' POLLUTES User PATH with system entries. Read the CURRENT User-scope PATH explicitly:
$userPath = [Environment]::GetEnvironmentVariable('PATH','User')   # NOT $env:PATH
$newPath  = "$destDir;$userPath"
[Environment]::SetEnvironmentVariable('PATH', $newPath, 'User')    # 'User' NOT 'Machine' (no admin)

# CRITICAL 3 — ZipFile.ExtractToDirectory THROWS if the dest dir exists. Use a fresh New-Guid temp dir;
# do NOT pre-create it. Load the assembly (works on PS 5.1): Add-Type -AssemblyName System.IO.Compression.FileSystem

# CRITICAL 4 — tag vs filename. GitHub tag_name = "v1.2.3" (WITH v). Filenames use version WITHOUT v.
# URL path uses the tag WITH v. Strip leading 'v' for the filename; keep it for the release URL.
$verNoV = $tag -replace '^v',''

# CRITICAL 5 — $Args is reserved; naming a param $Args breaks `irm | iex`. Use [switch]$DryRun only.

# GOTCHA 6 — under $ErrorActionPreference='Stop', Write-Error THROWS before exit is reached. Define an
# Abort($msg) helper that writes to STDERR + calls exit 1 (mirrors install.sh's err()), don't rely on
# Write-Error reaching exit.

# GOTCHA 7 — Invoke-WebRequest's progress bar tanks large-download throughput. Set
# $ProgressPreference = 'SilentlyContinue' before any IWR -OutFile.

# GOTCHA 8 — checksums.txt line is "<64hex>  <filename>" (TWO spaces). Split on \s+ and EXACT-match
# field 2 against the zip name (no regex — `.` is a wildcard; `1.2.3` substring-matches `1.2.30`).
# Normalize hex to lowercase before comparing (Get-FileHash returns uppercase).

# GOTCHA 9 — target PS 5.1 syntax (the Windows default shell). Avoid ?? null-coalescing, ternary ? :,
# and #requires -PSEdition. #Requires -Version 5.1 IS fine. Validate on pwsh 7.6.2 (installed here).

# GOTCHA 10 — abort-before-write invariant: on ANY failure, nothing is left at the install location.
# Copy the verified stagecoach.exe into $LOCALAPPDATA\stagecoach only AFTER the SHA256 gate passes;
# clean the temp dir in a finally block (mirrors install.sh's trap).
```

---

## Implementation Blueprint

### The single file: `install.ps1` (structure = install.sh's 9 phases)

Create `install.ps1` at the repo root. Mirror `install.sh`'s phase structure (it is the POSIX twin).
The script is a top-to-bottom block that executes under `irm | iex`; a `param()` block at the very
top may carry only a `[switch]$DryRun` (no positional args — piping passes none).

### Reference skeleton (adapt; do NOT copy verbatim without adapting to PS 5.1)

```powershell
#Requires -Version 5.1
# install.ps1 — stagecoach Windows installer (PRD §21.3; goal G27). The irm | iex Windows analog
# of install.sh (repo root). Dependency-free (no PowerShell gallery modules). Invocation:
#   irm https://github.com/dabstractor/stagecoach/raw/main/install.ps1 | iex
#
# Mirrors install.sh phase-for-phase (arch detect → resolve tag → asset+checksums names → download →
# SHA256 gate → extract → install to a PATH dir → success). Abort-before-write: on ANY failure
# nothing is left at the install location. Tags STAGECOACH_INSTALL_METHOD=direct (User scope) so
# `stagecoach upgrade` self-swaps (FR-U5; detect.go ChannelDirect).
#
# Env hooks (mirror install.sh): STAGECOACH_VERSION pins a tag; STAGECOACH_REPO_OWNER /
# STAGECOACH_REPO_NAME override the GitHub repo (test/mirror hook).
param([switch] $DryRun)

$ErrorActionPreference = 'Stop'          # cmdlet errors terminate → try/catch/finally cleans temp
$ProgressPreference    = 'SilentlyContinue'  # IWR progress bar tanks download throughput

$owner = if ($env:STAGECOACH_REPO_OWNER) { $env:STAGECOACH_REPO_OWNER } else { 'dabstractor' }
$name  = if ($env:STAGECOACH_REPO_NAME)  { $env:STAGECOACH_REPO_NAME }  else { 'stagecoach' }

# Abort helper — mirrors install.sh err(): stderr + exit 1 (NOT Write-Error, which throws under Stop).
function Abort([string]$msg) { [Console]::Error.WriteLine("stagecoach: $msg"); exit 1 }

# (0) TLS 1.2 — PS 5.1 defaults to TLS 1.0/1.1 which GitHub REJECTS. MUST precede any HTTP call.
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

# (1) arch detect: $env:PROCESSOR_ARCHITECTURE → amd64/arm64. Reject anything else (stagecoach ships
#     amd64/arm64 only on Windows — mirrors install.sh's arch gate).
switch -Regex ($env:PROCESSOR_ARCHITECTURE) {
  '^(AMD64|X64)$' { $arch = 'amd64'; break }
  '^ARM64$'       { $arch = 'arm64'; break }
  default         { Abort "unsupported arch: $env:PROCESSOR_ARCHITECTURE (stagecoach ships amd64/arm64 only on Windows). See https://github.com/$owner/$name#install (Scoop or Chocolatey)." }
}

# (2) resolve latest release tag via the GitHub Releases API (unauthenticated; 60/h/IP). tag_name
#     comes WITH a leading v. REQUIRED headers: User-Agent (GitHub 403s without) + Accept.
#     STAGECOACH_VERSION pins a tag (accept with/without leading v) and SKIPS the API call.
$headers = @{ 'User-Agent' = 'stagecoach-install.ps1'; 'Accept' = 'application/vnd.github+json' }
$rel = $null
if ($env:STAGECOACH_VERSION) {
  $tag = $env:STAGECOACH_VERSION
  if ($tag -notmatch '^v') { $tag = "v$tag" }
} else {
  $rel = Invoke-RestMethod -Uri "https://api.github.com/repos/$owner/$name/releases/latest" -Headers $headers
  $tag = $rel.tag_name
  if (-not $tag) { Abort "could not resolve the latest release tag (empty tag_name). Pin with STAGECOACH_VERSION." }
}
$verNoV = $tag -replace '^v',''   # FILENAMES use version WITHOUT v (download.go assetName)

# (3) asset + checksums names + URLs. Prefer the release assets' browser_download_url (authoritative;
#     matches releases.go SelectAsset) when we made the API call; else construct the direct
#     releases/download URL (version pin → no API call → no assets list).
$zipName  = "stagecoach_${verNoV}_windows_${arch}.zip"
$sumsName = "stagecoach_${verNoV}_checksums.txt"
function Find-AssetUrl([object]$assets, [string]$needle) {
  foreach ($a in $assets) { if ($a.name -ceq $needle) { return $a.browser_download_url } }
  return $null
}
if ($rel) {
  $zipUrl  = Find-AssetUrl $rel.assets $zipName
  $sumsUrl = Find-AssetUrl $rel.assets $sumsName
  if (-not $zipUrl)  { Abort "no release asset named $zipName" }
  if (-not $sumsUrl) { Abort "no release asset named $sumsName" }
} else {
  $base    = "https://github.com/$owner/$name/releases/download"   # 302→objects.githubusercontent.com
  $zipUrl  = "$base/$tag/$zipName"
  $sumsUrl = "$base/$tag/$sumsName"
}

if ($DryRun) {
  Write-Host "DRY-RUN  tag=$tag  arch=$arch"
  Write-Host "zip  = $zipUrl"
  Write-Host "sums = $sumsUrl"
  exit 0
}

# (4..9) download → verify → extract → install → tag PATH/env → success. Temp dir reaped in finally.
[Console]::Error.WriteLine("stagecoach: installing $verNoV (windows/$arch)")
$tmp = Join-Path $env:TEMP "stagecoach-install-$(New-Guid)"
try {
  # (4) download zip + checksums (IWR -OutFile is binary-safe; follows the 302 redirect by default).
  $zipPath  = Join-Path $tmp $zipName
  $sumsPath = Join-Path $tmp $sumsName
  Invoke-WebRequest -Uri $zipUrl  -OutFile $zipPath
  Invoke-WebRequest -Uri $sumsUrl -OutFile $sumsPath

  # (5) SHA256 hard gate (download.go VerifySHA256 + install.sh step 6). checksums line:
  #     "<64hex>  <filename>" (TWO spaces). Exact field-2 match; lowercase hex.
  $expected = $null
  foreach ($line in (Get-Content $sumsPath)) {
    $f = $line -split '\s+' | Where-Object { $_ }        # collapses the 2-space separator
    if ($f.Count -ge 2 -and $f[1] -ceq $zipName) { $expected = $f[0].ToLower(); break }
  }
  if (-not $expected) { Abort "no checksum line for $zipName in $sumsName" }
  $got = (Get-FileHash -Algorithm SHA256 $zipPath).Hash.ToLower()
  if ($expected -ne $got) { Abort "SHA256 mismatch for $zipName (expected $expected, got $got)" }

  # (6) extract stagecoach.exe via System.IO.Compression.ZipFile (no gallery modules). ExtractToDirectory
  #     THROWS if the dest exists → point it at a fresh New-Guid dir; do NOT pre-create it. The goreleaser
  #     zip holds stagecoach.exe at the ROOT (no wrapper dir) — confirmed by .goreleaser.yaml + install.sh.
  Add-Type -AssemblyName System.IO.Compression.FileSystem
  $extractDir = Join-Path $tmp "extract"      # fresh + non-existent → ExtractToDirectory creates it
  [System.IO.Compression.ZipFile]::ExtractToDirectory($zipPath, $extractDir)
  $exe = Join-Path $extractDir 'stagecoach.exe'
  if (-not (Test-Path $exe)) { Abort "extraction did not produce stagecoach.exe" }

  # (7) install to $LOCALAPPDATA\stagecoach (PRD §21.3; the rustup/starship/uv pattern). User-owned,
  #     no admin. Copy the VERIFIED exe (abort-before-write: only after the SHA256 gate passed).
  $destDir = Join-Path $env:LOCALAPPDATA 'stagecoach'
  if (-not (Test-Path $destDir)) { New-Item -ItemType Directory -Path $destDir -Force | Out-Null }
  Copy-Item $exe (Join-Path $destDir 'stagecoach.exe') -Force

  # (8) prepend $destDir to the USER PATH (NOT Machine — no admin). GOTCHA: read the CURRENT
  #     User-scope PATH (NOT $env:PATH, which is merged Process scope). Idempotent.
  $userPath = [Environment]::GetEnvironmentVariable('PATH','User')
  $entries  = if ($userPath) { $userPath.Split(';') } else { @() }
  if ($entries -notcontains $destDir) {
    $newPath = if ($userPath) { "$destDir;$userPath" } else { $destDir }
    [Environment]::SetEnvironmentVariable('PATH', $newPath, 'User')
  }

  # (9) tag the install method = direct (USER scope). detect.go reads STAGECOACH_INSTALL_METHOD →
  #     ChannelDirect → FR-U5 self-swap. Mirrors the npm wrapper tagging its installs.
  [Environment]::SetEnvironmentVariable('STAGECOACH_INSTALL_METHOD', 'direct', 'User')
}
finally {
  Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue   # reap temp (install.sh trap twin)
}

Write-Host "installed stagecoach $verNoV to $destDir"
Write-Host "verify with: stagecoach --version"
Write-Host "Re-open your terminal for PATH changes to take effect."
```

> The skeleton above is the implementation reference. Implement it as a single `install.ps1`,
> adapting whitespace/comments to match `install.sh`'s header style. Keep it dependency-free.

### Implementation Tasks (ordered — all inside the one new file)

```yaml
Task 1: CREATE install.ps1 at repo root (the ONLY deliverable)
  - HEADER: comment block at top — cross-ref PRD §21.3 + §21.2; state this is the Windows analog of
    install.sh; document the irm | iex invocation; document STAGECOACH_VERSION pin + the two
    STAGECOACH_REPO_OWNER/NAME overrides (mirror install.sh's header + env-hook doc).
  - PARAM: param([switch] $DryRun) ONLY. NEVER a param named $Args (breaks irm|iex — gotcha 5).
  - PROLOGUE: $ErrorActionPreference='Stop'; $ProgressPreference='SilentlyContinue'; the owner/name
    env-hook ternaries; the Abort($msg) helper (stderr + exit 1, NOT Write-Error — gotcha 6).
  - STEP (0) TLS 1.2: [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    BEFORE any HTTP call (gotcha 1).
  - STEP (1) arch detect: $env:PROCESSOR_ARCHITECTURE regex → amd64 (AMD64|X64) / arm64 (ARM64);
    else Abort with a clear pointer to Scoop/Chocolatey (mirror install.sh step 1's actionable error).
  - STEP (2) resolve tag: Invoke-RestMethod .../releases/latest with User-Agent+Accept headers; OR
    pin from $env:STAGECOACH_VERSION (normalize leading v). tag = WITH v; $verNoV = WITHOUT v (gotcha 4).
  - STEP (3) names+URLs: zipName/sumsName from $verNoV (download.go assetName/checksumsName); prefer
    browser_download_url from the API assets (Find-AssetUrl exact -ceq match), else construct the
    direct releases/download/$tag/ URL. Abort if a pinned release lacks the asset.
  - DRYRUN GATE: if $DryRun, print tag/arch/zip/sums URLs and exit 0 (validation hook).
  - STEP (4) download zip+checksums to a New-Guid temp dir via Invoke-WebRequest -OutFile.
  - STEP (5) SHA256 gate: parse checksums line, exact-match field 2 vs zipName, lowercase hex, compare
    to Get-FileHash -Algorithm SHA256; Abort on mismatch (hard gate, install.sh step 6 twin).
  - STEP (6) extract via ZipFile.ExtractToDirectory to a fresh temp dir (gotcha 3); assert
    stagecoach.exe exists at the extract root (goreleaser puts it at the root).
  - STEP (7) install: Copy-Item the VERIFIED stagecoach.exe to $LOCALAPPDATA\stagecoach (create dir).
  - STEP (8) PATH: read [Environment]::GetEnvironmentVariable('PATH','User') (NOT $env:PATH — gotcha 2);
    idempotently prepend $destDir; SetEnvironmentVariable('PATH', $new, 'User').
  - STEP (9) tag: SetEnvironmentVariable('STAGECOACH_INSTALL_METHOD','direct','User').
  - FINALLY: Remove-Item temp (abort-before-write invariant).
  - SUCCESS: Write-Host the three lines incl. "Re-open your terminal for PATH changes to take effect."
  - SYNTAX TARGET: Windows PowerShell 5.1 (gotcha 9). No ??, no ternary, no #requires -PSEdition.

Task 2: VALIDATE (does not modify install.ps1 beyond fixes the gate surfaces)
  - Parse check via the AST parser (no execution, no network) — see Level 1.
  - DryRun via STAGECOACH_VERSION (no network) AND via the live API (network) — see Level 2.
  - DO NOT add a CI step (out of scope; ci.yml has no ps1 linting; a PS-lint CI step is a separate
    decision). DO NOT touch docs/packaging.md (P1.M3.T1.S1) or release.yml (P1.M2.T1.S2).
```

### Integration Points

```yaml
REPO FILES (this task touches ONE):
  - add: install.ps1                       # new, repo root, git-tracked (twin of install.sh)

NO CHANGES TO (ownership boundaries — respect them):
  - .github/workflows/release.yml          # P1.M2.T1.S2 (winget job + CHOCOLATEY_API_KEY)
  - .goreleaser.yaml                       # P1.M2.T1.S1 (already Complete)
  - docs/packaging.md                      # P1.M3.T1.S1 (PowerShell docs section)
  - internal/upgrade/*.go                  # STAGECOACH_INSTALL_METHOD + ChannelDirect already land

IMPLICIT CONTRACTS install.ps1 must honor (read-only — do NOT change the Go side):
  - asset name = stagecoach_<v-without-v>_windows_<arch>.zip  (download.go assetName)
  - checksums  = stagecoach_<v-without-v>_checksums.txt        (download.go checksumsName)
  - checksums.txt line = "<64hex>  <filename>" (TWO spaces)     (download.go VerifySHA256)
  - STAGECOACH_INSTALL_METHOD=direct → ChannelDirect (detect.go) — must be 'User' scope
  - GitHub tag_name has a leading v; filenames do NOT (URL path keeps the v)
```

---

## Validation Loop

### Level 1: Syntax & Style (parse check — primary gate, no execution, no network)

```bash
# pwsh 7.6.2 is installed at /usr/bin/pwsh on this dev box. AST parse = zero execution, zero network.
pwsh -NoProfile -Command "$errs=$null; [void][System.Management.Automation.Language.Parser]::ParseFile((Resolve-Path 'install.ps1').Path,[ref]$null,[ref]$errs); if(\$errs){\$errs | ForEach-Object { \$_.Message }; exit 1} else {'parse OK'}"
# Expected: prints 'parse OK', exit 0. If any message prints → READ it, fix, re-run.

# Quick smoke that the param + top-level don't error on dot-source with -DryRun and a PINNED version
# (no network: pinned version skips the releases/latest API call; DryRun exits before download).
STAGECOACH_VERSION=0.0.0 pwsh -NoProfile -File install.ps1 -DryRun
# Expected: prints "DRY-RUN  tag=v0.0.0  arch=<amd64|arm64>" + the two URL lines, exit 0.
```

### Level 2: DryRun against the live API (network — resolves the real latest tag)

```bash
# Resolves the actual latest release tag from GitHub, prints the computed asset URLs, exits before
# any download/install. Confirms the API call + asset-name construction end-to-end.
pwsh -NoProfile -File install.ps1 -DryRun
# Expected: "DRY-RUN  tag=v<latest>  arch=<amd64|arm64>" + zip/sums URLs whose paths contain
# stagecoach_<latest-without-v>_windows_<arch>.zip and _checksums.txt. Non-zero arch on Linux dev
# box ($env:PROCESSOR_ARCHITECTURE may be unset/X86) → set it to fake Windows:
pwsh -NoProfile -Command "\$env:PROCESSOR_ARCHITECTURE='AMD64'; & ./install.ps1 -DryRun"
```

### Level 3: Manual end-to-end smoke (optional, Windows only — NOT required on Linux dev box)

```bash
# On a real Windows host (or windows-latest CI runner) the full install path:
#   irm https://github.com/dabstractor/stagecoach/raw/main/install.ps1 | iex
# Then in a FRESH terminal (PATH reload):
#   stagecoach --version
# Verify the install tag landed:
#   [Environment]::GetEnvironmentVariable('STAGECOACH_INSTALL_METHOD','User')   # → 'direct'
#   $env:LOCALAPPDATA\stagecoach\stagecoach.exe exists
# Not required for this task (Linux dev box has no stagecoach windows release to install); recorded
# for P1.M3 / a future windows-latest e2e harness.
```

### Level 4: Cross-surface consistency (the mirror invariant)

```bash
# install.ps1 must produce the SAME asset names as install.sh / download.go (byte-for-behavior).
# Spot-check the name templates agree (informational; no tool to run):
#   download.go:  stagecoach_<v-without-v>_windows_<arch>.zip            + _checksums.txt
#   install.sh:   stagecoach_${version}_${os}_${arch}.tar.gz (linux/darwin) — windows uses install.ps1
#   install.ps1:  stagecoach_${verNoV}_windows_${arch}.zip               + _checksums.txt   ✓
#   .goreleaser:  {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }} + format_override windows→zip
# All three sources agree. If any disagree, the SHA256 gate or the download 404s — fix install.ps1.
```

---

## Final Validation Checklist

### Technical Validation
- [ ] Level 1 parse check prints `parse OK` (zero parse errors) on `pwsh -NoProfile`.
- [ ] Level 2 DryRun (pinned `STAGECOACH_VERSION=0.0.0`, no network) prints resolved URLs and exits 0.
- [ ] `install.ps1` targets Windows PowerShell 5.1 syntax (no `??`, no ternary, no `-PSEdition`).

### Feature Validation (the 9 contract steps)
- [ ] Header cross-references PRD §21.3; states it is the Windows analog of install.sh; documents
      the `irm | iex` invocation + the `STAGECOACH_VERSION` / repo-override env hooks.
- [ ] (1) arch detect via `$env:PROCESSOR_ARCHITECTURE` (AMD64/X64→amd64, ARM64→arm64), else Abort.
- [ ] (2) latest tag via GitHub Releases API (`Invoke-RestMethod .../releases/latest`, User-Agent set),
      or pinned via `$env:STAGECOACH_VERSION`.
- [ ] (3) downloads `stagecoach_<v>_windows_<arch>.zip` + `stagecoach_<v>_checksums.txt`.
- [ ] (4) SHA256 hard gate — mismatch → Abort, nothing installed.
- [ ] (5) extracts `stagecoach.exe` to `$LOCALAPPDATA\stagecoach` via `System.IO.Compression.ZipFile`.
- [ ] (6) prepends `$LOCALAPPDATA\stagecoach` to the **User** PATH (reads User-scope PATH, not `$env:PATH`).
- [ ] (7) sets `STAGECOACH_INSTALL_METHOD=direct` in the **User** environment.
- [ ] (8) success message includes "Re-open your terminal for PATH changes to take effect."
- [ ] (9) dependency-free (no PowerShell gallery modules; no Expand-Archive).

### Code Quality & Boundaries
- [ ] Mirrors install.sh's 9-phase structure + header-comment style + Abort/err helper.
- [ ] Abort-before-write invariant: temp reaped in `finally`; exe copied only after SHA256 passes.
- [ ] `-DryRun` switch present and used by Level 1/2 validation.
- [ ] Only `install.ps1` changed — `git status` shows exactly one new file at repo root.
- [ ] No edits to release.yml / .goreleaser.yaml / docs/packaging.md / any *.go (ownership boundaries).

---

## Anti-Patterns to Avoid

- ❌ Don't read `$env:PATH` and write it back to `'User'` scope — it pollutes User PATH with system entries. Read `GetEnvironmentVariable('PATH','User')` (gotcha 2).
- ❌ Don't omit the TLS 1.2 line — PS 5.1 will fail every download against GitHub (gotcha 1).
- ❌ Don't use `Write-Error` and expect `exit 1` to run under `$ErrorActionPreference='Stop'` — Write-Error throws first. Use a stderr+exit `Abort` helper (gotcha 6).
- ❌ Don't pre-create the extract dir before `ZipFile::ExtractToDirectory` — it throws (gotcha 3).
- ❌ Don't name a param `$Args` — it breaks `irm | iex` (gotcha 5).
- ❌ Don't use PS-7-only syntax (`??`, ternary) — the invocation lands in PS 5.1 on stock Windows.
- ❌ Don't use regex to match the checksums line against the filename — `.` is a wildcard and `1.2.3` substring-matches `1.2.30`. Exact field-2 equality (gotcha 8).
- ❌ Don't add a CI step for PowerShell linting or touch docs/release.yml — out of scope for this task.
- ❌ Don't leave the binary at the install location on any failure path — abort-before-write invariant.

---

## Confidence Score: 9/10

The task is a single new file with a complete behavioral spec (9 steps), a structural template
(`install.sh`), a confirmed asset-naming contract (3 agreeing sources), a confirmed loop-closure
(`STAGECOACH_INSTALL_METHOD=direct` → ChannelDirect), and a working local validator (`pwsh` 7.6.2 +
AST parse + `-DryRun`). The one-pass risk is concentrated in the PowerShell gotchas (TLS 1.2, PATH
User-scope read-back, ExtractToDirectory dest-exists, $Args), all of which are enumerated above with
authoritative URLs. No ambiguity in scope or ownership boundaries. Not 10/10 only because end-to-end
install verification requires a real Windows release + host (Level 3 is optional/deferred).