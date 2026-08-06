'use strict';
// Self-validating smoke test for the npm wrapper shim (npm/bin/stagecoach.js).
// Asserts: (1) the shim execs a cached fake binary, (2) STAGECOACH_INSTALL_METHOD=npm
// is set on the child env, (3) the child exit code propagates, (4) the fallback path
// (no cached binary) prints the verbatim message + exits 1.
//
// WIN32 SKIP: spawnFileSync to a .exe needs a real PE binary; a .cmd won't run without
// shell:true (which the shim omits for argv fidelity). S3/CI owns the Windows stub.
// S1 proves the platform-independent LOGIC here.
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const pkg = require('../package.json');

if (process.platform === 'win32') {
  console.log('smoke: skipped on win32 (S3/CI adds a Windows stub)');
  process.exit(0);
}

function assert(cond, msg) {
  if (!cond) {
    console.error('SMOKE FAIL: ' + msg);
    process.exit(1);
  }
}

const shim = path.join(__dirname, '..', 'bin', 'stagecoach.js');

// On unix, process.platform === goos (the goreleaser value, e.g. 'linux'/'darwin').
const goos = process.platform;
const goarch = process.arch === 'x64' ? 'amd64' : process.arch;

// 1. temp cache root + fake binary directory mirroring the shim's cache-path scheme.
const cacheRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'sc-smoke-'));
const binDir = path.join(cacheRoot, pkg.version, `${goos}-${goarch}`);
fs.mkdirSync(binDir, { recursive: true });
const cachedBin = path.join(binDir, 'stagecoach');

// 2. fake binary that echoes the env tag + proves it ran.
fs.writeFileSync(cachedBin, '#!/bin/sh\necho "SMOKE-OK install_method=$STAGECOACH_INSTALL_METHOD"\n', { mode: 0o755 });
fs.chmodSync(cachedBin, 0o755);

// 3. run the shim with the redirected cache + capture stdout.
const r = spawnSync(
  process.execPath,
  [shim, '--version'],
  { stdio: ['ignore', 'pipe', 'pipe'], env: { ...process.env, STAGECOACH_CACHE_DIR: cacheRoot } }
);
const out = r.stdout.toString();
assert(out.includes('SMOKE-OK'), 'shim did not exec the fake binary; stderr=' + r.stderr.toString());
assert(out.includes('install_method=npm'), 'STAGECOACH_INSTALL_METHOD not set to npm on the child env');

// 4. exit-code propagation: a fake that exits 42 -> shim exits 42.
fs.writeFileSync(cachedBin, '#!/bin/sh\nexit 42\n', { mode: 0o755 });
fs.chmodSync(cachedBin, 0o755);
const r2 = spawnSync(
  process.execPath,
  [shim],
  { stdio: ['ignore', 'pipe', 'pipe'], env: { ...process.env, STAGECOACH_CACHE_DIR: cacheRoot } }
);
assert(r2.status === 42, 'exit-code propagation failed: shim exited ' + r2.status + ' (want 42)');

// 5. fallback path: NO cached binary -> verbatim message to stderr + exit 1.
const emptyCache = fs.mkdtempSync(path.join(os.tmpdir(), 'sc-smoke-empty-'));
const r3 = spawnSync(
  process.execPath,
  [shim],
  { stdio: ['ignore', 'pipe', 'pipe'], env: { ...process.env, STAGECOACH_CACHE_DIR: emptyCache } }
);
assert(r3.status === 1, 'fallback should exit 1, got ' + r3.status);
assert(r3.stderr.toString().includes('stagecoach native binary not installed'), 'missing fallback message');

console.log('SMOKE PASS');