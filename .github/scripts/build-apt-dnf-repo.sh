#!/usr/bin/env bash
# .github/scripts/build-apt-dnf-repo.sh
# Build a GPG-signed apt repo + a dnf repo from the release's .deb/.rpm, into the gh-pages checkout.
# Called by release.yml `apt-dnf-repo`. Idempotent: rebuilds ALL metadata over the accumulated pool
# (old packages remain from the gh-pages clone; new ones are added; Packages/Release/resign every run).
#
# Args: $1 = dir of NEW .deb/.rpm (from `gh release download`), $2 = the gh-pages checkout dir.
# Env: KEY_ID (GPG key id), APT_GPG_PRIVATE_KEY (ASCII-armored secret, no passphrase).
#
# Layout produced under $2:
#   apt/dists/stable/{Release,InRelease,Release.gpg,main/binary-<arch>/Packages{,.gz}}
#   apt/pool/main/<arch>/*.deb
#   rpm/{*.rpm, repodata/, stagecoach.repo}
set -euo pipefail

NEW_PKGS="${1:?usage: $0 <new-packages-dir> <repo-dir>}"
REPO="${2:?}"
KEY_ID="${KEY_ID:?need KEY_ID}"
GPG_KEY_ARMOR="${APT_GPG_PRIVATE_KEY:?need APT_GPG_PRIVATE_KEY}"
BASE_URL="https://dabstractor.github.io/stagecoach"

export GNUPGHOME="$(mktemp -d)"
trap 'rm -rf "$GNUPGHOME"' EXIT

# --- import the signing key (no passphrase; loopback so pinentry never blocks in CI) ---
gpg --batch --import <<< "$GPG_KEY_ARMOR"
# local-user signing with an imported secret key needs no ownertrust under --batch; pinentry off:
GPG_SIGN=(gpg --batch --yes --pinentry-mode loopback --local-user "$KEY_ID")

# ============================================================ apt repo
APT="$REPO/apt"
POOL="$APT/pool/main"
mkdir -p "$POOL/amd64" "$POOL/arm64"
shopt -s nullglob
for f in "$NEW_PKGS"/*linux_amd64.deb; do cp "$f" "$POOL/amd64/"; done
for f in "$NEW_PKGS"/*linux_arm64.deb; do cp "$f" "$POOL/arm64/"; done

# Run ftparchive FROM the apt root so Filename is repo-relative (pool/main/<arch>/...) — apt then
# resolves the .deb at <deb-base-url>/<Filename>. Running from the pool dir would yield a wrong path.
cd "$APT"
for arch in amd64 arm64; do
  mkdir -p "dists/stable/main/binary-$arch"
  apt-ftparchive packages "pool/main/$arch" > "dists/stable/main/binary-$arch/Packages"
  gzip -9c "dists/stable/main/binary-$arch/Packages" > "dists/stable/main/binary-$arch/Packages.gz"
done

# Release (checksums over the dists/stable tree) + per-repo metadata
apt-ftparchive \
  -o APT::FTPArchive::Release::Origin="stagecoach" \
  -o APT::FTPArchive::Release::Label="stagecoach" \
  -o APT::FTPArchive::Release::Suite="stable" \
  -o APT::FTPArchive::Release::Codename="stable" \
  -o APT::FTPArchive::Release::Architectures="amd64 arm64" \
  -o APT::FTPArchive::Release::Components="main" \
  release dists/stable > dists/stable/Release

# sign: InRelease (inline clearsign, modern apt) + Release.gpg (detached, legacy)
"${GPG_SIGN[@]}" --output dists/stable/InRelease --clearsign dists/stable/Release
"${GPG_SIGN[@]}" --output dists/stable/Release.gpg --detach-sign dists/stable/Release

# ============================================================ dnf repo
RPM="$REPO/rpm"
mkdir -p "$RPM"
cp "$NEW_PKGS"/*.rpm "$RPM/" 2>/dev/null || true
CREATEREPO="$(command -v createrepo_c || command -v createrepo || true)"
[ -n "$CREATEREPO" ] || { echo "createrepo_c not found" >&2; exit 1; }
"$CREATEREPO" "$RPM" >/dev/null
cat > "$RPM/stagecoach.repo" <<EOF
[stagecoach]
name=stagecoach
baseurl=${BASE_URL}/rpm/
enabled=1
# nfpm does not GPG-sign the RPMs; disable the repo-level gpgcheck (apt side IS signed via InRelease).
gpgcheck=0
EOF

echo "apt + dnf repo built under $REPO"