'use strict';
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const crypto = require('node:crypto');

const tmp = () => fs.mkdtempSync(path.join(os.tmpdir(), 'ambit-npm-'));
const sha = (buf) => crypto.createHash('sha256').update(buf).digest('hex');
const VERSION = '1.2.3';
const ARTIFACT = 'ambit_1.2.3_linux_amd64';
function checksumsFor(buf, artifact = ARTIFACT, version = VERSION) {
  return { version, algorithm: 'sha256', artifacts: { [artifact]: sha(buf) } };
}
module.exports = { tmp, sha, VERSION, ARTIFACT, checksumsFor };
