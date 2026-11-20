'use strict';
const path = require('node:path');
const os = require('node:os');

const OS_MAP = { darwin: 'darwin', linux: 'linux', win32: 'windows' };
const ARCH_MAP = { x64: 'amd64', arm64: 'arm64' };
const SUPPORTED = ['darwin/amd64', 'darwin/arm64', 'linux/amd64', 'linux/arm64', 'windows/amd64', 'windows/arm64'];
const DEFAULT_BASE = 'https://github.com/KoJaco/ambit/releases/download';

function mapPlatform(platform, arch) {
  const o = OS_MAP[platform];
  const a = ARCH_MAP[arch];
  if (!o || !a) {
    throw new Error(
      `ambit: unsupported platform ${platform}/${arch}. Supported: ${SUPPORTED.join(', ')}.`
    );
  }
  return { os: o, arch: a };
}

function artifactName(version, osName, archName) {
  return `ambit_${version}_${osName}_${archName}${osName === 'windows' ? '.exe' : ''}`;
}

function downloadUrl(version, artifact, env = process.env) {
  const base = (env.AMBIT_DOWNLOAD_BASE || DEFAULT_BASE).replace(/\/+$/, '');
  return `${base}/v${version}/${artifact}`;
}

// Returns the ambit cache root (version dirs live directly beneath it).
// platform is the Node platform name (darwin|linux|win32) or mapped os name.
function cacheRoot(platform, env = process.env, home = os.homedir()) {
  if (env.AMBIT_CACHE_DIR) return env.AMBIT_CACHE_DIR;
  let base;
  if (platform === 'darwin') base = path.join(home, 'Library', 'Caches');
  else if (platform === 'win32' || platform === 'windows') {
    base = env.LOCALAPPDATA || path.join(home, 'AppData', 'Local');
  } else base = env.XDG_CACHE_HOME || path.join(home, '.cache');
  return path.join(base, 'ambit');
}

module.exports = { mapPlatform, artifactName, downloadUrl, cacheRoot, SUPPORTED, DEFAULT_BASE };
