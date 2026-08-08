#Requires -Version 5.1
# install.ps1 — stagecoach Windows installer (PRD §21.3 "Install paths": the irm | iex Windows
# analog of install.sh; goal G27).
#
# Advertised in the README + docs/packaging.md (P1.M3.T1.S1) as:
#   irm https://github.com/dabstractor/stagecoach/raw/main/install.ps1 | iex
#
# Downloads the goreleaser release archive for THIS machine (windows x amd64/arm64),
# SHA256-verifies it against the release's checksums.txt, extracts stagecoach.exe to a
# user-local dir ($LOCALAPPDATA\stagecoach), and prepends that dir to the USER PATH — WITHOUT
# prompting for admin (the rustup/starship/uv pattern). It is the fallback for Windows users with
# NO package manager and leaves a package-manager-UNOWNED binary tagged
# STAGECOACH_INSTALL_METHOD=direct so `stagecoach upgrade` self-swaps it (FR-U5).
#
# This is the PowerShell twin of the four other installers, which it mirrors phase-for-phase so all
# five produce a functionally identical install via the same asset naming + SHA256 gate:
#   • install.sh                   — the POSIX-sh curl|sh installer (the phase template)
#   • npm/install.cjs              — the npm postinstall downloader (JS)
#   • internal/upgrade/download.go — assetName()/checksumsName()/VerifySHA256() (Go)
#   • plugins/asdf-stagecoach/bin/install — the asdf/mise plugin (POSIX sh)
#
# Env hooks (mirror install.sh; `irm | iex` passes no args, so pinning is via env):
#   STAGECOACH_VERSION        pin a version (e.g. 0.1.0 or v0.1.0); default = the latest Release.
#   STAGECOACH_REPO_OWNER     override the GitHub repo owner (default dabstractor; test/mirror hook).
#   STAGECOACH_REPO_NAME      override the GitHub repo name (default stagecoach; test/mirror hook).
#
# Abort-before-write invariant: on ANY failure (network, checksum, extraction) it exits non-zero
# with NO binary left at the install location — the verified stagecoach.exe is copied into place
# only AFTER the SHA256 gate passes; the temp dir is reaped in a finally block. (Mirrors
# install.sh's trap + download.go DownloadAndVerifyArchive.)
#
# Targets Windows PowerShell 5.1 (the stock Windows shell the irm | iex invocation lands in).
# Validated on PowerShell 7.6.2 via the AST parse check + -DryRun gate. Dependency-free (no
# PowerShell gallery modules; no Expand-Archive).

# `irm | iex` pipes the script with no positional args, so the ONLY param is -DryRun (a pin
# version comes in through STAGECOACH_VERSION). NEVER name a param $Args — it breaks irm | iex.
param([switch] $DryRun)

$ErrorActionPreference = 'Stop'          # cmdlet errors terminate → try/catch/finally reaps temp
$ProgressPreference    = 'SilentlyContinue'  # Invoke-WebRequest's progress bar tanks download throughput

$owner = if ($env:STAGECOACH_REPO_OWNER) { $env:STAGECOACH_REPO_OWNER } else { 'dabstractor' }
$name  = if ($env:STAGECOACH_REPO_NAME)  { $env:STAGECOACH_REPO_NAME }  else { 'stagecoach' }

# Abort helper — mirrors install.sh's err(): write to STDERR + exit 1. Do NOT use Write-Error:
# under $ErrorActionPreference='Stop' it throws before `exit` is reached (so exit never runs).
function Abort([string]$msg) { [Console]::Error.WriteLine("stagecoach: $msg"); exit 1 }

# (0) TLS 1.2 — Windows PowerShell 5.1 defaults to TLS 1.0/1.1 which GitHub REJECTS, so EVERY
#     download would fail without this. MUST precede any Invoke-RestMethod / Invoke-WebRequest.
#     (pwsh 7 defaults to TLS 1.2+, so this is a no-op there.)
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

# (1) Platform detect: $env:PROCESSOR_ARCHITECTURE → goreleaser arch. stagecoach ships amd64/arm64
#     on Windows only — mirrors install.sh's arch gate. Reject anything else with a clear,
#     actionable pointer to the package-manager installers.
switch -Regex ($env:PROCESSOR_ARCHITECTURE) {
  '^(AMD64|X64)$' { $arch = 'amd64'; break }
  '^ARM64$'       { $arch = 'arm64'; break }
  default         { Abort "unsupported arch: $env:PROCESSOR_ARCHITECTURE (stagecoach ships amd64/arm64 only on Windows). See https://github.com/$owner/$name#install (Scoop or Chocolatey)." }
}

# (2) Resolve the latest release tag via the GitHub Releases API (unauthenticated; 60 req/h/IP —
#     fine for a one-shot installer). tag_name comes WITH a leading v. REQUIRED headers: User-Agent
#     (GitHub 403s without one) + Accept. STAGECOACH_VERSION pins a tag (accept with/without a
#     leading v) AND skips the API call entirely.
$headers = @{ 'User-Agent' = 'stagecoach-install.ps1'; 'Accept' = 'application/vnd.github+json' }
$rel = $null
if ($env:STAGECOACH_VERSION) {
  $tag = $env:STAGECOACH_VERSION
  if ($tag -notmatch '^v') { $tag = "v$tag" }
} else {
  $rel = Invoke-RestMethod -Uri "https://api.github.com/repos/$owner/$name/releases/latest" -Headers $headers
  $tag = $rel.tag_name
  if (-not $tag) { Abort "could not resolve the latest release tag (empty tag_name). Pin a version with STAGECOACH_VERSION." }
}
# FILENAMES use the version WITHOUT a leading v (download.go assetName strips the v; goreleaser
# {{.Version}} = tag without v). The release URL path keeps the v (gotcha: tag vs filename).
$verNoV = $tag -replace '^v',''

# (3) Asset + checksums names + URLs (download.go assetName/checksumsName). Prefer the release
#     assets' browser_download_url (authoritative; matches releases.go SelectAsset) when the API
#     was called; otherwise construct the direct releases/download URL (version pin → no API call
#     → no assets list). Exact-match the asset name — never substring/regex (a `.` would be a
#     wildcard and `1.2.3` would substring-match `1.2.30`).
$zipName  = "stagecoach_${verNoV}_windows_${arch}.zip"
$sumsName = "stagecoach_${verNoV}_checksums.txt"
function Find-AssetUrl([object]$assets, [string]$needle) {
  foreach ($a in $assets) { if ($a.name -ceq $needle) { return $a.browser_download_url } }
  return $null
}
if ($rel) {
  $zipUrl  = Find-AssetUrl $rel.assets $zipName
  $sumsUrl = Find-AssetUrl $rel.assets $sumsName
  if (-not $zipUrl)  { Abort "release $tag has no asset named $zipName" }
  if (-not $sumsUrl) { Abort "release $tag has no asset named $sumsName" }
} else {
  $base    = "https://github.com/$owner/$name/releases/download"   # 302 → objects.githubusercontent.com
  $zipUrl  = "$base/$tag/$zipName"
  $sumsUrl = "$base/$tag/$sumsName"
}

# -DryRun gate: resolve + print, then bail BEFORE any download/install. Used by the validation
# loop (Level 1/2) to exercise the API call + asset-name construction end-to-end without touching
# the filesystem or network for the archives.
if ($DryRun) {
  Write-Host "DRY-RUN  tag=$tag  arch=$arch"
  Write-Host "zip  = $zipUrl"
  Write-Host "sums = $sumsUrl"
  exit 0
}

# (4..9) download → verify → extract → install → tag PATH/env → success. The temp dir is reaped in
# the finally block (abort-before-write invariant).
[Console]::Error.WriteLine("stagecoach: installing $verNoV (windows/$arch)")
$tmp = Join-Path $env:TEMP "stagecoach-install-$(New-Guid)"
try {
  # (4) Download the zip + checksums (Invoke-WebRequest -OutFile is binary-safe and follows the
  #     302 redirect to objects.githubusercontent.com by default).
  $zipPath  = Join-Path $tmp $zipName
  $sumsPath = Join-Path $tmp $sumsName
  Invoke-WebRequest -Uri $zipUrl  -OutFile $zipPath
  Invoke-WebRequest -Uri $sumsUrl -OutFile $sumsPath

  # (5) SHA256 hard gate (download.go VerifySHA256 + install.sh step 6). A checksums.txt line is
  #     "<64hex>  <filename>" (TWO spaces). Split on whitespace, collapse empties, then EXACT-match
  #     field 2 against the zip name (no regex — see gotcha). Normalize hex to lowercase before
  #     comparing (Get-FileHash returns uppercase). Mismatch → Abort, nothing installed.
  $expected = $null
  foreach ($line in (Get-Content $sumsPath)) {
    $f = $line -split '\s+' | Where-Object { $_ }        # collapses the 2-space separator
    if ($f.Count -ge 2 -and $f[1] -ceq $zipName) { $expected = $f[0].ToLower(); break }
  }
  if (-not $expected) { Abort "no checksum line for $zipName in $sumsName" }
  $got = (Get-FileHash -Algorithm SHA256 $zipPath).Hash.ToLower()
  if ($expected -ne $got) { Abort "SHA256 mismatch for $zipName (expected $expected, got $got)" }

  # (6) Extract stagecoach.exe via System.IO.Compression.ZipFile (no PowerShell gallery modules; no
  #     Expand-Archive). ExtractToDirectory THROWS if the dest exists, so point it at a fresh dir
  #     (do NOT pre-create it). The goreleaser zip holds stagecoach.exe at the ROOT (no wrapper
  #     dir) — confirmed by .goreleaser.yaml (binary: stagecoach) + install.sh.
  Add-Type -AssemblyName System.IO.Compression.FileSystem
  $extractDir = Join-Path $tmp "extract"      # fresh + non-existent → ExtractToDirectory creates it
  [System.IO.Compression.ZipFile]::ExtractToDirectory($zipPath, $extractDir)
  $exe = Join-Path $extractDir 'stagecoach.exe'
  if (-not (Test-Path $exe)) { Abort "extraction did not produce stagecoach.exe at the archive root" }

  # (7) Install to $LOCALAPPDATA\stagecoach (PRD §21.3; the rustup/starship/uv pattern). User-owned,
  #     no admin. Copy the VERIFIED exe — abort-before-write: only AFTER the SHA256 gate passed.
  $destDir = Join-Path $env:LOCALAPPDATA 'stagecoach'
  if (-not (Test-Path $destDir)) { New-Item -ItemType Directory -Path $destDir -Force | Out-Null }
  Copy-Item $exe (Join-Path $destDir 'stagecoach.exe') -Force

  # (8) Prepend $destDir to the USER PATH (NOT Machine — no admin). GOTCHA: read the CURRENT
  #     User-scope PATH explicitly — NOT $env:PATH, which is the merged Process scope; writing that
  #     back to 'User' would POLLUTE User PATH with system entries. Idempotent (skip if present).
  $userPath = [Environment]::GetEnvironmentVariable('PATH','User')
  $entries  = if ($userPath) { $userPath.Split(';') } else { @() }
  if ($entries -notcontains $destDir) {
    $newPath = if ($userPath) { "$destDir;$userPath" } else { $destDir }
    [Environment]::SetEnvironmentVariable('PATH', $newPath, 'User')
  }

  # (9) Tag the install method = direct in the USER environment. detect.go reads
  #     STAGECOACH_INSTALL_METHOD → ChannelDirect (the ONLY self-swap-eligible channel; FR-U5) so
  #     `stagecoach upgrade` recognizes + self-swaps this install. Mirrors the npm wrapper tagging
  #     its installs. (User scope — no admin; User takes priority over Machine.)
  [Environment]::SetEnvironmentVariable('STAGECOACH_INSTALL_METHOD', 'direct', 'User')
}
finally {
  # Reap the temp dir on every path (success or failure) — the install.sh `trap` twin. Under
  # $ErrorActionPreference='Stop', Abort() already exit 1'd, but this still runs for any thrown
  # exception before the catch-free finally unwinds.
  Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}

Write-Host "installed stagecoach $verNoV to $destDir"
Write-Host "verify with: stagecoach --version"
Write-Host "Re-open your terminal for PATH changes to take effect."