'use strict';
const fs = require('node:fs');
const crypto = require('node:crypto');

function sha256File(path) {
  return new Promise((resolve, reject) => {
    const h = crypto.createHash('sha256');
    const s = fs.createReadStream(path);
    s.on('error', reject);
    s.on('data', (d) => h.update(d));
    s.on('end', () => resolve(h.digest('hex')));
  });
}

async function verifyFile(path, expected) {
  const actual = await sha256File(path);
  return { ok: actual === String(expected).toLowerCase(), actual };
}

module.exports = { sha256File, verifyFile };
