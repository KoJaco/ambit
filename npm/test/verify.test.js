'use strict';
const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');
const { verifyFile } = require('../lib/verify');
const { tmp, sha } = require('./helpers');

test('verifyFile match and mismatch', async () => {
  const f = path.join(tmp(), 'f');
  fs.writeFileSync(f, 'hello');
  assert.deepStrictEqual(await verifyFile(f, sha('hello')), { ok: true, actual: sha('hello') });
  const r = await verifyFile(f, sha('other'));
  assert.strictEqual(r.ok, false);
  assert.strictEqual(r.actual, sha('hello'));
});
