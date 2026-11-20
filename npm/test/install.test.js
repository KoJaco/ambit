'use strict';
const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');
const { ensureBinary, loadChecksums } = require('../lib/install');
const { tmp, VERSION, ARTIFACT, checksumsFor } = require('./helpers');

const GOOD = Buffer.from('#!/bin/sh\necho ok\n');
const base = (root, checksums) => ({
  platform: 'linux', arch: 'x64', version: VERSION, checksums, cacheRoot: root,
});
const finalPath = (root) => path.join(root, VERSION, 'linux-amd64', 'ambit');
const spyFetch = (buf, calls) => async (url, dest) => { calls.push({ url, dest }); fs.writeFileSync(dest, buf); };

test('matching checksum returns executable file', async () => {
  const root = tmp(); const calls = [];
  const p = await ensureBinary({ ...base(root, checksumsFor(GOOD)), fetchFile: spyFetch(GOOD, calls) });
  assert.strictEqual(p, finalPath(root));
  assert.ok(fs.statSync(p).mode & 0o100);
  assert.strictEqual(calls.length, 1);
  assert.match(calls[0].url, /\/v1\.2\.3\/ambit_1\.2\.3_linux_amd64$/);
  assert.deepStrictEqual(fs.readdirSync(path.dirname(p)), ['ambit']);
});

test('mismatch refuses, leaves no binary, removes temp file', async () => {
  const root = tmp(); const calls = [];
  await assert.rejects(
    ensureBinary({ ...base(root, checksumsFor(GOOD)), fetchFile: spyFetch(Buffer.from('evil'), calls) }),
    /expected [0-9a-f]{64}, got [0-9a-f]{64}.*not trustworthy/
  );
  assert.strictEqual(fs.existsSync(finalPath(root)), false);
  assert.deepStrictEqual(fs.readdirSync(path.dirname(finalPath(root))), []);
});

test('verification happens before chmod/rename (file still non-exec and not final at fetch end)', async () => {
  const root = tmp(); const events = [];
  const origChmod = fs.chmodSync; const origRename = fs.renameSync;
  fs.chmodSync = (...a) => { events.push('chmod'); return origChmod(...a); };
  fs.renameSync = (...a) => { events.push('rename'); return origRename(...a); };
  try {
    await assert.rejects(
      ensureBinary({
        ...base(root, checksumsFor(GOOD)),
        fetchFile: async (u, d) => { events.push('fetch'); fs.writeFileSync(d, 'evil'); },
      })
    );
    assert.deepStrictEqual(events, ['fetch']);
    events.length = 0;
    await ensureBinary({
      ...base(root, checksumsFor(GOOD)),
      fetchFile: async (u, d) => { events.push('fetch'); fs.writeFileSync(d, GOOD); },
    });
    assert.deepStrictEqual(events, ['fetch', 'chmod', 'rename']);
  } finally {
    fs.chmodSync = origChmod; fs.renameSync = origRename;
  }
});

test('cache hit does not fetch', async () => {
  const root = tmp(); const calls = [];
  fs.mkdirSync(path.dirname(finalPath(root)), { recursive: true });
  fs.writeFileSync(finalPath(root), GOOD);
  const p = await ensureBinary({ ...base(root, checksumsFor(GOOD)), fetchFile: spyFetch(GOOD, calls) });
  assert.strictEqual(p, finalPath(root));
  assert.strictEqual(calls.length, 0);
});

test('corrupt cached binary is re-downloaded', async () => {
  const root = tmp(); const calls = [];
  fs.mkdirSync(path.dirname(finalPath(root)), { recursive: true });
  fs.writeFileSync(finalPath(root), 'corrupt');
  await ensureBinary({ ...base(root, checksumsFor(GOOD)), fetchFile: spyFetch(GOOD, calls) });
  assert.strictEqual(calls.length, 1);
  assert.deepStrictEqual(fs.readFileSync(finalPath(root)), GOOD);
});

test('missing checksum entry refuses without fetching', async () => {
  const root = tmp(); const calls = [];
  const cs = { version: VERSION, algorithm: 'sha256', artifacts: {} };
  await assert.rejects(ensureBinary({ ...base(root, cs), fetchFile: spyFetch(GOOD, calls) }), /no pinned checksum for ambit_1\.2\.3_linux_amd64/);
  assert.strictEqual(calls.length, 0);
});

test('unsupported platform refuses without fetching', async () => {
  const calls = [];
  await assert.rejects(
    ensureBinary({ ...base(tmp(), checksumsFor(GOOD)), platform: 'sunos', fetchFile: spyFetch(GOOD, calls) }),
    /unsupported platform/
  );
  assert.strictEqual(calls.length, 0);
});

test('version mismatch refuses', async () => {
  const root = tmp(); const calls = [];
  await assert.rejects(
    ensureBinary({ ...base(root, checksumsFor(GOOD, ARTIFACT, '9.9.9')), fetchFile: spyFetch(GOOD, calls) }),
    /do not match package version/
  );
  assert.strictEqual(calls.length, 0);
});

test('loadChecksums: version mismatch, unreadable, invalid', () => {
  const d = tmp(); const f = path.join(d, 'c.json');
  fs.writeFileSync(f, JSON.stringify(checksumsFor(GOOD, ARTIFACT, '1.0.0')));
  assert.throws(() => loadChecksums(f, VERSION), /does not match package version/);
  assert.deepStrictEqual(loadChecksums(f, '1.0.0').version, '1.0.0');
  assert.throws(() => loadChecksums(path.join(d, 'nope.json'), VERSION), /cannot read/);
  fs.writeFileSync(f, '{not json');
  assert.throws(() => loadChecksums(f, VERSION), /cannot read/);
  fs.writeFileSync(f, '{"version":"1.2.3"}');
  assert.throws(() => loadChecksums(f, VERSION), /invalid/);
});
