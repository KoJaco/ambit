'use strict';
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { mapPlatform, artifactName, downloadUrl } = require('./platform');
const { verifyFile } = require('./verify');

const FETCH_TIMEOUT_MS = 120000;

function loadChecksums(file, version) {
  let data;
  try {
    data = JSON.parse(fs.readFileSync(file, 'utf8'));
  } catch (e) {
    throw new Error(`ambit: cannot read pinned checksums (${file}): ${e.message}. Refusing to run.`);
  }
  if (!data || typeof data !== 'object' || data.algorithm !== 'sha256' ||
      !data.artifacts || typeof data.artifacts !== 'object') {
    throw new Error('ambit: checksums.json is invalid. Refusing to run.');
  }
  if (data.version !== version) {
    throw new Error(
      `ambit: checksums.json version (${data.version}) does not match package version (${version}). Refusing to run.`
    );
  }
  return data;
}

async function defaultFetchFile(url, destPath) {
  const u = new URL(url);
  if (u.protocol !== 'https:' && u.protocol !== 'http:') {
    throw new Error(`ambit: refusing non-http(s) URL: ${url}`);
  }
  const res = await fetch(url, { redirect: 'follow', signal: AbortSignal.timeout(FETCH_TIMEOUT_MS) });
  if (!res.ok || !res.body) throw new Error(`ambit: download failed (${res.status}) for ${url}`);
  const out = fs.createWriteStream(destPath);
  try {
    const reader = res.body.getReader();
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      if (!out.write(value)) await new Promise((r) => out.once('drain', r));
    }
    await new Promise((resolve, reject) => {
      out.once('error', reject);
      out.end(resolve);
    });
  } catch (e) {
    out.destroy();
    throw e;
  }
}

function rmQuiet(p) {
  try { fs.rmSync(p, { force: true }); } catch { /* ignore */ }
}

// Cache hit = binary exists AND re-hashing it matches the pinned checksum.
// (Re-hash is a streaming sha256; cheap for ~15MB, and avoids trusting a marker.)
async function ensureBinary(opts) {
  const {
    platform, arch, version, checksums, cacheRoot,
    fetchFile = defaultFetchFile,
    log = () => {},
    env = process.env,
  } = opts;
  const m = mapPlatform(platform, arch);
  const artifact = artifactName(version, m.os, m.arch);

  if (!checksums || checksums.version !== version || !checksums.artifacts) {
    throw new Error('ambit: pinned checksums do not match package version. Refusing to run.');
  }
  const expected = checksums.artifacts[artifact];
  if (typeof expected !== 'string' || !/^[0-9a-fA-F]{64}$/.test(expected)) {
    throw new Error(`ambit: no pinned checksum for ${artifact}. A release without published checksums is not a release. Refusing to download.`);
  }

  const dir = path.join(cacheRoot, version, `${m.os}-${m.arch}`);
  const final = path.join(dir, m.os === 'windows' ? 'ambit.exe' : 'ambit');

  if (fs.existsSync(final)) {
    const r = await verifyFile(final, expected);
    if (r.ok) return final;
    log(`ambit: cached binary failed verification; re-downloading`);
    rmQuiet(final);
  }

  fs.mkdirSync(dir, { recursive: true });
  const tmp = path.join(dir, `.download-${process.pid}-${crypto.randomBytes(6).toString('hex')}`);
  const url = downloadUrl(version, artifact, env);
  log(`ambit: downloading ${url}`);
  try {
    await fetchFile(url, tmp);
    const r = await verifyFile(tmp, expected);
    if (!r.ok) {
      rmQuiet(tmp);
      throw new Error(
        `ambit: checksum mismatch for ${artifact}: expected ${expected}, got ${r.actual}. ` +
        'This release is not trustworthy; refusing to run it.'
      );
    }
    fs.chmodSync(tmp, 0o755);
    fs.renameSync(tmp, final);
  } catch (e) {
    rmQuiet(tmp);
    throw e;
  }
  log(`ambit: verified and cached ${final}`);
  return final;
}

module.exports = { ensureBinary, loadChecksums, defaultFetchFile };
