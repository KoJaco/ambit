'use strict';
const test = require('node:test');
const assert = require('node:assert');
const path = require('node:path');
const { mapPlatform, artifactName, downloadUrl, cacheRoot } = require('../lib/platform');

test('platform mapping', () => {
  assert.deepStrictEqual(mapPlatform('darwin', 'arm64'), { os: 'darwin', arch: 'arm64' });
  assert.deepStrictEqual(mapPlatform('linux', 'x64'), { os: 'linux', arch: 'amd64' });
  assert.deepStrictEqual(mapPlatform('win32', 'x64'), { os: 'windows', arch: 'amd64' });
});

test('unsupported platform/arch errors with supported list', () => {
  assert.throws(() => mapPlatform('freebsd', 'x64'), /unsupported platform freebsd\/x64.*linux\/amd64/);
  assert.throws(() => mapPlatform('linux', 'ia32'), /unsupported/);
});

test('artifact and URL naming', () => {
  assert.strictEqual(artifactName('1.0.0', 'linux', 'amd64'), 'ambit_1.0.0_linux_amd64');
  assert.strictEqual(artifactName('1.0.0', 'windows', 'arm64'), 'ambit_1.0.0_windows_arm64.exe');
  assert.strictEqual(
    downloadUrl('1.0.0', 'a', {}),
    'https://github.com/KoJaco/ambit/releases/download/v1.0.0/a'
  );
  assert.strictEqual(downloadUrl('1.0.0', 'a', { AMBIT_DOWNLOAD_BASE: 'http://x/y/' }), 'http://x/y/v1.0.0/a');
});

test('cache dir resolution', () => {
  assert.strictEqual(cacheRoot('linux', { AMBIT_CACHE_DIR: '/c' }, '/h'), '/c');
  assert.strictEqual(cacheRoot('linux', {}, '/h'), path.join('/h', '.cache', 'ambit'));
  assert.strictEqual(cacheRoot('linux', { XDG_CACHE_HOME: '/x' }, '/h'), path.join('/x', 'ambit'));
  assert.strictEqual(cacheRoot('darwin', {}, '/h'), path.join('/h', 'Library', 'Caches', 'ambit'));
  assert.strictEqual(cacheRoot('win32', { LOCALAPPDATA: '/L' }, '/h'), path.join('/L', 'ambit'));
});

test('package.json publishes as @kojaco/ambit with an ambit bin', () => {
  const pkg = require('../package.json');
  assert.strictEqual(pkg.name, '@kojaco/ambit');
  assert.strictEqual(pkg.publishConfig.access, 'public');
  assert.ok(pkg.bin.ambit);
  assert.ok(require('node:fs').existsSync(path.join(__dirname, '..', pkg.bin.ambit)));
});
