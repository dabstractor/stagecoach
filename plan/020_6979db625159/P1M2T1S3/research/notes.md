# Research Notes — P1.M2.T1.S3 (install.ps1)

Scope: create `install.ps1` at repo root — the Windows `irm | iex` analog of `install.sh`.
Single new file. No Go/CI/docs changes (docs owned by P1.M3.T1.S1; release.yml by P1.M2.T1.S2).

## 1. Structural template: `install.sh` (repo root, 9002 bytes, git-tracked)
9 commented phases — install.ps1 mirrors them 1:1:
0. env hooks (owner/name/version/download-base override)
1. arch detect (`uname -m` → `PROCESSOR_ARCHITECTURE`)
2. resolve latest stable tag (git ls-remote primary, GitHub API fallback)
3. asset + checksums name (version WITHOUT leading v in filename; URL tag WITH v)
4. download both (DIRECT releases/download URL, not API → avoids 60/h limit)
5. SHA256 hard gate (`<64hex>  <filename>` TWO spaces; exact field-2 match, no regex)
6. extract single root binary to temp
7. install to writable PATH dir, NO auto-elevate (atomic copy→rename)
8. success msg + "not on PATH" warning
- abort-before-write invariant: trap cleans temp; verified binary moved atomically.

## 2. Asset naming — CONFIRMED against 3 sources (must match byte-for-byte)
- `.goreleaser.yaml`: `binary: stagecoach` (→ `stagecoach.exe` INSIDE the windows zip, at root);
  `archives.name_template: {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}`;
  `format_overrides windows → zip`; `ldflags -X main.version={{.Version}}` (Version = tag WITHOUT v).
  → windows asset = `stagecoach_<v-no-v>_windows_amd64.zip` / `_arm64.zip`.
- `internal/upgrade/download.go assetName()`: strip leading `v` → `stagecoach_<ver>_windows_<arch>.zip`.
- `internal/upgrade/download.go checksumsName()`: `stagecoach_<ver-no-v>_checksums.txt`.
- checksums.txt line: `<64hex>  <filename>` (TWO spaces; `strings.Fields` collapses whitespace).

## 3. GitHub Releases API — pattern from `internal/upgrade/releases.go`
- Endpoint: `GET https://api.github.com/repos/{owner}/{repo}/releases/latest` (unauthenticated).
- Headers REQUIRED: `User-Agent` (GitHub 403s the Go/PS default client), `Accept: application/vnd.github+json`.
- JSON: `tag_name` = `"v1.2.3"` (WITH leading v); `assets[].name` + `assets[].browser_download_url`.
- `browser_download_url` is a DIRECT link → 302-redirects to objects.githubusercontent.com (IWR follows redirects by default).
- Rate limit: 60 req/h/IP unauthenticated (fine for a one-shot installer; pinning via `$env:STAGECOACH_VERSION` skips the API entirely).
- Item description: resolve tag via API, then "download from the release assets" → reuse the single API response's `assets[].browser_download_url` (authoritative; matches SelectAsset). Direct-URL construction (`releases/download/v<tag>/...`) is the fallback when version is pinned (no API call).

## 4. Loop-closure: `STAGECOACH_INSTALL_METHOD=direct` → ChannelDirect → FR-U5 self-swap
- `internal/upgrade/detect.go:50`: `ChannelDirect Channel = "direct"` — the ONLY self-swap-eligible channel (FR-U1/U5).
- `detect.go:227-232`: reads `STAGECOACH_INSTALL_METHOD` env, validates against `validChannels` (incl. `direct`), returns `Channel(v)`. The npm wrapper sets the same env to `npm`. install.ps1 setting `direct` in USER scope is the reliable pin → `stagecoach upgrade` self-swaps. CONFIRMED.
- Default (ambiguous) detection also falls to ChannelDirect (detect.go:208), so even without the env tag a $LOCALAPPDATA\stagecoach binary is treated as direct — but the env tag is the spec'd, reliable path.

## 5. PowerShell gotchas (the high-value one-pass context)
1. **TLS 1.2 MANDATORY on PS 5.1** — Windows PowerShell 5.1 defaults to TLS 1.0/1.1; GitHub/objects.githubusercontent.com REJECT it (connection failures). MUST set `[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12` BEFORE any Invoke-RestMethod/Invoke-WebRequest. pwsh 7 unaffected (defaults to 1.2+). This is THE #1 install.ps1 failure mode. (SO 41618766; MS Learn answers/262118.)
2. **PATH User-scope read-back bug** — `$env:PATH` is the PROCESS scope (merged Machine+User). Writing it back to User scope POLLUTES User PATH with system entries. MUST read the CURRENT User-scope PATH via `[Environment]::GetEnvironmentVariable('PATH','User')`, prepend `$destDir`, then `[Environment]::SetEnvironmentVariable('PATH',$new,'User')`. Idempotent: skip if `$destDir` already in the User-scope split.
3. **`$Args` is reserved** — naming a `param()` `$Args` breaks `irm | iex` (caveman issue #381). Use `[switch]$DryRun` only; pin version via `$env:STAGECOACH_VERSION` (mirror install.sh — piping passes no args).
4. **`$ErrorActionPreference='Stop'`** makes cmdlet errors terminating → `try/catch/finally` cleans temp. But `Write-Error` under Stop throws before `exit`. Use an `Abort($msg)` helper that writes to stderr (`[Console]::Error.WriteLine`) + `exit 1` (mirrors install.sh `err()`); don't rely on Write-Error reaching exit.
5. **`ZipFile.ExtractToDirectory` throws if dest EXISTS** — point it at a fresh non-existent temp dir (New-Guid); do NOT pre-create it. Load via `Add-Type -AssemblyName System.IO.Compression.FileSystem` (works on PS 5.1). ExtractToDirectory creates the dir. Item description MANDATES ZipFile (not Expand-Archive, not gallery modules).
6. **goreleaser zip layout** — `stagecoach.exe` is at the ROOT of the zip (no wrapper dir); install.sh confirms `${tmp}/stagecoach` at root. After extract, `$extractDir\stagecoach.exe` exists.
7. **`$ProgressPreference='SilentlyContinue'`** — Invoke-WebRequest's progress bar tanks large-download throughput; set it before IWR -OutFile.
8. **`exit` under `irm | iex`** — `exit 1` terminates the host process (fine for a one-shot installer; not used inside a long-lived shell).
9. **PS 5.1 vs pwsh 7 syntax** — target PS 5.1 (Windows default; the shell `irm|iex` lands in). Avoid `??`, ternary `? :`, `#requires -PSEdition`. `#Requires -Version 5.1` is fine. Validate on pwsh 7.6.2 (installed at /usr/bin/pwsh on this dev box).

## 6. Validation gate — pwsh 7.6.2 installed locally
- No PSScriptAnalyzer / shellcheck-for-ps1 in CI today. S3 gate = a `pwsh` parse check (AST parser, no execution, no network):
  ```
  pwsh -NoProfile -Command "$errs=$null;[void][System.Management.Automation.Language.Parser]::ParseFile((Resolve-Path 'install.ps1').Path,[ref]$null,[ref]$errs); if($errs){$errs|%{$_.Message};exit 1}else{'parse OK'}"
  ```
- Stronger: `-DryRun` switch (resolves version + computes/prints URLs, exits before download) → `pwsh -NoProfile -File install.ps1 -DryRun` (needs network for the API call; pin via STAGECOACH_VERSION to skip).
- CI wiring for PS linting is OUT OF SCOPE (would be a separate decision; item OUTPUT only requires "a parse check confirms valid PowerShell").
- windows-latest CI matrix exists (ci.yml) — a future task COULD add a parse-check step there; not S3.

## 7. Ownership boundaries (no overlap)
- S3 touches ONLY `install.ps1` (new, repo root).
- P1.M2.T1.S2 = `.github/workflows/release.yml` (no overlap; CHOCOLATEY_API_KEY wiring is its job).
- P1.M2.T1.S1 = `.goreleaser.yaml` `chocolateys:` pipe (already Complete).
- P1.M3.T1.S1 = `docs/packaging.md` PowerShell section (Planned — S3 must NOT touch docs).

## 8. Authoritative URLs (for the PRP context section)
- PRD §21.3 install paths (spec/SPEC.md) — the `irm ... install.ps1 | iex` invocation line.
- MS Learn about_Environment_Variables (User/Machine/Process scope): https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_environment_variables
- Environment.SetEnvironmentVariable (scope overloads): https://learn.microsoft.com/en-us/dotnet/api/system.environment.setenvironmentvariable
- ZipFile.ExtractToDirectory: https://learn.microsoft.com/en-us/dotnet/api/system.io.compression.zipfile.extracttodirectory
- GitHub REST API rate limits (60/h unauth): https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api
- GitHub REST releases (tag_name + assets): https://docs.github.com/en/rest/releases/releases
- TLS 1.2 IWR gotcha (canonical answer): https://stackoverflow.com/questions/41618766/powershell-invoke-webrequest-fails-with-ssl-tls-secure-channel
- $Args breaks irm|iex: https://github.com/JuliusBrussee/caveman/issues/381