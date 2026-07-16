#!/usr/bin/env node
// ccpill npm 启动器：把平台包里的 Go 二进制搬到稳定路径后代理执行。
// 为什么搬运而不是原地执行：npx 缓存目录是易失的，statusline 命令必须指向
// 稳定路径（~/.claude/ccpill/bin），否则 `ccpill --install` 写进 settings.json
// 的路径会在 npx 缓存清理后失效。首跑或版本变化时才复制，其余时候零开销。
'use strict';
const { spawnSync } = require('child_process');
const fs = require('fs');
const path = require('path');
const os = require('os');

const pkg = require('../package.json');
const exeName = process.platform === 'win32' ? 'ccpill.exe' : 'ccpill';

function platformBinary() {
  const key = process.platform + '-' + process.arch;
  const supported = ['win32-x64', 'linux-x64', 'linux-arm64', 'darwin-x64', 'darwin-arm64'];
  if (!supported.includes(key)) {
    console.error('ccpill: unsupported platform ' + key + ' — build from source: go install github.com/cass-2003/ccpill@latest');
    process.exit(1);
  }
  try {
    return require.resolve('ccpill-' + key + '/' + exeName);
  } catch (e) {
    console.error('ccpill: platform package ccpill-' + key + ' not installed (optionalDependencies skipped?). Try: npm i -g ccpill-' + key);
    process.exit(1);
  }
}

function stableDir() {
  const base = process.env.CLAUDE_CONFIG_DIR || path.join(os.homedir(), '.claude');
  return path.join(base, 'ccpill', 'bin');
}

function ensureStable() {
  const dir = stableDir();
  const target = path.join(dir, exeName);
  const versionFile = path.join(dir, 'VERSION');
  let current = '';
  try { current = fs.readFileSync(versionFile, 'utf8').trim(); } catch (e) {}
  if (current === pkg.version && fs.existsSync(target)) return target;
  const src = platformBinary();
  fs.mkdirSync(dir, { recursive: true });
  fs.copyFileSync(src, target);
  if (process.platform !== 'win32') fs.chmodSync(target, 0o755);
  fs.writeFileSync(versionFile, pkg.version + '\n');
  return target;
}

const bin = ensureStable();
const r = spawnSync(bin, process.argv.slice(2), { stdio: 'inherit' });
process.exit(r.status === null ? 1 : r.status);
