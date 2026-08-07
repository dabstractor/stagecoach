#!/usr/bin/env node
'use strict';
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

// (a) FR-U2: tag every invocation so `stagecoach upgrade` detects npm + delegates
//     to `npm install -g stagecoach-ai@latest` (FR-U3) instead of
//     self-swapping the cached binary. MUST be set on process.env BEFORE spawning
//     so the native child inherits it via { env: process.env }.
process.env.STAGECOACH_INSTALL_METHOD = 'npm';

const pkg = require('../package.json');

// (b) Node -> goreleaser map. MUST match internal/upgrade/download.go assetName()
//     so S2's install.cjs writes the exact path the shim resolves here.
//     goos: process.platform==='win32' -> 'windows' (goreleaser goos key).
//     goarch: process.arch==='x64' -> 'amd64' (goreleaser goarch key).
//     binaryName: stagecoach.exe on windows, stagecoach elsewhere.
const goos = process.platform === 'win32' ? 'windows' : process.platform;
const goarch = process.arch === 'x64' ? 'amd64' : process.arch;
const binaryName = process.platform === 'win32' ? 'stagecoach.exe' : 'stagecoach';

// Cache-path scheme (home-dir versioned, STAGECOACH_CACHE_DIR-overridable).
// Home-dir (NOT package-dir): a global root-owned install makes node_modules
// unwritable for postinstall; the home dir always is. S2's install.cjs MUST
// write to the identical path.
const cacheRoot = process.env.STAGECOACH_CACHE_DIR || path.join(os.homedir(), '.stagecoach', 'versions');
const cachedBin = path.join(cacheRoot, pkg.version, `${goos}-${goarch}`, binaryName);

// (c) Absent -> verbatim --ignore-scripts fallback to stderr + exit 1.
//     This is the --ignore-scripts / corporate-npm trap: postinstall was
//     blocked so the binary never downloaded.
if (!fs.existsSync(cachedBin)) {
  process.stderr.write(
    'stagecoach native binary not installed (npm --ignore-scripts or postinstall blocked). ' +
    'Install directly: https://github.com/dabstractor/stagecoach#install\n'
  );
  process.exit(1);
}

// (d) Present -> exec the cached native binary, forwarding argv + inheriting
//     stdio + env. spawnSync (NOT execSync) streams stdout/stderr/TTY through
//     (--edit, progress, multi-turn interactive UX preserved).
const result = spawnSync(cachedBin, process.argv.slice(2), { stdio: 'inherit', env: process.env });

// result.error = spawn failure (EACCES on non-executable, ENOENT race), NOT
// the child's own exit. Surface it + exit 1.
if (result.error) {
  process.stderr.write('stagecoach: failed to launch native binary: ' + result.error.message + '\n');
  process.exit(1);
}

// Propagate the child exit code. result.status is null when the child was
// killed by a SIGNAL (unix) -> exit 1 (conventional fallback). NEVER pass
// null to process.exit (it exits 0, hiding the signal kill).
process.exit(typeof result.status === 'number' ? result.status : 1);