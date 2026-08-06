#!/usr/bin/env node
'use strict';
//
// install.cjs — stagecoach npm postinstall downloader (P2.M1.T1.S2).
//
// npm runs this as postinstall (see package.json scripts.postinstall='node install.cjs'). It:
//   (0) rejects unsupported platforms loudly;
//   (dev guard) skips a 0.0.0 dev placeholder unless STAGECOACH_DOWNLOAD_BASE overrides;
//   (1) fetches checksums.txt for this version (following GitHub's 302 redirect);
//   (2) downloads the matching goreleaser archive to a temp file;
//   (3) SHA256-verifies it CONSTANT-TIME against the checksums line, aborting-before-write on mismatch
//       (a tampered/truncated archive never lingers; FR-U11 staging analog);
//   (4) extracts the single stagecoach/stagecoach.exe into the versioned cache path S1's shim resolves;
//   (any error) prints a clear message + the direct-install fallback URL to stderr and exits non-zero
//       (never silently half-installs).
//
// This is the JS twin of internal/upgrade/download.go (P1.M1.T3.S2 Complete) — it mirrors the Go
// assetName()/checksumsName()/VerifySHA256()/abort-before-write discipline so the npm-cached binary
// is byte-identical to what `stagecoach upgrade` would fetch.
//
// DECISION (external_deps.md §1 + research §3): use the DIRECT releases/download/<tag>/<asset> URL,
// NOT the GitHub API. The unauthenticated API is rate-limited to 60 req/h/IP and breaks installs at
// scale (CI behind NAT, corporate networks). The direct URL has no such limit + needs no auth for a
// public repo (the esbuild/turbo/prisma pattern).
//
// GOTCHA: GitHub's download URL 302-redirects to objects.githubusercontent.com, and Node's
// https.get/http.get does NOT auto-follow redirects (unlike Go net/http). We implement explicit
// redirect-following below. The helper picks http vs https by URL protocol so the local test
// (http://127.0.0.1, no redirect) and prod (https + redirect) share one code path.

const https = require('node:https');
const http = require('node:http');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { URL } = require('node:url');

const tar = require('tar');           // tar.gz extraction (isaacs/node-tar v7 — engines.node>=18)
const extractZip = require('extract-zip'); // windows zip extraction (maxogden/extract-zip v2)

// --- Constants / platform map (must match internal/upgrade/download.go + S1's shim EXACTLY) ---
const pkg = require('./package.json');
const version = pkg.version;                                  // goreleaser {{.Version}} = tag WITHOUT leading v
const goos = process.platform === 'win32' ? 'windows' : process.platform;
const goarch = process.arch === 'x64' ? 'amd64' : process.arch;
const binaryName = process.platform === 'win32' ? 'stagecoach.exe' : 'stagecoach';
const ext = process.platform === 'win32' ? '.zip' : '.tar.gz';
// assetName mirrors download.go assetName(): stagecoach_<v-no-v>_<os>_<arch>.zip|.tar.gz
const assetName = `stagecoach_${version}_${goos}_${goarch}${ext}`;
const checksumsName = `stagecoach_${version}_checksums.txt`;
// the goreleaser matrix only builds these (linux/darwin/windows × amd64/arm64)
const SUPPORTED = new Set(['linux', 'darwin', 'win32']);
const FALLBACK_URL = 'https://github.com/dabstractor/stagecoach#install';

// --- URL + auth config ---
const tag = 'v' + version;                                   // npm version "1.0.0" -> tag "v1.0.0"
const base = process.env.STAGECOACH_DOWNLOAD_BASE ||
  'https://github.com/dabstractor/stagecoach/releases/download';
const headers = { 'User-Agent': `stagecoach-npm/${version}` }; // GH requires a real UA
if (process.env.STAGECOACH_GITHUB_TOKEN) {
  headers.Authorization = 'Bearer ' + process.env.STAGECOACH_GITHUB_TOKEN;
}

// =============================================================================
// HTTP helpers
// =============================================================================

// httpRequest — single GET, returns the (un-consumed) IncomingMessage. Picks http/https by protocol
// so prod (https) and the local test (http) share one path. 60s timeout; surfaces req errors.
function httpRequest(url, hdrs) {
  return new Promise((resolve, reject) => {
    const U = new URL(url);
    const lib = U.protocol === 'https:' ? https : http;
    const req = lib.get(url, { headers: hdrs, timeout: 60000 }, resolve);
    req.on('error', reject);
    req.on('timeout', () => req.destroy(new Error('request timed out after 60s')));
  });
}

// fetchFollowingRedirects — returns the FINAL 2xx IncomingMessage stream.
// Node does NOT auto-follow; GitHub's download URL 302 -> objects.githubusercontent.com.
// Handles 301/302/303/307/308, resolves a relative Location against the prior URL, caps hops at 5.
async function fetchFollowingRedirects(url, hdrs, hops) {
  if (hops === undefined) hops = 5;
  const res = await httpRequest(url, hdrs);
  if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
    if (hops <= 0) {
      res.resume(); // drain before throwing
      throw new Error('too many redirects (>5) fetching ' + url);
    }
    res.resume(); // drain the 3xx body before recursing
    const next = new URL(res.headers.location, url).href; // resolve relative Location
    return fetchFollowingRedirects(next, hdrs, hops - 1);
  }
  if (res.statusCode < 200 || res.statusCode >= 300) {
    res.resume(); // drain before throwing (connection reuse)
    throw new Error(`HTTP ${res.statusCode} fetching ${url}`);
  }
  return res; // final 2xx stream (caller consumes)
}

// fetchText — collect the final stream into a string (Buffer.concat of chunks). Used for checksums.txt
// (small text; no need to stream).
async function fetchText(url, hdrs) {
  const res = await fetchFollowingRedirects(url, hdrs);
  return new Promise((resolve, reject) => {
    const chunks = [];
    res.on('data', (c) => chunks.push(c));
    res.on('end', () => resolve(Buffer.concat(chunks).toString('utf8')));
    res.on('error', reject);
  });
}

// downloadFile — pipe the final stream into a file. On ANY error (network, disk) remove the partial
// file so a truncated download never lingers (abort-before-write analog; mirrors download.go).
function downloadFile(url, hdrs, dest) {
  return fetchFollowingRedirects(url, hdrs).then(
    (res) =>
      new Promise((resolve, reject) => {
        const ws = fs.createWriteStream(dest);
        const fail = (err) => {
          ws.close(() => {
            try { fs.rmSync(dest, { force: true }); } catch (_) {}
            reject(err);
          });
        };
        res.on('error', fail);
        ws.on('error', fail);
        ws.on('finish', resolve);
        res.pipe(ws);
      })
  );
}

// =============================================================================
// SHA256 + constant-time compare (mirrors download.go VerifySHA256)
// =============================================================================

// sha256File — stream the file through SHA256 (archives are multi-MB; never buffer the whole thing).
function sha256File(p) {
  return new Promise((resolve, reject) => {
    const h = crypto.createHash('sha256');
    fs.createReadStream(p)
      .on('data', (d) => h.update(d))
      .on('end', () => resolve(h.digest('hex')))
      .on('error', reject);
  });
}

// timingSafeEqualHex — constant-time compare of two 64-hex digests (the expected + the computed).
// Normalizes `want` to trim+lowercase first (download.go VerifySHA256 does the same). Guards equal
// length first because crypto.timingSafeEqual THROWS on length mismatch.
function timingSafeEqualHex(gotHex, wantHex) {
  const want = Buffer.from(String(wantHex).trim().toLowerCase(), 'hex');
  const got = Buffer.from(gotHex, 'hex');
  return got.length === want.length && crypto.timingSafeEqual(got, want);
}

// isHex64 — true iff s is exactly 64 hex chars ([0-9a-fA-F]). Mirrors download.go isHex64; the validity
// gate for a checksums.txt digest field.
function isHex64(s) {
  return typeof s === 'string' && s.length === 64 && /^[0-9a-fA-F]{64}$/.test(s);
}

// =============================================================================
// checksums.txt parse (mirrors download.go FetchChecksums)
// =============================================================================

// parseChecksum — parse "<64hex>  <filename>" lines and return the digest for `name`.
// Requires exactly 2 whitespace-separated fields + a valid 64-hex digest per line. Throws on a
// malformed line OR if the asset has no entry (mirrors download.go ErrChecksumParse / ErrChecksumMissing).
function parseChecksum(text, name) {
  const map = {};
  const lines = text.split('\n');
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].trim();
    if (line === '') continue;
    const fields = line.split(/\s+/);
    if (fields.length !== 2 || !isHex64(fields[0])) {
      throw new Error(`malformed checksums.txt line ${i + 1}: ${line}`);
    }
    map[fields[1]] = fields[0].toLowerCase();
  }
  if (!(name in map)) {
    throw new Error(`checksums.txt has no entry for ${name}`);
  }
  return map[name];
}

// =============================================================================
// main
// =============================================================================

async function main() {
  // (0) unsupported platform -> loud failure (goreleaser only builds linux/darwin/windows × amd64/arm64).
  if (!SUPPORTED.has(process.platform)) {
    throw new Error(
      `unsupported platform '${process.platform}' ` +
        "(stagecoach ships linux/darwin/windows x amd64/arm64). " +
        'Install directly: ' + FALLBACK_URL
    );
  }

  // (dev guard) version 0.0.0 + no override -> skip. package.json version is the dev placeholder
  // until S3 bumps it at publish; a real release download for v0.0.0 would 404 and break `npm install`
  // during dev. The local test sets STAGECOACH_DOWNLOAD_BASE (bypassing this guard).
  if (version === '0.0.0' && !process.env.STAGECOACH_DOWNLOAD_BASE) {
    console.log(
      'stagecoach: dev placeholder version 0.0.0 — skipping binary download ' +
        '(publish sets a real version).'
    );
    return;
  }

  // (1) cache dir (must match S1's shim resolution EXACTLY):
  //     <STAGECOACH_CACHE_DIR||~/.stagecoach/versions>/<version>/<goos>-<goarch>/
  const cacheRoot =
    process.env.STAGECOACH_CACHE_DIR || path.join(os.homedir(), '.stagecoach', 'versions');
  const destDir = path.join(cacheRoot, version, `${goos}-${goarch}`);

  // (2) fetch checksums.txt (follows the 302), parse the asset's expected SHA256.
  const sumsText = await fetchText(`${base}/${tag}/${checksumsName}`, headers);
  const expected = parseChecksum(sumsText, assetName);

  // (3) download the archive to a temp file.
  const tmpArchive = path.join(os.tmpdir(), `${assetName}.part`);
  await downloadFile(`${base}/${tag}/${assetName}`, headers, tmpArchive);

  // (4) SHA256-verify CONSTANT-TIME. On mismatch REMOVE the partial file + throw (abort-before-write:
  // a tampered/truncated archive must never linger for a caller to accidentally use). Mirrors
  // download.go DownloadAndVerifyArchive's os.Remove on failure.
  const got = await sha256File(tmpArchive);
  if (!timingSafeEqualHex(got, expected)) {
    try { fs.rmSync(tmpArchive, { force: true }); } catch (_) {}
    throw new Error(`SHA256 mismatch for ${assetName}: got ${got} want ${expected}`);
  }

  // (5) extract the single binary into destDir (mkdir -p first). The binary ships at the archive ROOT
  // (goreleaser builds.binary='stagecoach'), so extracting INTO <goos>-<goarch>/ lands it exactly at
  // the path S1's shim resolves. tar.gz -> tar.x; zip -> extract-zip (no shell-out; Windows may lack
  // system tar/unzip).
  fs.mkdirSync(destDir, { recursive: true });
  if (process.platform === 'win32') {
    await extractZip(tmpArchive, { dir: destDir });
  } else {
    await tar.x({ file: tmpArchive, cwd: destDir });
  }
  try { fs.rmSync(tmpArchive, { force: true }); } catch (_) {}

  // (6) defense-in-depth: confirm the binary landed where S1's shim expects.
  const binPath = path.join(destDir, binaryName);
  if (!fs.existsSync(binPath)) {
    throw new Error(`extraction did not produce ${binPath}`);
  }
  console.log(`stagecoach: installed ${version} (${goos}-${goarch}) to ${binPath}`);
}

// =============================================================================
// entry — async main with loud-failure handler
// =============================================================================

(async () => {
  try {
    await main();
  } catch (e) {
    // LOUD failure: never silently half-install. Print the error + the direct-install fallback URL
    // (the SAME #install anchor S1's shim uses) and exit non-zero so npm surfaces the failure.
    const msg = e && e.message ? e.message : String(e);
    process.stderr.write(
      'stagecoach postinstall failed: ' + msg + '\n' +
      'Install directly: ' + FALLBACK_URL + '\n'
    );
    process.exit(1);
  }
})();