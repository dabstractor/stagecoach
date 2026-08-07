#!/usr/bin/env sh
# smoke.sh — local smoke test for bin/install against a fixture archive. (P2.M4.T1.S1)
#
# Mirrors npm/test/install.test.cjs in POSIX sh: builds a fixture archive + checksums at runtime
# (no committed binaries), serves them via a local http server, runs bin/install against the
# local server (via STAGECOACH_DOWNLOAD_BASE), and asserts:
#   - HAPPY path: install exits 0; the binary is installed, executable, and runs.
#   - MISMATCH path: install exits NON-ZERO, prints a mismatch message, and leaves NO binary behind
#     (abort-before-write — a tampered archive never lingers for the caller to extract).
#
# python3 ships on ubuntu-latest + macos-latest (the CI runners). Prints SMOKE PASS on success.
set -eu

fail() { printf 'SMOKE FAIL: %s\n' "$*" >&2; exit 1; }

# --- portable sha256 (host may be macos: shasum, not sha256sum) -------------------
# Same detection as bin/install (GOTCHA 6). The smoke test builds the fixture checksum on the host.
if command -v sha256sum >/dev/null 2>&1; then
  sha256() { sha256sum "$1" | awk '{print $1}'; }
elif command -v shasum >/dev/null 2>&1; then
  sha256() { shasum -a 256 "$1" | awk '{print $1}'; }
else
  fail 'smoke: need sha256sum or shasum on the host'
fi

# --- host os/arch (SAME map as bin/install) for the fixture archive name ----------
case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux)  os=linux ;;
  *) fail "unsupported host OS: $(uname -s)" ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) fail "unsupported host arch: $(uname -m)" ;;
esac

version=0.0.0
asset="stagecoach_${version}_${os}_${arch}.tar.gz"
sums="stagecoach_${version}_checksums.txt"

# --- fixture build + local http server -------------------------------------------
# Guard the trap with ${var:-} since tmp1/tmp2/srvpid are assigned mid-script.
serve="$(mktemp -d)"
tmp1=""
tmp2=""
srvpid=""
fixrepo=""
cleanup() {
  rm -rf "${serve:-}" "${tmp1:-}" "${tmp2:-}" "${fixrepo:-}"
  if [ -n "${srvpid:-}" ]; then kill "$srvpid" 2>/dev/null || true; fi
}
trap cleanup EXIT

# Build a fake stagecoach binary + a matching archive + checksums under $serve/v0.0.0/.
# The fixture archive layout (binary at the root) matches goreleaser builds.binary='stagecoach'.
mkdir -p "${serve}/v${version}"
payload="$(mktemp -d)"
printf '#!/bin/sh\necho fake-stagecoach\n' > "${payload}/stagecoach"
chmod +x "${payload}/stagecoach"
tar -czf "${serve}/v${version}/${asset}" -C "$payload" stagecoach
printf '%s  %s\n' "$(sha256 "${serve}/v${version}/${asset}")" "$asset" > "${serve}/v${version}/${sums}"

# Serve $serve via a local http server on an OS-assigned free port. We use a tiny inline Python
# server (not `python3 -m http.server 0`) because binding the socket in-process lets us print the
# CHOSEN port to stdout deterministically + flush it — independent of the http.server banner's
# wording/buffering, which differs across CPython versions (e.g. 3.14 block-buffers it to a non-TTY).
# This is the same goal (local fixture server, random port) achieved robustly across Python versions.
# SimpleHTTPRequestHandler serves relative to the process cwd, so `cd "$serve"` makes $serve the root.
cat > "${serve}/srv.py" <<'PY'
import http.server, socketserver
with socketserver.TCPServer(("127.0.0.1", 0), http.server.SimpleHTTPRequestHandler) as httpd:
    print(httpd.server_address[1], flush=True)
    httpd.serve_forever()
PY
( cd "$serve" && exec python3 "${serve}/srv.py" ) >"${serve}/port.txt" 2>"${serve}/srv.log" &
srvpid=$!
port=""
i=0
while [ $i -lt 50 ]; do
  port="$(cat "${serve}/port.txt" 2>/dev/null)"
  case "$port" in
    ''|*[!0-9]*) port=""; i=$((i + 1)); sleep 0.1 ;;
    *) break ;;
  esac
done
[ -n "$port" ] || fail "http server did not start ($(cat "${serve}/srv.log"))"
base="http://127.0.0.1:${port}"
HERE="$(cd "$(dirname "$0")" && pwd)"

# --- HAPPY PATH -----------------------------------------------------------------
tmp1="$(mktemp -d)"
ASDF_INSTALL_TYPE=version ASDF_INSTALL_VERSION="$version" ASDF_INSTALL_PATH="$tmp1" \
STAGECOACH_DOWNLOAD_BASE="$base" sh "${HERE}/../bin/install" \
  || fail "happy path: install exited non-zero"
[ -x "${tmp1}/bin/stagecoach" ] || fail "happy path: binary not installed or not executable"
"${tmp1}/bin/stagecoach" | grep -q 'fake-stagecoach' || fail "happy path: binary did not run correctly"

# --- MISMATCH PATH (abort-before-write) -----------------------------------------
# Rewrite checksums with a wrong digest (64 zeros), fresh install path, run install → must abort
# non-zero with a mismatch message and leave NO binary behind.
printf '%064d  %s\n' 0 "$asset" > "${serve}/v${version}/${sums}"
tmp2="$(mktemp -d)"
if ASDF_INSTALL_TYPE=version ASDF_INSTALL_VERSION="$version" ASDF_INSTALL_PATH="$tmp2" \
   STAGECOACH_DOWNLOAD_BASE="$base" sh "${HERE}/../bin/install" 2>"${serve}/mismatch.err"; then
  fail "mismatch path: install should have exited non-zero on a bad checksum"
fi
grep -qi 'mismatch' "${serve}/mismatch.err" || fail "mismatch path: stderr did not mention mismatch"
[ ! -e "${tmp2}/bin/stagecoach" ] || fail "mismatch path: a binary was left behind (abort-before-write violated)"

# --- LATEST-STABLE PATH ----------------------------------------------------------
# Without bin/latest-stable, asdf hands bin/install the ENTIRE version list as ASDF_INSTALL_VERSION
# → a multi-line download URL → curl "(3) URL rejected: Malformed input". This callback is what makes
# `asdf install stagecoach latest` resolve to ONE version. Verify it offline against a LOCAL git repo
# (list-all honors STAGECOACH_GIT_REPO): it must return the single newest tag, and sort -V must put
# 0.0.10 AFTER 0.0.9 (lexicographic order would wrongly pick 0.0.9). Also exercise the optional
# prefix filter ($1): "0.0.1" must match both 0.0.1 and 0.0.10 and resolve to 0.0.10.
fixrepo="$(mktemp -d)"
git -C "$fixrepo" init -q
git -C "$fixrepo" config user.email "t@t"
git -C "$fixrepo" config user.name "t"
git -C "$fixrepo" commit -q --allow-empty -m init
for t in 0.0.1 0.0.2 0.0.10 0.0.9; do git -C "$fixrepo" tag "v$t"; done

latest_out="$(STAGECOACH_GIT_REPO="$fixrepo" sh "${HERE}/../bin/latest-stable" 2>"${serve}/latest.err")" \
  || fail "latest-stable: exited non-zero ($(cat "${serve}/latest.err"))"
[ "$latest_out" = "0.0.10" ] \
  || fail "latest-stable: expected 0.0.10, got '${latest_out}' (is sort -V being used?)"
# the asdf contract: exactly ONE line (a bare version) — never the whole list.
nlines="$(printf '%s\n' "$latest_out" | wc -l | tr -d ' ')"
[ "$nlines" = "1" ] \
  || fail "latest-stable: must print one line, got ${nlines}"

flt="$(STAGECOACH_GIT_REPO="$fixrepo" sh "${HERE}/../bin/latest-stable" 0.0.1)" \
  || fail "latest-stable (filter 0.0.1): exited non-zero"
[ "$flt" = "0.0.10" ] \
  || fail "latest-stable (filter 0.0.1): expected 0.0.10, got '${flt}'"

printf 'SMOKE PASS\n'