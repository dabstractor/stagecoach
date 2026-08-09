#!/bin/sh
# install.sh — stagecoach curl|sh installer (PRD §21.3 "Direct binary (curl|sh one-liner from
# GitHub Releases)"; goal G9).
#
# Advertised in README.md + docs/README.md:
#   curl -fsSL https://github.com/dabstractor/stagecoach/raw/main/install.sh | bash
#
# Downloads the goreleaser release archive for THIS machine (linux/darwin x amd64/arm64),
# SHA256-verifies it against the release's checksums.txt, and installs the bare `stagecoach`
# binary to a writable dir on $PATH — WITHOUT auto-elevating (PRD FR-U4: never auto-sudo; if the
# chosen dir is not writable it prints the exact sudo command to re-run instead).
#
# This is the POSIX-sh twin of the three other installers, which it mirrors exactly so all four
# produce a byte-identical binary via the same asset naming + SHA256 gate:
#   • npm/install.cjs              — the npm postinstall downloader (JS)
#   • internal/upgrade/download.go — assetName()/checksumsName()/VerifySHA256() (Go)
#   • plugins/asdf-stagecoach/bin/install — the asdf/mise plugin (POSIX sh; closest analog)
#
# Env hooks:
#   STAGECOACH_VERSION        pin a version (e.g. 0.1.0 or v0.1.0); default = latest STABLE tag.
#                             (Piping to `bash` passes no args, so pin via this env.)
#   STAGECOACH_DOWNLOAD_BASE  override the releases/download URL prefix (test/mirror hook; shared
#                             with the asdf plugin + npm wrapper for cross-surface consistency).
#
# Abort-before-write invariant: on ANY failure (network, checksum, extraction, permission) it exits
# non-zero with NO binary left at the install location — the verified binary is moved into place
# atomically; the temp dir is reaped via trap. (Mirrors download.go DownloadAndVerifyArchive +
# npm/install.cjs.)
set -eu

owner="${STAGECOACH_REPO_OWNER:-dabstractor}"
name="${STAGECOACH_REPO_NAME:-stagecoach}"
base="${STAGECOACH_DOWNLOAD_BASE:-https://github.com/${owner}/${name}/releases/download}"
git_repo="${STAGECOACH_GIT_REPO:-https://github.com/${owner}/${name}.git}"

err() { printf 'stagecoach: %s\n' "$*" >&2; }

# (1) Platform detect: uname -> goreleaser GOOS/GOARCH. Windows is intentionally unsupported here —
# Windows users use Scoop/Chocolatey/PowerShell (PRD §21.3). Reject anything else with a clear, actionable error.
case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux)  os=linux ;;
  *)
    err "unsupported OS: $(uname -s). This installer supports macOS + Linux."
    err "Windows users: see https://github.com/${owner}/${name}#install (Scoop, Chocolatey, or the PowerShell installer)."
    exit 1
    ;;
esac
case "$(uname -m)" in
  x86_64|amd64)  arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) err "unsupported arch: $(uname -m) (stagecoach ships amd64/arm64 only)."; exit 1 ;;
esac

# (2) Resolve version. STAGECOACH_VERSION env (or $1) pins a tag; default = latest STABLE tag.
# Latest is resolved WITHOUT the rate-limited GitHub API — `git ls-remote --refs --tags` has no
# per-IP quota (external_deps.md §1/§3; same decision as the asdf plugin's bin/list-all). Pre-release
# tags (-rc/-beta/-alpha/-pre) are excluded so "latest" means latest STABLE.
input=""
[ -n "${STAGECOACH_VERSION:-}" ] && input="$STAGECOACH_VERSION"
[ -z "$input" ] && [ "$#" -gt 0 ] && input="$1"
# normalize: "latest"/empty -> resolve; strip a leading 'v' from a pinned tag.
case "$input" in
  ""|latest|LATEST) input="" ;;
  v*) input="${input#v}" ;;
esac

if [ -n "$input" ]; then
  version="$input"
else
  version="$(git ls-remote --refs --tags "$git_repo" 2>/dev/null \
    | sed 's#^.*refs/tags/##' \
    | sed 's/^v//' \
    | grep -E '^[0-9]' \
    | grep -vEi -- '-(rc|beta|alpha|pre)([0-9]|\.|-|$)' \
    | sort -V \
    | tail -n1)" || true
  if [ -z "$version" ]; then
    # Fallback: the GitHub releases/latest API. Rate-limited to 60 req/h/IP, but available when git
    # is not installed (a curl|sh user may have no git). Parses the "tag_name" JSON field; strips a
    # leading v. A 404/rate-limit yields empty -> the hard error below.
    version="$(curl -fsSL "https://api.github.com/repos/${owner}/${name}/releases/latest" 2>/dev/null \
      | sed -n 's/^[[:space:]]*"tag_name":[[:space:]]*"v\{0,1\}\([^"]*\)".*/\1/p' \
      | head -n1)" || true
  fi
  if [ -z "$version" ]; then
    err "could not resolve the latest release (git ls-remote and the GitHub API both failed)."
    err "pin a version explicitly, e.g.:"
    err "  STAGECOACH_VERSION=0.1.0 curl -fsSL https://github.com/${owner}/${name}/raw/main/install.sh | bash"
    exit 1
  fi
fi

# (3) goreleaser asset + checksums names (version WITHOUT leading v in the FILENAME; URL tag WITH v).
# Matches download.go assetName()/checksumsName() + .goreleaser.yaml name_template.
asset="stagecoach_${version}_${os}_${arch}.tar.gz"
sums="stagecoach_${version}_checksums.txt"
asset_url="${base}/v${version}/${asset}"
sums_url="${base}/v${version}/${sums}"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

err "installing stagecoach ${version} (${os}/${arch})"

# (4) Download archive + checksums from the DIRECT releases/download URL. curl -fsSL follows the 302
# redirect to objects.githubusercontent.com. DIRECT url (not the API) avoids the 60 req/h limit.
if ! curl -fsSL -o "${tmp}/${asset}" "$asset_url"; then
  err "failed to download ${asset_url}"
  exit 1
fi
if ! curl -fsSL -o "${tmp}/${sums}" "$sums_url"; then
  err "failed to download ${sums_url}"
  exit 1
fi

# (5) Portable SHA256: sha256sum (Linux/coreutils) or shasum -a 256 (macOS/BSD). Function form avoids
# word-splitting of a variable-stored command. Detect with command -v (POSIX), not which.
if command -v sha256sum >/dev/null 2>&1; then
  # shellcheck disable=SC2317 # defined conditionally, called below
  sha256() { sha256sum "$1" | awk '{print $1}'; }
elif command -v shasum >/dev/null 2>&1; then
  # shellcheck disable=SC2317 # defined conditionally, called below
  sha256() { shasum -a 256 "$1" | awk '{print $1}'; }
else
  err "need sha256sum or shasum to verify the download"
  exit 1
fi

# (6) checksums.txt line is "<64hex>  <filename>" (TWO spaces). awk field-2 EXACT match avoids regex
# pitfalls (`.` is a wildcard; `1.2.3` substring-matches `1.2.30`). Matches download.go's
# strings.Fields parse + the asdf plugin's bin/install.
expected="$(awk -v f="$asset" '$2==f {print $1; exit}' "${tmp}/${sums}")"
if [ -z "$expected" ]; then
  err "no checksum line for ${asset} in ${sums}"
  exit 1
fi
got="$(sha256 "${tmp}/${asset}")"
if [ "$expected" != "$got" ]; then
  err "SHA256 mismatch for ${asset} (expected ${expected}, got ${got})"
  exit 1
fi

# (7) Extract the single root-level binary into the temp dir (goreleaser builds.binary='stagecoach'
# → the archive's sole entry is ./stagecoach). The verified binary stays in $tmp until step (8)
# moves it atomically into place — the abort-before-write guarantee.
tar -xzf "${tmp}/${asset}" -C "$tmp"
src="${tmp}/stagecoach"
if [ ! -f "$src" ]; then
  err "extraction did not produce a stagecoach binary"
  exit 1
fi

# (8) Install to a writable dir on $PATH, WITHOUT auto-elevating (PRD FR-U4). Preference:
#   ~/.local/bin  (user-owned; created if missing)
#   /usr/local/bin (only if already writable by this user)
# If neither is writable, leave everything untouched and PRINT the exact sudo command (never elevate).
home_local="${HOME:-}/.local/bin"
target=""
if mkdir -p "$home_local" 2>/dev/null && [ -w "$home_local" ]; then
  target="$home_local"
elif [ -w "/usr/local/bin" ]; then
  target="/usr/local/bin"
else
  err "no writable install dir on PATH (~/.local/bin and /usr/local/bin are not writable)."
  err "re-run with privileges:"
  err "  curl -fsSL https://github.com/${owner}/${name}/raw/main/install.sh | sudo STAGECOACH_VERSION=${version} bash"
  exit 1
fi

# Atomic install: copy to a temp name INSIDE the target dir, chmod, then rename over the final path.
# A failure (disk full, etc.) never leaves a half-written binary at the install path; a leftover
# staging file is removed on any error. Renaming over a running executable is safe on Unix — the
# running process keeps the old inode (same principle FR-U9 relies on for `stagecoach upgrade`).
dest="${target}/stagecoach"
staging="${target}/.stagecoach.install.$$"
if ! cp "$src" "$staging"; then
  rm -f "$staging"; err "failed to stage the binary"; exit 1
fi
if ! chmod +x "$staging"; then
  rm -f "$staging"; err "failed to make the binary executable"; exit 1
fi
if ! mv -f "$staging" "$dest"; then
  rm -f "$staging"; err "failed to install the binary to ${dest}"; exit 1
fi

# (9) Success. Progress went to stderr (prefixed `stagecoach:`); the result line goes to stdout so it
# is script-parseable. Warn if the install dir is not on $PATH.
on_path=false
case ":${PATH:-}:" in
  *":${target}:"*) on_path=true ;;
esac
if [ "$on_path" = false ]; then
  err "${target} is not on your PATH. Add it (e.g. in your shell profile):"
  err "  export PATH=\"${target}:\$PATH\""
fi
printf 'installed stagecoach %s to %s\n' "$version" "$dest"
printf 'verify with: %s --version\n' "$dest"