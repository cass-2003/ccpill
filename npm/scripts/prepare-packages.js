#!/usr/bin/env node
// 发布准备：把版本号盖进主包 + 5 个平台包模板，二进制从 dist/ 搬进发布目录。
// 用法: node npm/scripts/prepare-packages.js <version>   （version 不带 v 前缀）
// 产出: npm-publish/main + npm-publish/platforms/<key>，供 CI 逐个 npm publish。
// 主包 optionalDependencies 版本与平台包同步盖章——保证版本号永远严格一致。
'use strict';
const fs = require('fs');
const path = require('path');

const version = process.argv[2];
if (!version || !/^\d+\.\d+\.\d+/.test(version)) {
  console.error('usage: prepare-packages.js <version>  e.g. 0.2.1');
  process.exit(1);
}

const root = path.join(__dirname, '..', '..');
const out = path.join(root, 'npm-publish');
fs.rmSync(out, { recursive: true, force: true });

// dist/ 里 Go 交叉编译产物 → npm 平台包目录名/二进制名
const platforms = {
  'win32-x64':    { dist: 'ccpill-windows-amd64.exe', bin: 'ccpill.exe' },
  'linux-x64':    { dist: 'ccpill-linux-amd64',       bin: 'ccpill' },
  'linux-arm64':  { dist: 'ccpill-linux-arm64',       bin: 'ccpill' },
  'darwin-x64':   { dist: 'ccpill-darwin-amd64',      bin: 'ccpill' },
  'darwin-arm64': { dist: 'ccpill-darwin-arm64',      bin: 'ccpill' },
};

for (const [key, p] of Object.entries(platforms)) {
  const src = path.join(root, 'npm', 'platforms', key, 'package.json');
  const dstDir = path.join(out, 'platforms', key);
  fs.mkdirSync(dstDir, { recursive: true });
  const pkg = JSON.parse(fs.readFileSync(src, 'utf8'));
  pkg.version = version;
  fs.writeFileSync(path.join(dstDir, 'package.json'), JSON.stringify(pkg, null, 2) + '\n');
  const distBin = path.join(root, 'dist', p.dist);
  if (!fs.existsSync(distBin)) {
    console.error('missing binary: ' + distBin);
    process.exit(1);
  }
  fs.copyFileSync(distBin, path.join(dstDir, p.bin));
  if (!p.bin.endsWith('.exe')) fs.chmodSync(path.join(dstDir, p.bin), 0o755);
  console.log('prepared ccpill-' + key + '@' + version);
}

const mainSrc = path.join(root, 'npm', 'main');
const mainDst = path.join(out, 'main');
fs.mkdirSync(path.join(mainDst, 'bin'), { recursive: true });
const main = JSON.parse(fs.readFileSync(path.join(mainSrc, 'package.json'), 'utf8'));
main.version = version;
for (const dep of Object.keys(main.optionalDependencies)) {
  main.optionalDependencies[dep] = version;
}
fs.writeFileSync(path.join(mainDst, 'package.json'), JSON.stringify(main, null, 2) + '\n');
fs.copyFileSync(path.join(mainSrc, 'bin', 'ccpill.js'), path.join(mainDst, 'bin', 'ccpill.js'));
console.log('prepared ccpill@' + version);
