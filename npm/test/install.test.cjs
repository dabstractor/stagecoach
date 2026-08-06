'use strict';
//
// install.test.cjs — self-validating harness for npm/install.cjs (P2.M1.T1.S2).
//
// Builds a FIXTURE archive + checksums.txt at runtime (no committed binaries), serves them from a
// local node:http server, and runs install.cjs against it to assert:
//   (happy path)  download + SHA256-verify + extract a fake binary into the cache; it is executable;
//   (mismatch)    a tampered checksums.txt makes install.cjs exit non-zero WITHOUT writing the binary
//                 (abort-before-write) and prints the fallback URL.
//
// The win32 branch builds a .zip fixture for the production code path, but this host test only runs
// the executable assertion on unix (a real PE binary is needed on win32; a .cmd won't spawn without
// shell:true). The zip path is structurally identical and is exercised on a windows runner by S3/CI.
//
// NOTE on the async spawn: install.cjs is run with child_process.spawn (NOT spawnSync). The test
// process IS the fixture http server; spawnSync would block the parent's event loop so the server
// could not answer install.cjs's requests (the child's TCP connect would succeed at the kernel
// level, but the request would hang until the 60s timeout). Async spawn keeps the server serving.

const http = require('node:http');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn } = require('node:child_process');
const tar = require('tar'); // dep — available after `npm install --ignore-scripts` in npm/

const pkg = require('../package.json');

function assert(cond, msg) {
  if (!cond) {
    console.error('INSTALL-TEST FAIL: ' + msg);
    process.exit(1);
  }
}

// runInstall — async spawn of install.cjs, resolving to { status, stdout, stderr } on child exit.
// Uses spawn (not spawnSync) so the parent's fixture http server keeps serving the child's requests.
function runInstall(env) {
  return new Promise((resolve, reject) => {
    const child = spawn(process.execPath, [path.join(__dirname, '..', 'install.cjs')], {
      env,
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    let stdout = '';
    let stderr = '';
    child.stdout.on('data', (d) => (stdout += d.toString('utf8')));
    child.stderr.on('data', (d) => (stderr += d.toString('utf8')));
    child.on('error', reject);
    child.on('exit', (status) => resolve({ status, stdout, stderr }));
  });
}

(async () => {
  const version = pkg.version;
  const goos = process.platform === 'win32' ? 'windows' : process.platform;
  const goarch = process.arch === 'x64' ? 'amd64' : process.arch;
  const ext = process.platform === 'win32' ? '.zip' : '.tar.gz';
  const assetName = `stagecoach_${version}_${goos}_${goarch}${ext}`;
  const checksumsName = `stagecoach_${version}_checksums.txt`;

  // (1) build fixture: a fake binary -> tar.gz (unix) / zip (win32). The fake binary content on unix
  //     is a tiny shell script (so the executable-bit assertion + an optional run is meaningful).
  const serveDir = fs.mkdtempSync(path.join(os.tmpdir(), 'sc-srv-'));
  const fakeBinName = process.platform === 'win32' ? 'stagecoach.exe' : 'stagecoach';
  const fakeContent =
    process.platform === 'win32'
      ? 'fake'
      : '#!/bin/sh\necho fake-stagecoach-$STAGECOACH_INSTALL_METHOD\n';
  fs.writeFileSync(path.join(serveDir, fakeBinName), fakeContent);
  if (process.platform !== 'win32') {
    fs.chmodSync(path.join(serveDir, fakeBinName), 0o755);
  }

  const archivePath = path.join(serveDir, assetName);
  // tar.c packs fakeBinName (at the archive ROOT) into assetName; gzip:true for .tar.gz.
  await tar.c({ file: archivePath, cwd: serveDir, gzip: true }, [fakeBinName]);

  // checksums.txt: "<64hex>  <assetName>" (two spaces; install.cjs parses whitespace-split).
  const hash = crypto
    .createHash('sha256')
    .update(fs.readFileSync(archivePath))
    .digest('hex');
  fs.writeFileSync(path.join(serveDir, checksumsName), `${hash}  ${assetName}\n`);

  // (2) local http server serving the fixture dir. install.cjs GETs /<tag>/<assetName> +
  //     /<tag>/<checksumsName> (the base/tag/asset URL shape); serve by the LAST path segment
  //     (the filename) so any tag prefix resolves to the fixture in serveDir.
  const server = http.createServer((req, res) => {
    const urlPath = decodeURIComponent(req.url.split('?')[0]);
    const name = path.basename(urlPath);
    const f = path.join(serveDir, name);
    if (req.method !== 'GET' || !fs.existsSync(f) || fs.statSync(f).isDirectory()) {
      res.statusCode = 404;
      res.end('404');
      return;
    }
    res.statusCode = 200;
    fs.createReadStream(f).pipe(res);
  });
  await new Promise((r) => server.listen(0, '127.0.0.1', r));
  const base = `http://127.0.0.1:${server.address().port}`;

  // (3) HAPPY PATH: run install.cjs against the fixture (STAGECOACH_DOWNLOAD_BASE bypasses the 0.0.0
  //     dev guard; STAGECOACH_CACHE_DIR isolates the cache).
  const cache = fs.mkdtempSync(path.join(os.tmpdir(), 'sc-cache-'));
  let r = await runInstall({
    ...process.env,
    STAGECOACH_DOWNLOAD_BASE: base,
    STAGECOACH_CACHE_DIR: cache,
  });
  assert(
    r.status === 0,
    'happy path should exit 0, got ' + r.status + ' stderr=' + r.stderr
  );

  const installed = path.join(cache, version, `${goos}-${goarch}`, fakeBinName);
  assert(fs.existsSync(installed), 'binary not extracted to ' + installed);
  if (process.platform !== 'win32') {
    // executable bit (matches S1's smoke.cjs unix expectation).
    assert(!!(fs.statSync(installed).mode & 0o111), 'binary not executable: ' + installed);
  }

  // (4) CHECKSUM-MISMATCH ABORT: rewrite checksums.txt with a WRONG digest (0x00 * 32). install.cjs
  //     must exit NON-ZERO, print the mismatch + the fallback URL, and NOT leave a binary behind
  //     (abort-before-write: the tampered archive is removed before extraction).
  const cache2 = fs.mkdtempSync(path.join(os.tmpdir(), 'sc-cache2-'));
  fs.writeFileSync(path.join(serveDir, checksumsName), `${'0'.repeat(64)}  ${assetName}\n`);
  let r2 = await runInstall({
    ...process.env,
    STAGECOACH_DOWNLOAD_BASE: base,
    STAGECOACH_CACHE_DIR: cache2,
  });
  assert(r2.status !== 0, 'mismatch should fail loudly, got exit ' + r2.status);
  assert(/mismatch|postinstall failed/i.test(r2.stderr), 'stderr should report the mismatch: ' + r2.stderr);
  assert(/#install/.test(r2.stderr), 'stderr should include the fallback URL: ' + r2.stderr);
  assert(
    !fs.existsSync(path.join(cache2, version, `${goos}-${goarch}`, fakeBinName)),
    'mismatch must NOT leave a binary behind (abort-before-write)'
  );

  server.close();
  console.log('INSTALL-TEST PASS');
})().catch((e) => {
  console.error('INSTALL-TEST ERROR', e);
  process.exit(1);
});