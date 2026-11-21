#!/usr/bin/env node
'use strict';
const path = require('node:path');
const { spawn } = require('node:child_process');
const { ensureBinary, loadChecksums } = require('../lib/install');
const { cacheRoot } = require('../lib/platform');

// All shim logging goes to stderr: stdout must stay clean for `ambit mcp`.
const log = (msg) => process.stderr.write(msg + '\n');

async function main() {
  const version = require('../package.json').version;
  const checksums = loadChecksums(path.join(__dirname, '..', 'checksums.json'), version);
  const binary = await ensureBinary({
    platform: process.platform,
    arch: process.arch,
    version,
    checksums,
    cacheRoot: cacheRoot(process.platform, process.env),
    log,
  });
  const child = spawn(binary, process.argv.slice(2), { stdio: 'inherit' });
  for (const sig of ['SIGINT', 'SIGTERM']) {
    process.on(sig, () => { try { child.kill(sig); } catch { /* ignore */ } });
  }
  child.on('error', (e) => { log(`ambit: failed to start binary: ${e.message}`); process.exit(1); });
  child.on('exit', (code, signal) => {
    if (signal) process.exit(128 + (require('node:os').constants.signals[signal] || 0));
    process.exit(code === null ? 1 : code);
  });
}

main().catch((e) => {
  log(e && e.message ? e.message : String(e));
  process.exit(1);
});
