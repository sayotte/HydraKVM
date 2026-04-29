// Copyright (C) 2026 Stephen Ayotte
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

import { spawn } from 'node:child_process';
import { mkdir, stat } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import * as esbuild from 'esbuild';

const HERE = dirname(fileURLToPath(import.meta.url));
const DIST = resolve(HERE, '../server/internal/http/web/dist');
const ENTRY = resolve(HERE, 'src/app.ts');
const CSS_INPUT = resolve(HERE, 'src/styles.css');
const CSS_OUTPUT = resolve(DIST, 'styles.css');
const JS_OUTPUT = resolve(DIST, 'app.js');
const TAILWIND_BIN = resolve(HERE, 'node_modules/.bin/tailwindcss');
const TAILWIND_CONFIG = resolve(HERE, 'tailwind.config.mjs');

const args = new Set(process.argv.slice(2));
const watch = args.has('--watch');
const prod = args.has('--prod');

async function reportSize(label, path) {
  const s = await stat(path);
  process.stdout.write(`${label}: ${path} (${s.size} bytes)\n`);
}

async function buildJs() {
  const opts = {
    entryPoints: [ENTRY],
    outfile: JS_OUTPUT,
    bundle: true,
    format: 'esm',
    target: 'es2022',
    sourcemap: prod ? 'linked' : 'inline',
    minify: prod,
    logLevel: 'info',
    ...(prod ? { drop: ['console'] } : {}),
  };
  if (watch) {
    const ctx = await esbuild.context(opts);
    await ctx.watch();
    return ctx;
  }
  await esbuild.build(opts);
  await reportSize('js  ', JS_OUTPUT);
  return null;
}

function spawnTailwind() {
  const tailwindArgs = [
    '--config', TAILWIND_CONFIG,
    '--input', CSS_INPUT,
    '--output', CSS_OUTPUT,
  ];
  if (prod) tailwindArgs.push('--minify');
  if (watch) tailwindArgs.push('--watch');
  const child = spawn(TAILWIND_BIN, tailwindArgs, {
    cwd: HERE,
    stdio: 'inherit',
  });
  return child;
}

async function runTailwindOnce() {
  return new Promise((resolveP, rejectP) => {
    const child = spawnTailwind();
    child.on('exit', (code) => {
      if (code === 0) resolveP();
      else rejectP(new Error(`tailwindcss exited ${code}`));
    });
    child.on('error', rejectP);
  });
}

async function main() {
  await mkdir(DIST, { recursive: true });

  if (watch) {
    const esbuildCtx = await buildJs();
    const tailwindChild = spawnTailwind();
    const shutdown = async () => {
      try { await esbuildCtx?.dispose(); } catch { /* noop */ }
      try { tailwindChild.kill('SIGTERM'); } catch { /* noop */ }
      process.exit(0);
    };
    process.on('SIGTERM', shutdown);
    process.on('SIGINT', shutdown);
    return;
  }

  await buildJs();
  await runTailwindOnce();
  await reportSize('css ', CSS_OUTPUT);
}

main().catch((err) => {
  process.stderr.write(`build failed: ${err?.message ?? err}\n`);
  process.exit(1);
});
