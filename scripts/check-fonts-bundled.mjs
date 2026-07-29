#!/usr/bin/env node
// check-fonts-bundled.mjs — verify the brand fonts survive the Nuxt build.
//
// Why this exists: Geist and Geist Mono were previously pulled in with an
// `@import url(...)` to Google Fonts at the top of main.css. Vite stripped
// that import during the build, so the deployed CSS shipped zero @font-face
// rules and every page silently rendered in the system fallback. Nothing
// failed — not a test, not a lint, not a Lighthouse audit — because a missing
// webfont is invisible to everything except a human looking at the page.
//
// @nuxt/fonts now self-hosts the files, but the underlying hazard remains:
// font loading is a build-time side effect with no runtime error. This script
// turns that silent failure into a loud one, and it does so by asserting on
// the build output rather than on the source, so it catches any future cause
// — a module removal, a config regression, an upstream provider change — not
// just the specific @import mistake that started it.
//
// Run from the repo root or from web/ after `pnpm build`.

import { readdirSync, readFileSync, existsSync } from 'node:fs';
import { join, dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const outputDir = join(repoRoot, 'web', '.output', 'public');

const REQUIRED_FAMILIES = ['Geist', 'Geist Mono'];

const errors = [];

if (!existsSync(outputDir)) {
  console.error(`error: no build output at ${outputDir} — run \`pnpm build\` in web/ first.`);
  process.exit(2);
}

// 1. The generated CSS must declare @font-face for every brand family.
const cssDir = join(outputDir, '_nuxt');
const css = readdirSync(cssDir)
  .filter((f) => f.endsWith('.css'))
  .map((f) => readFileSync(join(cssDir, f), 'utf8'))
  .join('\n');

const faces = css.match(/@font-face\s*\{[^}]*\}/g) ?? [];
if (faces.length === 0) {
  errors.push('no @font-face rules in the built CSS — the fonts will not load at all');
}

for (const family of REQUIRED_FAMILIES) {
  // Match the family name exactly, not as a prefix: "Geist Mono" and the
  // metric-matched "Geist Fallback: Arial" faces both start with "Geist".
  const declared = faces.some((face) => {
    const m = face.match(/font-family:\s*"?([^";}]+)"?/);
    return m && m[1].trim() === family;
  });
  if (!declared) {
    errors.push(`no @font-face declares font-family: ${family}`);
  }
}

// 2. The woff2 files must actually be emitted, or the @font-face rules 404.
const fontsDir = join(outputDir, '_fonts');
const fontFiles = existsSync(fontsDir)
  ? readdirSync(fontsDir).filter((f) => /\.(woff2?|ttf|otf)$/.test(f))
  : [];
if (fontFiles.length === 0) {
  errors.push(`no font files emitted to ${fontsDir}`);
}

// 3. Nothing may point at a third-party font host. Those requests would be
//    blocked by our CSP (font-src 'self'; style-src 'self') and would put a
//    render-blocking third party back on the critical path.
const thirdParty = css.match(/https?:\/\/[^"')\s]*(?:googleapis|gstatic)\.com[^"')\s]*/g) ?? [];
if (thirdParty.length > 0) {
  errors.push(`built CSS references a third-party font host: ${thirdParty[0]}`);
}

if (errors.length > 0) {
  console.error('FAIL: brand fonts are not correctly bundled\n');
  for (const e of errors) console.error(`  - ${e}`);
  console.error('\nSee web/nuxt.config.ts (fonts) and web/app/assets/css/main.css.');
  process.exit(1);
}

console.log(
  `ok: ${faces.length} @font-face rules, ${fontFiles.length} font files, no third-party font hosts`,
);
