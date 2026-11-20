'use strict';
const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const os = require('node:os');
const { spawnSync, spawn } = require('node:child_process');
const { tmp, sha } = require('./helpers');

const SKIP = process.platform === 'win32' ? 'unix-only' : false;
const SRC = path.join(__dirname, '..');

// Copy shim into a temp package dir so we can pin test checksums and version.
function stagePackage(binBuf) {
  const dir = tmp();
  for (const d of ['bin', 'lib']) fs.cpSync(path.join(SRC, d), path.join(dir, d), { recursive: true });
  const version = '3.4.5';
  fs.writeFileSync(path.join(dir, 'package.json'), JSON.stringify({ name: 'ambit', version }));
  const m = { darwin: 'darwin', linux: 'linux' }[process.platform];
  const a = { x64: 'amd64', arm64: 'arm64' }[process.arch];
  const artifact = `ambit_${version}_${m}_${a}`;
  fs.writeFileSync(path.join(dir, 'checksums.json'), JSON.stringify({
    version, algorithm: 'sha256', artifacts: { [artifact]: sha(binBuf) },
  }));
  return { dir, version, artifact };
}

const FAKE = Buffer.from('#!/bin/sh\nfor a in "$@"; do echo "ARG:$a"; done\n[ "$1" = fail ] && exit 7\nexit 0\n');

async function withServer(files, fn) {
  const hits = [];
  const srv = http.createServer((req, res) => {
    hits.push(req.url);
    const body = files[req.url];
    if (!body) { res.statusCode = 404; return res.end(); }
    res.end(body);
  });
  await new Promise((r) => srv.listen(0, '127.0.0.1', r));
  try { return await fn(`http://127.0.0.1:${srv.address().port}`, hits); }
  finally { srv.close(); }
}

function run(dir, args, env) {
  return new Promise((resolve) => {
    const c = spawn(process.execPath, [path.join(dir, 'bin', 'ambit.js'), ...args], {
      env: { ...process.env, ...env }, stdio: ['ignore', 'pipe', 'pipe'],
    });
    let out = ''; let err = '';
    c.stdout.on('data', (d) => (out += d)); c.stderr.on('data', (d) => (err += d));
    c.on('close', (code) => resolve({ code, out, err }));
  });
}

test('end-to-end: download via loopback, args pass through, stdout clean of shim logs', { skip: SKIP }, async () => {
  const { dir, version, artifact } = stagePackage(FAKE);
  const cache = tmp();
  await withServer({ [`/v${version}/${artifact}`]: FAKE }, async (base, hits) => {
    const env = { AMBIT_DOWNLOAD_BASE: base, AMBIT_CACHE_DIR: cache };
    for (const args of [['init'], ['start'], ['mcp'], ['check'], ['hook', 'install'], ['version']]) {
      const r = await run(dir, args, env);
      assert.strictEqual(r.code, 0, r.err);
      assert.strictEqual(r.out, args.map((a) => `ARG:${a}\n`).join(''));
    }
    assert.strictEqual(hits.length, 1, 'downloaded once, then cache hits');
    const r = await run(dir, ['fail', 'x'], env);
    assert.strictEqual(r.code, 7);
  });
});

test('end-to-end: tampered download refuses, exits non-zero, caches nothing', { skip: SKIP }, async () => {
  const { dir, version, artifact } = stagePackage(FAKE);
  const cache = tmp();
  await withServer({ [`/v${version}/${artifact}`]: Buffer.from('#!/bin/sh\necho pwned\n') }, async (base) => {
    const r = await run(dir, ['init'], { AMBIT_DOWNLOAD_BASE: base, AMBIT_CACHE_DIR: cache });
    assert.notStrictEqual(r.code, 0);
    assert.doesNotMatch(r.out, /pwned/);
    assert.match(r.err, /not trustworthy/);
    const leftovers = [];
    (function walk(d) { for (const e of fs.readdirSync(d, { withFileTypes: true })) {
      e.isDirectory() ? walk(path.join(d, e.name)) : leftovers.push(e.name); } })(cache);
    assert.deepStrictEqual(leftovers, []);
  });
});

test('end-to-end: placeholder checksums refuse without any download', { skip: SKIP }, async () => {
  const dir = tmp();
  for (const d of ['bin', 'lib']) fs.cpSync(path.join(SRC, d), path.join(dir, d), { recursive: true });
  fs.copyFileSync(path.join(SRC, 'package.json'), path.join(dir, 'package.json'));
  fs.copyFileSync(path.join(SRC, 'checksums.json'), path.join(dir, 'checksums.json'));
  await withServer({}, async (base, hits) => {
    const r = await run(dir, ['init'], { AMBIT_DOWNLOAD_BASE: base, AMBIT_CACHE_DIR: tmp() });
    assert.notStrictEqual(r.code, 0);
    assert.match(r.err, /no pinned checksum/);
    assert.strictEqual(hits.length, 0);
  });
});
